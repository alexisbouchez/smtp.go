// Package smtpclient implements an SMTP client (RFC 5321).
package smtpclient

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/internal/textproto"
)

// Client is an SMTP client for sending mail.
type Client struct {
	conn      *textproto.Conn
	netConn   net.Conn
	hostname  string // Server hostname from greeting.
	localName string // Client identity for EHLO.
	exts      smtp.Extensions
	logger    *slog.Logger
	tls       bool
}

// Option configures a Client.
type Option func(*options)

type options struct {
	dialer    *net.Dialer
	timeout   time.Duration
	localName string
	tlsConfig *tls.Config
	logger    *slog.Logger
}

// WithDialer sets a custom net.Dialer for the connection.
func WithDialer(d *net.Dialer) Option {
	return func(o *options) { o.dialer = d }
}

// WithTimeout sets the overall timeout for dial + greeting.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithLocalName sets the hostname used in EHLO.
func WithLocalName(name string) Option {
	return func(o *options) { o.localName = name }
}

// WithTLSConfig sets the TLS configuration for STARTTLS.
func WithTLSConfig(c *tls.Config) Option {
	return func(o *options) { o.tlsConfig = c }
}

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// Dial connects to the SMTP server at addr, reads the greeting, and sends EHLO.
// It falls back to HELO if EHLO is rejected.
func Dial(ctx context.Context, addr string, opts ...Option) (*Client, error) {
	o := &options{
		dialer:    &net.Dialer{},
		timeout:   30 * time.Second,
		localName: "localhost",
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(o)
	}

	// Apply timeout to the entire dial+greeting+EHLO sequence.
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	nc, err := o.dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("smtp: dial %s: %w", addr, err)
	}

	c := &Client{
		conn:      textproto.NewConn(nc),
		netConn:   nc,
		localName: o.localName,
		logger:    o.logger,
	}

	c.conn.SetDeadlineFromContext(ctx)

	// Read greeting (RFC 5321 §4.3.1).
	reply, err := c.conn.ReadReply()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("smtp: reading greeting: %w", err)
	}
	if reply.Code != int(smtp.ReplyServiceReady) {
		nc.Close()
		return nil, replyToError(reply)
	}

	if len(reply.Lines) > 0 {
		c.hostname = reply.Lines[0]
	}

	// Send EHLO, fall back to HELO if rejected.
	if err := c.ehlo(ctx); err != nil {
		nc.Close()
		return nil, err
	}

	return c, nil
}

// NewClient wraps an existing net.Conn as an SMTP client. The caller is
// responsible for having already established the connection. The greeting
// must not have been read yet.
func NewClient(nc net.Conn, localName string) (*Client, error) {
	c := &Client{
		conn:      textproto.NewConn(nc),
		netConn:   nc,
		localName: localName,
		logger:    slog.Default(),
	}

	// Read greeting.
	reply, err := c.conn.ReadReply()
	if err != nil {
		return nil, fmt.Errorf("smtp: reading greeting: %w", err)
	}
	if reply.Code != int(smtp.ReplyServiceReady) {
		return nil, replyToError(reply)
	}

	if len(reply.Lines) > 0 {
		c.hostname = reply.Lines[0]
	}

	// EHLO with HELO fallback.
	if err := c.ehlo(context.Background()); err != nil {
		return nil, err
	}

	return c, nil
}

// ehlo sends EHLO and falls back to HELO if rejected (RFC 5321 §4.1.1.1).
func (c *Client) ehlo(ctx context.Context) error {
	c.conn.SetDeadlineFromContext(ctx)

	reply, err := c.conn.Cmd("EHLO %s", c.localName)
	if err != nil {
		return fmt.Errorf("smtp: EHLO: %w", err)
	}

	if reply.Code == int(smtp.ReplyOK) {
		c.exts = smtp.ParseEHLOResponse(reply.Lines)
		return nil
	}

	// EHLO rejected — try HELO.
	if reply.Code == int(smtp.ReplySyntaxError) || reply.Code == int(smtp.ReplyCommandNotImpl) {
		reply, err = c.conn.Cmd("HELO %s", c.localName)
		if err != nil {
			return fmt.Errorf("smtp: HELO: %w", err)
		}
		if reply.Code != int(smtp.ReplyOK) {
			return replyToError(reply)
		}
		c.exts = nil // No extensions with HELO.
		return nil
	}

	return replyToError(reply)
}

// Extensions returns the extensions advertised by the server in the last
// EHLO response. Returns nil if the server only supports HELO.
func (c *Client) Extensions() smtp.Extensions {
	return c.exts
}

// Mail sends the MAIL FROM command with optional extension parameters
// (RFC 5321 §4.1.1.2, RFC 1870 SIZE, RFC 6152 8BITMIME, RFC 6531 SMTPUTF8, RFC 3461 DSN).
func (c *Client) Mail(ctx context.Context, from string, opts ...MailOption) error {
	c.conn.SetDeadlineFromContext(ctx)

	cmd := fmt.Sprintf("MAIL FROM:<%s>", from)

	var mo mailOptions
	for _, opt := range opts {
		opt(&mo)
	}
	if mo.size > 0 {
		cmd += fmt.Sprintf(" SIZE=%d", mo.size)
	}
	if mo.body != "" {
		cmd += fmt.Sprintf(" BODY=%s", mo.body)
	}
	if mo.smtpUTF8 {
		cmd += " SMTPUTF8"
	}
	if mo.dsnRet != "" {
		cmd += fmt.Sprintf(" RET=%s", mo.dsnRet)
	}
	if mo.dsnEnvID != "" {
		cmd += fmt.Sprintf(" ENVID=%s", mo.dsnEnvID)
	}

	if err := c.conn.WriteLine(cmd); err != nil {
		return fmt.Errorf("smtp: MAIL FROM: %w", err)
	}
	reply, err := c.conn.ReadReply()
	if err != nil {
		return fmt.Errorf("smtp: MAIL FROM: %w", err)
	}
	if reply.Code != int(smtp.ReplyOK) {
		return replyToError(reply)
	}
	return nil
}

// Rcpt sends the RCPT TO command with optional extension parameters
// (RFC 5321 §4.1.1.3, RFC 3461 DSN).
func (c *Client) Rcpt(ctx context.Context, to string, opts ...RcptOption) error {
	c.conn.SetDeadlineFromContext(ctx)

	cmd := fmt.Sprintf("RCPT TO:<%s>", to)

	var ro rcptOptions
	for _, opt := range opts {
		opt(&ro)
	}
	if ro.dsnNotify != "" {
		cmd += fmt.Sprintf(" NOTIFY=%s", ro.dsnNotify)
	}
	if ro.dsnOrcpt != "" {
		cmd += fmt.Sprintf(" ORCPT=%s", ro.dsnOrcpt)
	}

	if err := c.conn.WriteLine(cmd); err != nil {
		return fmt.Errorf("smtp: RCPT TO: %w", err)
	}
	reply, err := c.conn.ReadReply()
	if err != nil {
		return fmt.Errorf("smtp: RCPT TO: %w", err)
	}
	if reply.Code != int(smtp.ReplyOK) {
		return replyToError(reply)
	}
	return nil
}

// ServerMaxSize returns the maximum message size advertised by the server
// via the SIZE extension (RFC 1870), or 0 if not advertised.
func (c *Client) ServerMaxSize() int64 {
	if c.exts == nil {
		return 0
	}
	param := c.exts.Param(smtp.ExtSIZE)
	if param == "" {
		return 0
	}
	var n int64
	fmt.Sscanf(param, "%d", &n)
	return n
}

// Data sends the DATA command and streams the message body from r.
// The body is dot-stuffed automatically (RFC 5321 §4.1.1.4).
func (c *Client) Data(ctx context.Context, r io.Reader) error {
	c.conn.SetDeadlineFromContext(ctx)

	reply, err := c.conn.Cmd("DATA")
	if err != nil {
		return fmt.Errorf("smtp: DATA: %w", err)
	}
	if reply.Code != int(smtp.ReplyStartMailInput) {
		return replyToError(reply)
	}

	// Stream body through dot writer.
	dw := c.conn.DotWriter()
	if _, err := io.Copy(dw, r); err != nil {
		dw.Close()
		return fmt.Errorf("smtp: writing DATA body: %w", err)
	}
	if err := dw.Close(); err != nil {
		return fmt.Errorf("smtp: closing DATA body: %w", err)
	}

	// Read final reply.
	reply, err = c.conn.ReadReply()
	if err != nil {
		return fmt.Errorf("smtp: reading DATA reply: %w", err)
	}
	if reply.Code != int(smtp.ReplyOK) {
		return replyToError(reply)
	}
	return nil
}

// Bdat sends a BDAT chunk (RFC 3030). Set last=true for the final chunk.
func (c *Client) Bdat(ctx context.Context, data []byte, last bool) error {
	c.conn.SetDeadlineFromContext(ctx)

	cmd := fmt.Sprintf("BDAT %d", len(data))
	if last {
		cmd += " LAST"
	}
	if err := c.conn.WriteLine(cmd); err != nil {
		return fmt.Errorf("smtp: BDAT: %w", err)
	}

	// Write the raw data (no dot-stuffing for BDAT).
	bw := c.conn.BufWriter()
	if _, err := bw.Write(data); err != nil {
		return fmt.Errorf("smtp: BDAT write: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("smtp: BDAT flush: %w", err)
	}

	// Read reply.
	reply, err := c.conn.ReadReply()
	if err != nil {
		return fmt.Errorf("smtp: BDAT reply: %w", err)
	}
	if reply.Code != int(smtp.ReplyOK) {
		return replyToError(reply)
	}
	return nil
}

// StartTLS sends the STARTTLS command and upgrades the connection to TLS
// (RFC 3207). After a successful upgrade, it re-issues EHLO to refresh
// the server's extension list.
func (c *Client) StartTLS(ctx context.Context, config *tls.Config) error {
	c.conn.SetDeadlineFromContext(ctx)

	reply, err := c.conn.Cmd("STARTTLS")
	if err != nil {
		return fmt.Errorf("smtp: STARTTLS: %w", err)
	}
	if reply.Code != int(smtp.ReplyServiceReady) {
		return replyToError(reply)
	}

	// Upgrade to TLS.
	tlsConn := tls.Client(c.netConn, config)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("smtp: TLS handshake: %w", err)
	}

	c.netConn = tlsConn
	c.conn.ReplaceConn(tlsConn)
	c.tls = true

	// Re-issue EHLO after TLS upgrade (RFC 3207 §4.2).
	return c.ehlo(ctx)
}

// IsTLS reports whether the connection is using TLS.
func (c *Client) IsTLS() bool {
	return c.tls
}

// Auth performs SASL authentication using the given mechanism (RFC 4954).
func (c *Client) Auth(ctx context.Context, mech smtp.SASLMechanism) error {
	c.conn.SetDeadlineFromContext(ctx)

	// Start the mechanism.
	initialResp, err := mech.Start()
	if err != nil {
		return fmt.Errorf("smtp: auth start: %w", err)
	}

	// Send AUTH command with optional initial response.
	var cmd string
	if initialResp != nil {
		cmd = fmt.Sprintf("AUTH %s %s", mech.Name(), base64.StdEncoding.EncodeToString(initialResp))
	} else {
		cmd = fmt.Sprintf("AUTH %s", mech.Name())
	}
	if err := c.conn.WriteLine(cmd); err != nil {
		return fmt.Errorf("smtp: auth write: %w", err)
	}

	// Process challenge/response loop.
	for {
		reply, err := c.conn.ReadReply()
		if err != nil {
			return fmt.Errorf("smtp: auth read: %w", err)
		}

		if reply.Code == int(smtp.ReplyAuthOK) {
			return nil // Authentication succeeded.
		}

		if reply.Code != int(smtp.ReplyAuthContinue) {
			return replyToError(reply)
		}

		// Decode the server challenge.
		challengeStr := ""
		if len(reply.Lines) > 0 {
			challengeStr = reply.Lines[0]
		}
		challenge, err := base64.StdEncoding.DecodeString(challengeStr)
		if err != nil {
			return fmt.Errorf("smtp: auth decode challenge: %w", err)
		}

		// Get client response.
		resp, err := mech.Next(challenge)
		if err != nil {
			// Cancel authentication.
			c.conn.WriteLine("*")
			c.conn.ReadReply()
			return fmt.Errorf("smtp: auth mechanism: %w", err)
		}

		// Send response.
		encoded := base64.StdEncoding.EncodeToString(resp)
		if err := c.conn.WriteLine(encoded); err != nil {
			return fmt.Errorf("smtp: auth response: %w", err)
		}
	}
}

// SubmitMessage performs STARTTLS (if available), AUTH, and then sends the
// message. This is the typical workflow for message submission (RFC 6409, port 587).
// If the connection is already TLS, the STARTTLS step is skipped.
func (c *Client) SubmitMessage(ctx context.Context, mech smtp.SASLMechanism, tlsConfig *tls.Config, from string, to []string, r io.Reader) error {
	// Step 1: STARTTLS if available and not already on TLS.
	if !c.tls && c.exts.Has(smtp.ExtSTARTTLS) && tlsConfig != nil {
		if err := c.StartTLS(ctx, tlsConfig); err != nil {
			return fmt.Errorf("smtp: submission STARTTLS: %w", err)
		}
	}

	// Step 2: Authenticate.
	if err := c.Auth(ctx, mech); err != nil {
		return fmt.Errorf("smtp: submission AUTH: %w", err)
	}

	// Step 3: Send the message.
	return c.SendMail(ctx, from, to, r)
}

// SendMail is a convenience method that performs MAIL FROM, RCPT TO for each
// recipient, and DATA in a single call.
func (c *Client) SendMail(ctx context.Context, from string, to []string, r io.Reader) error {
	if err := c.Mail(ctx, from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(ctx, rcpt); err != nil {
			return err
		}
	}
	return c.Data(ctx, r)
}

// Reset sends the RSET command to abort the current transaction (RFC 5321 §4.1.1.5).
func (c *Client) Reset(ctx context.Context) error {
	c.conn.SetDeadlineFromContext(ctx)

	reply, err := c.conn.Cmd("RSET")
	if err != nil {
		return fmt.Errorf("smtp: RSET: %w", err)
	}
	if reply.Code != int(smtp.ReplyOK) {
		return replyToError(reply)
	}
	return nil
}

// Noop sends a NOOP command as a keepalive (RFC 5321 §4.1.1.9).
func (c *Client) Noop(ctx context.Context) error {
	c.conn.SetDeadlineFromContext(ctx)

	reply, err := c.conn.Cmd("NOOP")
	if err != nil {
		return fmt.Errorf("smtp: NOOP: %w", err)
	}
	if reply.Code != int(smtp.ReplyOK) {
		return replyToError(reply)
	}
	return nil
}

// Close sends QUIT and closes the connection (RFC 5321 §4.1.1.10).
func (c *Client) Close() error {
	c.conn.Cmd("QUIT") // Best effort; ignore errors.
	return c.netConn.Close()
}

// replyToError converts a textproto.Reply to an SMTPError.
func replyToError(reply textproto.Reply) *smtp.SMTPError {
	msg := strings.Join(reply.Lines, "\n")

	// Try to extract enhanced code from first line.
	enhanced := smtp.EnhancedCode{}
	if len(reply.Lines) > 0 {
		cl, su, de, rest := textproto.ParseEnhancedCode(reply.Lines[0])
		if cl != 0 {
			enhanced = smtp.EnhancedCode{Class: cl, Subject: su, Detail: de}
			if len(reply.Lines) == 1 {
				msg = rest
			}
		}
	}

	return &smtp.SMTPError{
		Code:         smtp.ReplyCode(reply.Code),
		EnhancedCode: enhanced,
		Message:      msg,
	}
}
