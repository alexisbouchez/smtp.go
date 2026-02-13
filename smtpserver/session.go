package smtpserver

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/internal/textproto"
)

var base64Encoding = base64.StdEncoding

// sessionState tracks where the session is in the SMTP conversation.
type sessionState int

const (
	stateNew     sessionState = iota // Connected, waiting for greeting to be sent.
	stateGreeted                     // EHLO/HELO received.
	stateMail                        // MAIL FROM received.
	stateRcpt                        // At least one RCPT TO received.
	stateData                        // DATA in progress.
)

// session represents a single SMTP client connection.
type session struct {
	server *Server
	conn   *textproto.Conn
	state  sessionState

	clientHostname string
	esmtp          bool // True if client used EHLO.
	tls            bool // True if connection is TLS.
	authenticated  bool // True if AUTH succeeded.
	invalidCmds    int  // Count of unrecognized/rejected commands.

	reversePath  smtp.ReversePath
	forwardPaths []smtp.ForwardPath
	bdatBuffer   []byte // Accumulated BDAT chunks.
}

// handleConn is the entry point for a new client connection.
func (s *Server) handleConn(nc net.Conn) {
	conn := textproto.NewConn(nc)
	remoteAddr := nc.RemoteAddr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Watch for server shutdown — close the connection to unblock reads.
	go func() {
		select {
		case <-s.quit:
			cancel()
			nc.Close()
		case <-ctx.Done():
		}
	}()

	// Connection handler check.
	if s.connHandler != nil {
		if err := s.connHandler.OnConnect(ctx, nc.RemoteAddr()); err != nil {
			if smtpErr, ok := err.(*smtp.SMTPError); ok {
				conn.WriteReply(int(smtpErr.Code), smtpErr.Message)
			} else {
				conn.WriteReply(int(smtp.ReplyServiceNotAvailable), "Connection refused")
			}
			conn.Close()
			return
		}
	}

	sess := &session{
		server: s,
		conn:   conn,
		state:  stateNew,
	}

	defer conn.Close()

	// Send greeting banner (RFC 5321 §4.3.1).
	if err := conn.WriteReply(int(smtp.ReplyServiceReady), fmt.Sprintf("%s ESMTP ready", s.hostname)); err != nil {
		s.logger.Error("failed to send greeting", "err", err, "remote", remoteAddr)
		return
	}

	// Command loop.
	for {
		select {
		case <-ctx.Done():
			conn.WriteReply(int(smtp.ReplyServiceNotAvailable), "Server shutting down")
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(s.readTimeout))
		line, err := conn.ReadLine(textproto.MaxCommandLineLen)
		if err != nil {
			return // Connection closed or error.
		}

		// Reject NUL bytes in commands.
		if strings.ContainsRune(line, 0) {
			sess.reply(smtp.ReplySyntaxError, smtp.EnhancedCodeInvalidCommand, "NUL not allowed in commands")
			sess.invalidCmds++
			if s.maxInvalidCmds > 0 && sess.invalidCmds >= s.maxInvalidCmds {
				sess.reply(smtp.ReplyServiceNotAvailable, smtp.EnhancedCodeOtherNetwork, "Too many errors, closing connection")
				return
			}
			continue
		}

		verb, args := parseCommand(line)

		switch verb {
		case "EHLO":
			sess.handleEHLO(args)
		case "HELO":
			sess.handleHELO(args)
		case "MAIL":
			sess.handleMAIL(args)
		case "RCPT":
			sess.handleRCPT(args)
		case "DATA":
			sess.handleDATA()
		case "RSET":
			sess.handleRSET()
		case "NOOP":
			sess.handleNOOP()
		case "QUIT":
			sess.handleQUIT()
			return
		case "VRFY":
			sess.handleVRFY(args)
		case "EXPN":
			sess.reply(smtp.ReplyCommandNotImpl, smtp.EnhancedCodeInvalidCommand, "EXPN not implemented")
		case "STARTTLS":
			if sess.handleSTARTTLS() {
				// Connection upgraded — must re-issue EHLO. State reset handled inside.
			}
		case "AUTH":
			sess.handleAUTH(args)
		case "BDAT":
			sess.handleBDAT(args)
		default:
			sess.reply(smtp.ReplySyntaxError, smtp.EnhancedCodeInvalidCommand, "Command not recognized")
			sess.invalidCmds++
			if s.maxInvalidCmds > 0 && sess.invalidCmds >= s.maxInvalidCmds {
				sess.reply(smtp.ReplyServiceNotAvailable, smtp.EnhancedCodeOtherNetwork, "Too many errors, closing connection")
				return
			}
		}
	}
}

// parseCommand splits an SMTP command line into verb and argument string.
func parseCommand(line string) (verb string, args string) {
	verb, args, _ = strings.Cut(line, " ")
	verb = strings.ToUpper(verb)
	return
}

// reply sends a single-line reply with optional enhanced status code.
func (s *session) reply(code smtp.ReplyCode, enhanced smtp.EnhancedCode, msg string) {
	var line string
	if !enhanced.IsZero() {
		line = fmt.Sprintf("%s %s", enhanced, msg)
	} else {
		line = msg
	}
	s.conn.WriteReply(int(code), line)
}

// replyMulti sends a multi-line reply.
func (s *session) replyMulti(code smtp.ReplyCode, lines ...string) {
	s.conn.WriteReply(int(code), lines...)
}

// handleEHLO processes the EHLO command (RFC 5321 §4.1.1.1).
func (s *session) handleEHLO(args string) {
	if args == "" {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "EHLO requires a hostname")
		return
	}

	if s.server.heloHandler != nil {
		if err := s.server.heloHandler.OnHelo(context.Background(), args); err != nil {
			if smtpErr, ok := err.(*smtp.SMTPError); ok {
				s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
			} else {
				s.reply(smtp.ReplyLocalError, smtp.EnhancedCodeOtherNetwork, "Internal error")
			}
			return
		}
	}

	// Reset session state on EHLO re-issue (RFC 5321 §4.1.4).
	s.resetTransaction()
	s.clientHostname = args
	s.esmtp = true
	s.state = stateGreeted

	// Build EHLO response lines.
	lines := []string{
		fmt.Sprintf("%s Hello %s", s.server.hostname, args),
	}

	// Advertise extensions.
	if s.server.maxMessageSize > 0 {
		lines = append(lines, fmt.Sprintf("SIZE %d", s.server.maxMessageSize))
	}
	lines = append(lines, "PIPELINING")
	lines = append(lines, "8BITMIME")
	lines = append(lines, "ENHANCEDSTATUSCODES")
	lines = append(lines, "DSN")
	lines = append(lines, "SMTPUTF8")
	lines = append(lines, "CHUNKING")

	if s.server.tlsConfig != nil && !s.tls {
		lines = append(lines, "STARTTLS")
	}

	if s.server.authHandler != nil && !s.authenticated {
		lines = append(lines, "AUTH PLAIN LOGIN CRAM-MD5")
	}

	s.replyMulti(smtp.ReplyOK, lines...)
}

// handleHELO processes the HELO command (RFC 5321 §4.1.1.1).
func (s *session) handleHELO(args string) {
	if args == "" {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "HELO requires a hostname")
		return
	}

	if s.server.heloHandler != nil {
		if err := s.server.heloHandler.OnHelo(context.Background(), args); err != nil {
			if smtpErr, ok := err.(*smtp.SMTPError); ok {
				s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
			} else {
				s.reply(smtp.ReplyLocalError, smtp.EnhancedCodeOtherNetwork, "Internal error")
			}
			return
		}
	}

	s.resetTransaction()
	s.clientHostname = args
	s.esmtp = false
	s.state = stateGreeted

	s.reply(smtp.ReplyOK, smtp.EnhancedCodeOK, fmt.Sprintf("%s Hello %s", s.server.hostname, args))
}

// handleMAIL processes the MAIL FROM command (RFC 5321 §4.1.1.2).
func (s *session) handleMAIL(args string) {
	if s.state < stateGreeted {
		s.reply(smtp.ReplyBadSequence, smtp.EnhancedCodeInvalidCommand, "Send EHLO/HELO first")
		return
	}
	if s.state >= stateMail {
		s.reply(smtp.ReplyBadSequence, smtp.EnhancedCodeInvalidCommand, "MAIL already specified")
		return
	}

	// Submission mode requires authentication (RFC 6409 §4.1).
	if s.server.submissionMode && !s.authenticated {
		s.reply(smtp.ReplyAuthRequired, smtp.EnhancedCodeAuthRequired, "Authentication required")
		return
	}

	// Parse "FROM:<path> [params]".
	upper := strings.ToUpper(args)
	if !strings.HasPrefix(upper, "FROM:") {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Syntax: MAIL FROM:<address>")
		return
	}

	pathAndParams := args[5:] // Skip "FROM:"
	pathStr, _, _ := strings.Cut(pathAndParams, " ")
	pathStr = strings.TrimSpace(pathStr)

	reversePath, err := smtp.ParseReversePath(pathStr)
	if err != nil {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeBadSenderSyntax, "Invalid sender address")
		return
	}

	if s.server.mailHandler != nil {
		if err := s.server.mailHandler.OnMail(context.Background(), reversePath); err != nil {
			if smtpErr, ok := err.(*smtp.SMTPError); ok {
				s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
			} else {
				s.reply(smtp.ReplyLocalError, smtp.EnhancedCodeOtherNetwork, "Internal error")
			}
			return
		}
	}

	s.reversePath = reversePath
	s.forwardPaths = nil
	s.state = stateMail

	s.reply(smtp.ReplyOK, smtp.EnhancedCodeOtherAddress, "Originator ok")
}

// handleRCPT processes the RCPT TO command (RFC 5321 §4.1.1.3).
func (s *session) handleRCPT(args string) {
	if s.state < stateMail {
		s.reply(smtp.ReplyBadSequence, smtp.EnhancedCodeInvalidCommand, "Send MAIL first")
		return
	}

	if len(s.forwardPaths) >= s.server.maxRecipients {
		s.reply(smtp.ReplyInsufficientStorage, smtp.EnhancedCodeTooManyRecipients, "Too many recipients")
		return
	}

	// Parse "TO:<path> [params]".
	upper := strings.ToUpper(args)
	if !strings.HasPrefix(upper, "TO:") {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Syntax: RCPT TO:<address>")
		return
	}

	pathAndParams := args[3:] // Skip "TO:"
	pathStr, _, _ := strings.Cut(pathAndParams, " ")
	pathStr = strings.TrimSpace(pathStr)

	forwardPath, err := smtp.ParseForwardPath(pathStr)
	if err != nil {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeBadDestSyntax, "Invalid recipient address")
		return
	}

	if s.server.rcptHandler != nil {
		if err := s.server.rcptHandler.OnRcpt(context.Background(), forwardPath); err != nil {
			if smtpErr, ok := err.(*smtp.SMTPError); ok {
				s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
			} else {
				s.reply(smtp.ReplyLocalError, smtp.EnhancedCodeOtherNetwork, "Internal error")
			}
			return
		}
	}

	s.forwardPaths = append(s.forwardPaths, forwardPath)
	if s.state < stateRcpt {
		s.state = stateRcpt
	}

	s.reply(smtp.ReplyOK, smtp.EnhancedCodeDestValid, "Recipient ok")
}

// handleDATA processes the DATA command (RFC 5321 §4.1.1.4).
func (s *session) handleDATA() {
	if s.state < stateRcpt {
		s.reply(smtp.ReplyBadSequence, smtp.EnhancedCodeInvalidCommand, "Send RCPT first")
		return
	}

	// Send 354 to start data transfer.
	s.reply(smtp.ReplyStartMailInput, smtp.EnhancedCode{}, "Start mail input; end with <CRLF>.<CRLF>")
	s.state = stateData

	// Read the dot-stuffed body.
	reader := s.conn.DotReader()

	if s.server.dataHandler != nil {
		err := s.server.dataHandler.OnData(context.Background(), s.reversePath, s.forwardPaths, reader)
		if err != nil {
			// Drain any unread data.
			io.Copy(io.Discard, reader)
			if smtpErr, ok := err.(*smtp.SMTPError); ok {
				s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
			} else {
				s.reply(smtp.ReplyLocalError, smtp.EnhancedCodeOtherNetwork, "Internal error")
			}
			s.resetTransaction()
			s.state = stateGreeted
			return
		}
	}

	// Drain any unread data (in case handler didn't read it all).
	io.Copy(io.Discard, reader)

	s.reply(smtp.ReplyOK, smtp.EnhancedCodeOK, "Message accepted")
	s.resetTransaction()
	s.state = stateGreeted
}

// handleBDAT processes the BDAT command (RFC 3030).
func (s *session) handleBDAT(args string) {
	if s.state < stateRcpt {
		s.reply(smtp.ReplyBadSequence, smtp.EnhancedCodeInvalidCommand, "Send RCPT first")
		return
	}

	// Parse "SIZE [LAST]".
	parts := strings.Fields(args)
	if len(parts) < 1 {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Syntax: BDAT <size> [LAST]")
		return
	}

	var size int64
	if _, err := fmt.Sscanf(parts[0], "%d", &size); err != nil || size < 0 {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Invalid BDAT size")
		return
	}

	last := len(parts) >= 2 && strings.ToUpper(parts[1]) == "LAST"

	// Read exactly size bytes.
	chunk := make([]byte, size)
	if size > 0 {
		br := s.conn.BufReader()
		if _, err := io.ReadFull(br, chunk); err != nil {
			s.server.logger.Error("BDAT read error", "err", err)
			return
		}
	}

	s.bdatBuffer = append(s.bdatBuffer, chunk...)

	if last {
		// Deliver the accumulated message.
		if s.server.dataHandler != nil {
			r := strings.NewReader(string(s.bdatBuffer))
			err := s.server.dataHandler.OnData(context.Background(), s.reversePath, s.forwardPaths, r)
			if err != nil {
				if smtpErr, ok := err.(*smtp.SMTPError); ok {
					s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
				} else {
					s.reply(smtp.ReplyLocalError, smtp.EnhancedCodeOtherNetwork, "Internal error")
				}
				s.resetTransaction()
				s.state = stateGreeted
				return
			}
		}
		s.reply(smtp.ReplyOK, smtp.EnhancedCodeOK, "Message accepted")
		s.resetTransaction()
		s.state = stateGreeted
	} else {
		s.reply(smtp.ReplyOK, smtp.EnhancedCodeOK, fmt.Sprintf("%d bytes received", size))
	}
}

// handleRSET processes the RSET command (RFC 5321 §4.1.1.5).
func (s *session) handleRSET() {
	s.resetTransaction()
	if s.state > stateGreeted {
		s.state = stateGreeted
	}
	s.reply(smtp.ReplyOK, smtp.EnhancedCodeOK, "Reset ok")
}

// handleNOOP processes the NOOP command (RFC 5321 §4.1.1.9).
func (s *session) handleNOOP() {
	s.reply(smtp.ReplyOK, smtp.EnhancedCodeOK, "OK")
}

// handleQUIT processes the QUIT command (RFC 5321 §4.1.1.10).
func (s *session) handleQUIT() {
	s.reply(smtp.ReplyServiceClosing, smtp.EnhancedCodeOK, fmt.Sprintf("%s closing connection", s.server.hostname))
}

// handleVRFY processes the VRFY command (RFC 5321 §4.1.1.6).
func (s *session) handleVRFY(args string) {
	if s.server.vrfyHandler != nil {
		result, err := s.server.vrfyHandler.OnVrfy(context.Background(), args)
		if err != nil {
			if smtpErr, ok := err.(*smtp.SMTPError); ok {
				s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
			} else {
				s.reply(smtp.ReplyLocalError, smtp.EnhancedCodeOtherNetwork, "Internal error")
			}
			return
		}
		s.reply(smtp.ReplyOK, smtp.EnhancedCodeOK, result)
		return
	}
	// Default: RFC 5321 §7.3 recommends not revealing user information.
	s.reply(smtp.ReplyCannotVRFY, smtp.EnhancedCodeOK, "Cannot VRFY user, but will accept message")
}

// handleAUTH processes the AUTH command (RFC 4954).
func (s *session) handleAUTH(args string) {
	if s.server.authHandler == nil {
		s.reply(smtp.ReplyCommandNotImpl, smtp.EnhancedCodeInvalidCommand, "AUTH not available")
		return
	}
	if s.state < stateGreeted {
		s.reply(smtp.ReplyBadSequence, smtp.EnhancedCodeInvalidCommand, "Send EHLO/HELO first")
		return
	}
	if s.state >= stateMail {
		s.reply(smtp.ReplyBadSequence, smtp.EnhancedCodeInvalidCommand, "AUTH not allowed during mail transaction")
		return
	}
	if s.authenticated {
		s.reply(smtp.ReplyBadSequence, smtp.EnhancedCodeInvalidCommand, "Already authenticated")
		return
	}

	// Parse "MECHANISM [initial-response]".
	mechanism, initialResp, _ := strings.Cut(args, " ")
	mechanism = strings.ToUpper(mechanism)

	switch mechanism {
	case "PLAIN":
		s.authPLAIN(initialResp)
	case "LOGIN":
		s.authLOGIN()
	case "CRAM-MD5":
		s.authCRAMMD5()
	default:
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeInvalidParams, "Unrecognized authentication mechanism")
	}
}

// authPLAIN handles SASL PLAIN authentication (RFC 4616).
func (s *session) authPLAIN(initialResp string) {
	var decoded []byte
	var err error

	if initialResp != "" && initialResp != "=" {
		decoded, err = base64Decode(initialResp)
		if err != nil {
			s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Invalid base64")
			return
		}
	} else {
		// Request initial response.
		s.reply(smtp.ReplyAuthContinue, smtp.EnhancedCode{}, "")
		line, readErr := s.conn.ReadLine(textproto.MaxCommandLineLen)
		if readErr != nil {
			return
		}
		if line == "*" {
			s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeInvalidCommand, "Authentication cancelled")
			return
		}
		decoded, err = base64Decode(line)
		if err != nil {
			s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Invalid base64")
			return
		}
	}

	// PLAIN format: [authzid] NUL authcid NUL passwd
	parts := splitNull(decoded)
	if len(parts) != 3 {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Invalid PLAIN data")
		return
	}
	// parts[0] = authzid (identity), parts[1] = authcid (username), parts[2] = passwd
	username := parts[1]
	password := parts[2]

	if err := s.server.authHandler.Authenticate(context.Background(), "PLAIN", username, password); err != nil {
		if smtpErr, ok := err.(*smtp.SMTPError); ok {
			s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
		} else {
			s.reply(smtp.ReplyAuthFailed, smtp.EnhancedCodeAuthCredentials, "Authentication failed")
		}
		return
	}
	s.authenticated = true
	s.reply(smtp.ReplyAuthOK, smtp.EnhancedCodeOK, "Authentication successful")
}

// authLOGIN handles SASL LOGIN authentication (draft-murchison-sasl-login).
func (s *session) authLOGIN() {
	// Challenge: Username:
	s.reply(smtp.ReplyAuthContinue, smtp.EnhancedCode{}, base64Encode([]byte("Username:")))
	userLine, err := s.conn.ReadLine(textproto.MaxCommandLineLen)
	if err != nil {
		return
	}
	if userLine == "*" {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeInvalidCommand, "Authentication cancelled")
		return
	}
	userBytes, err := base64Decode(userLine)
	if err != nil {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Invalid base64")
		return
	}

	// Challenge: Password:
	s.reply(smtp.ReplyAuthContinue, smtp.EnhancedCode{}, base64Encode([]byte("Password:")))
	passLine, err := s.conn.ReadLine(textproto.MaxCommandLineLen)
	if err != nil {
		return
	}
	if passLine == "*" {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeInvalidCommand, "Authentication cancelled")
		return
	}
	passBytes, err := base64Decode(passLine)
	if err != nil {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Invalid base64")
		return
	}

	if err := s.server.authHandler.Authenticate(context.Background(), "LOGIN", string(userBytes), string(passBytes)); err != nil {
		if smtpErr, ok := err.(*smtp.SMTPError); ok {
			s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
		} else {
			s.reply(smtp.ReplyAuthFailed, smtp.EnhancedCodeAuthCredentials, "Authentication failed")
		}
		return
	}
	s.authenticated = true
	s.reply(smtp.ReplyAuthOK, smtp.EnhancedCodeOK, "Authentication successful")
}

// authCRAMMD5 handles SASL CRAM-MD5 authentication (RFC 2195).
func (s *session) authCRAMMD5() {
	// Generate a challenge.
	challenge := fmt.Sprintf("<%d.%d@%s>", time.Now().UnixNano(), time.Now().Unix(), s.server.hostname)
	s.reply(smtp.ReplyAuthContinue, smtp.EnhancedCode{}, base64Encode([]byte(challenge)))

	line, err := s.conn.ReadLine(textproto.MaxCommandLineLen)
	if err != nil {
		return
	}
	if line == "*" {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeInvalidCommand, "Authentication cancelled")
		return
	}

	decoded, err := base64Decode(line)
	if err != nil {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Invalid base64")
		return
	}

	// Response format: "username digest"
	resp := string(decoded)
	spaceIdx := strings.LastIndex(resp, " ")
	if spaceIdx < 0 {
		s.reply(smtp.ReplySyntaxParamError, smtp.EnhancedCodeSyntaxError, "Invalid CRAM-MD5 response")
		return
	}
	username := resp[:spaceIdx]
	// For CRAM-MD5, we pass the challenge as password so the handler can verify.
	// The handler is expected to compute HMAC-MD5 itself.
	// We pass "challenge:digest" as the password field for the handler to verify.
	digest := resp[spaceIdx+1:]
	password := challenge + ":" + digest

	if err := s.server.authHandler.Authenticate(context.Background(), "CRAM-MD5", username, password); err != nil {
		if smtpErr, ok := err.(*smtp.SMTPError); ok {
			s.reply(smtpErr.Code, smtpErr.EnhancedCode, smtpErr.Message)
		} else {
			s.reply(smtp.ReplyAuthFailed, smtp.EnhancedCodeAuthCredentials, "Authentication failed")
		}
		return
	}
	s.authenticated = true
	s.reply(smtp.ReplyAuthOK, smtp.EnhancedCodeOK, "Authentication successful")
}

// splitNull splits a byte slice on NUL bytes.
func splitNull(data []byte) []string {
	var parts []string
	start := 0
	for i, b := range data {
		if b == 0 {
			parts = append(parts, string(data[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, string(data[start:]))
	return parts
}

func base64Encode(data []byte) string {
	return base64Encoding.EncodeToString(data)
}

func base64Decode(s string) ([]byte, error) {
	return base64Encoding.DecodeString(s)
}

// handleSTARTTLS processes the STARTTLS command (RFC 3207).
// Returns true if the TLS upgrade succeeded and the session should continue.
func (s *session) handleSTARTTLS() bool {
	if s.server.tlsConfig == nil {
		s.reply(smtp.ReplyCommandNotImpl, smtp.EnhancedCodeInvalidCommand, "STARTTLS not available")
		return false
	}
	if s.tls {
		s.reply(smtp.ReplyBadSequence, smtp.EnhancedCodeInvalidCommand, "Already running TLS")
		return false
	}

	s.reply(smtp.ReplyServiceReady, smtp.EnhancedCode{}, "Ready to start TLS")

	// Upgrade the connection.
	tlsConn := tls.Server(s.conn.NetConn(), s.server.tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		s.server.logger.Error("TLS handshake failed", "err", err)
		return false // Connection is likely dead; the main loop will exit on next read.
	}

	// Replace the underlying connection with the TLS connection.
	s.conn.ReplaceConn(tlsConn)
	s.tls = true

	// Reset session state after TLS upgrade (RFC 3207 §4.2).
	s.resetTransaction()
	s.state = stateNew
	s.clientHostname = ""
	s.esmtp = false

	return true
}

// resetTransaction clears the current mail transaction state.
func (s *session) resetTransaction() {
	s.reversePath = smtp.ReversePath{}
	s.forwardPaths = nil
	s.bdatBuffer = nil

	if s.server.resetHandler != nil {
		s.server.resetHandler.OnReset(context.Background())
	}
}
