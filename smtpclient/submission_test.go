package smtpclient

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/smtpserver"
)

func TestSubmissionMode_RequiresAuth(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t,
		smtpserver.WithDataHandler(handler),
		smtpserver.WithAuthHandler(&testAuthHandler{}),
		smtpserver.WithSubmissionMode(true),
	)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// MAIL FROM without AUTH should fail with 530.
	err = c.Mail(ctx, "sender@example.com")
	if err == nil {
		t.Fatal("expected MAIL to fail without AUTH in submission mode")
	}
	smtpErr, ok := err.(*smtp.SMTPError)
	if !ok {
		t.Fatalf("expected *smtp.SMTPError, got %T: %v", err, err)
	}
	if smtpErr.Code != smtp.ReplyAuthRequired {
		t.Errorf("code = %d, want %d", smtpErr.Code, smtp.ReplyAuthRequired)
	}
}

func TestSubmissionMode_AuthThenSend(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t,
		smtpserver.WithDataHandler(handler),
		smtpserver.WithAuthHandler(&testAuthHandler{}),
		smtpserver.WithSubmissionMode(true),
	)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// Authenticate first.
	if err := c.Auth(ctx, smtp.PlainAuth("", "testuser", "testpass")); err != nil {
		t.Fatalf("Auth: %v", err)
	}

	// Now MAIL FROM should succeed.
	err = c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader("Submission message"))
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	msg := handler.lastMessage()
	if !strings.Contains(msg.Body, "Submission message") {
		t.Errorf("Body = %q, missing expected content", msg.Body)
	}
}

func TestSubmitMessage(t *testing.T) {
	cert := generateTestCert(t)
	serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}}

	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t,
		smtpserver.WithTLSConfig(serverTLS),
		smtpserver.WithDataHandler(handler),
		smtpserver.WithAuthHandler(&testAuthHandler{}),
		smtpserver.WithSubmissionMode(true),
	)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// Use SubmitMessage convenience method: STARTTLS + AUTH + send.
	clientTLS := &tls.Config{InsecureSkipVerify: true}
	err = c.SubmitMessage(ctx,
		smtp.PlainAuth("", "testuser", "testpass"),
		clientTLS,
		"sender@example.com",
		[]string{"user@example.com"},
		strings.NewReader("Subject: Submit\r\n\r\nSubmitted via SubmitMessage"),
	)
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}

	if !c.IsTLS() {
		t.Error("expected TLS after SubmitMessage")
	}

	msg := handler.lastMessage()
	if !strings.Contains(msg.Body, "Submitted via SubmitMessage") {
		t.Errorf("Body = %q, missing expected content", msg.Body)
	}
}
