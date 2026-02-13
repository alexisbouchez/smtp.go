// Package smtp provides shared types for the SMTP protocol (RFC 5321).
//
// This package contains reply codes, enhanced status codes, error types,
// email address parsing, SMTP extension definitions, and SASL authentication
// mechanisms. It is used by both the [github.com/alexisbouchez/smtp.go/smtpclient]
// and [github.com/alexisbouchez/smtp.go/smtpserver] packages.
//
// # Reply Codes
//
// [ReplyCode] constants cover all standard SMTP reply codes. The [SMTPError]
// type carries a reply code, optional [EnhancedCode], and human-readable
// message.
//
// # Address Types
//
// [Mailbox], [ReversePath], and [ForwardPath] represent RFC 5321 email
// addresses with full parsing and validation, including support for
// internationalized domain names (RFC 6531).
//
// # Authentication
//
// The [SASLMechanism] interface and its implementations ([PlainAuth],
// [LoginAuth], [CramMD5Auth]) provide client-side SASL authentication.
//
// # Extensions
//
// The [Extension] type and [Extensions] map track EHLO-advertised
// capabilities. Use [ParseEHLOResponse] to parse a server's EHLO reply.
package smtp
