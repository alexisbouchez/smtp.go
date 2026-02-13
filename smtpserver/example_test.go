package smtpserver_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/smtpserver"
)

// myHandler implements smtpserver.DataHandler for the example.
type myHandler struct{}

func (h *myHandler) OnData(_ context.Context, from smtp.ReversePath, to []smtp.ForwardPath, r io.Reader) error {
	body, _ := io.ReadAll(r)
	fmt.Printf("Received mail from %s to %d recipients (%d bytes)\n",
		from.Mailbox.String(), len(to), len(body))
	return nil
}

func Example() {
	handler := &myHandler{}

	srv := smtpserver.NewServer(
		smtpserver.WithAddr(":2525"),
		smtpserver.WithHostname("mail.example.com"),
		smtpserver.WithDataHandler(handler),
		smtpserver.WithMaxMessageSize(25*1024*1024),
		smtpserver.WithLogger(slog.Default()),
	)

	// srv.ListenAndServe() blocks until shutdown.
	_ = srv
	fmt.Println("Server configured on :2525")
	// Output: Server configured on :2525
}

func Example_withTLS() {
	cert, _ := tls.LoadX509KeyPair("cert.pem", "key.pem")
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	srv := smtpserver.NewServer(
		smtpserver.WithAddr(":465"),
		smtpserver.WithHostname("mail.example.com"),
		smtpserver.WithTLSConfig(tlsConfig),
	)

	_ = srv
	fmt.Println("TLS server configured on :465")
	// Output: TLS server configured on :465
}

func Example_submissionMode() {
	srv := smtpserver.NewServer(
		smtpserver.WithAddr(":587"),
		smtpserver.WithHostname("mail.example.com"),
		smtpserver.WithSubmissionMode(true),
		smtpserver.WithAuthHandler(nil), // Set your auth handler here.
	)

	_ = srv
	fmt.Println("Submission server configured on :587")
	// Output: Submission server configured on :587
}
