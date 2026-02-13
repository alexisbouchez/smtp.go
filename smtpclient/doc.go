// Package smtpclient implements an SMTP client (RFC 5321).
//
// # Quick Start
//
// Use [Dial] to connect to an SMTP server, then call [Client.SendMail]
// to send a message:
//
//	c, err := smtpclient.Dial(ctx, "mail.example.com:25")
//	if err != nil { ... }
//	defer c.Close()
//	err = c.SendMail(ctx, "from@example.com", []string{"to@example.com"}, body)
//
// # Message Submission (RFC 6409)
//
// For port 587 submission with STARTTLS and authentication, use
// [Client.SubmitMessage]:
//
//	err = c.SubmitMessage(ctx, smtp.PlainAuth("", user, pass), tlsCfg,
//	    "from@example.com", []string{"to@example.com"}, body)
//
// # Step-by-Step API
//
// For fine-grained control, use [Client.Mail], [Client.Rcpt], and
// [Client.Data] individually. Options like [WithSize], [WithBody],
// and DSN parameters can be passed to Mail and Rcpt.
//
// # STARTTLS
//
// Call [Client.StartTLS] to upgrade an existing connection to TLS.
// After a successful upgrade, the client re-issues EHLO automatically.
//
// # Authentication
//
// Call [Client.Auth] with any [smtp.SASLMechanism] (PLAIN, LOGIN, CRAM-MD5).
//
// # CHUNKING (RFC 3030)
//
// Call [Client.Bdat] to send message data in binary chunks without
// dot-stuffing.
package smtpclient
