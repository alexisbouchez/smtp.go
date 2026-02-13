// Package smtpserver implements an SMTP server (RFC 5321).
package smtpserver

import (
	"context"
	"io"
	"net"

	"github.com/alexisbouchez/smtp.go"
)

// ConnectionHandler is called when a new client connects. Return a non-nil
// error to reject the connection (e.g., for IP-based filtering).
type ConnectionHandler interface {
	OnConnect(ctx context.Context, conn net.Addr) error
}

// HeloHandler is called when the client sends EHLO or HELO.
type HeloHandler interface {
	OnHelo(ctx context.Context, hostname string) error
}

// MailHandler is called for MAIL FROM commands.
type MailHandler interface {
	OnMail(ctx context.Context, from smtp.ReversePath) error
}

// RcptHandler is called for RCPT TO commands.
type RcptHandler interface {
	OnRcpt(ctx context.Context, to smtp.ForwardPath) error
}

// DataHandler is called when the DATA body has been fully received.
// The reader provides the de-stuffed message body.
type DataHandler interface {
	OnData(ctx context.Context, from smtp.ReversePath, to []smtp.ForwardPath, r io.Reader) error
}

// ResetHandler is called when the transaction state is reset (RSET command
// or implicit reset via EHLO/HELO re-issue).
type ResetHandler interface {
	OnReset(ctx context.Context)
}

// VrfyHandler is called for VRFY commands. If not set, the server responds
// with 252 "Cannot VRFY user, but will accept message".
type VrfyHandler interface {
	OnVrfy(ctx context.Context, param string) (string, error)
}

// AuthHandler authenticates a client. The mechanism is the SASL mechanism
// name (e.g., "PLAIN"), identity is the authorization identity, and
// credentials holds the authentication data (password for PLAIN/LOGIN,
// challenge-response for CRAM-MD5).
type AuthHandler interface {
	Authenticate(ctx context.Context, mechanism string, username string, password string) error
}
