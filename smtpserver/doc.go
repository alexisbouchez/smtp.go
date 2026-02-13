// Package smtpserver implements an SMTP server (RFC 5321).
//
// # Quick Start
//
// Create a server with [NewServer] and functional options, then call
// [Server.ListenAndServe]:
//
//	srv := smtpserver.NewServer(
//	    smtpserver.WithAddr(":25"),
//	    smtpserver.WithHostname("mail.example.com"),
//	    smtpserver.WithDataHandler(myHandler),
//	)
//	log.Fatal(srv.ListenAndServe())
//
// # Handler Interfaces
//
// The server calls handler interfaces at each stage of the SMTP
// conversation:
//
//   - [ConnectionHandler] — new TCP connections (IP filtering)
//   - [HeloHandler] — EHLO/HELO commands
//   - [MailHandler] — MAIL FROM commands
//   - [RcptHandler] — RCPT TO commands (recipient validation)
//   - [DataHandler] — message body delivery
//   - [ResetHandler] — RSET or implicit transaction reset
//   - [VrfyHandler] — VRFY commands
//   - [AuthHandler] — SASL authentication
//
// All handlers are optional. Return an [smtp.SMTPError] from any handler
// to send a custom reply code and message to the client.
//
// # Extensions
//
// The server automatically advertises: PIPELINING, 8BITMIME,
// ENHANCEDSTATUSCODES, DSN, SMTPUTF8, CHUNKING, SIZE (if configured),
// STARTTLS (if TLS configured), and AUTH (if handler set).
//
// # Message Submission (RFC 6409)
//
// Enable [WithSubmissionMode] to require authentication before MAIL FROM.
//
// # Graceful Shutdown
//
// Call [Server.Shutdown] with a context deadline to stop accepting
// new connections and wait for existing sessions to finish.
package smtpserver
