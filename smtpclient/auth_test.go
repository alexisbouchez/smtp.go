package smtpclient

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/smtpserver"
)

// testAuthHandler accepts user/pass = "testuser"/"testpass".
type testAuthHandler struct{}

func (h *testAuthHandler) Authenticate(_ context.Context, mechanism, username, password string) error {
	switch mechanism {
	case "PLAIN", "LOGIN":
		if username == "testuser" && password == "testpass" {
			return nil
		}
		return &smtp.SMTPError{Code: smtp.ReplyAuthFailed, EnhancedCode: smtp.EnhancedCodeAuthCredentials, Message: "Bad credentials"}
	case "CRAM-MD5":
		// For CRAM-MD5, password contains "challenge:digest". We just accept for testing.
		if username == "testuser" {
			return nil
		}
		return &smtp.SMTPError{Code: smtp.ReplyAuthFailed, EnhancedCode: smtp.EnhancedCodeAuthCredentials, Message: "Bad credentials"}
	default:
		return &smtp.SMTPError{Code: smtp.ReplySyntaxParamError, EnhancedCode: smtp.EnhancedCodeInvalidParams, Message: "Unknown mechanism"}
	}
}

func TestAuth_PLAIN(t *testing.T) {
	handler := &testDataHandler{}
	authHandler := &testAuthHandler{}
	addr, cleanup := startTestServer(t,
		smtpserver.WithAuthHandler(authHandler),
		smtpserver.WithDataHandler(handler),
	)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// AUTH should be advertised.
	if !c.Extensions().Has(smtp.ExtAUTH) {
		t.Fatal("AUTH not advertised")
	}

	// Authenticate with PLAIN.
	err = c.Auth(ctx, smtp.PlainAuth("", "testuser", "testpass"))
	if err != nil {
		t.Fatalf("Auth PLAIN: %v", err)
	}

	// Send mail after auth.
	err = c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader("Authenticated message"))
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	msg := handler.lastMessage()
	if !strings.Contains(msg.Body, "Authenticated message") {
		t.Errorf("Body = %q, missing expected content", msg.Body)
	}
}

func TestAuth_PLAIN_BadCredentials(t *testing.T) {
	authHandler := &testAuthHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithAuthHandler(authHandler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	err = c.Auth(ctx, smtp.PlainAuth("", "testuser", "wrongpass"))
	if err == nil {
		t.Fatal("expected auth failure")
	}
	smtpErr, ok := err.(*smtp.SMTPError)
	if !ok {
		t.Fatalf("expected *smtp.SMTPError, got %T: %v", err, err)
	}
	if smtpErr.Code != smtp.ReplyAuthFailed {
		t.Errorf("code = %d, want %d", smtpErr.Code, smtp.ReplyAuthFailed)
	}
}

func TestAuth_LOGIN(t *testing.T) {
	handler := &testDataHandler{}
	authHandler := &testAuthHandler{}
	addr, cleanup := startTestServer(t,
		smtpserver.WithAuthHandler(authHandler),
		smtpserver.WithDataHandler(handler),
	)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	err = c.Auth(ctx, smtp.LoginAuth("testuser", "testpass"))
	if err != nil {
		t.Fatalf("Auth LOGIN: %v", err)
	}

	err = c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader("LOGIN message"))
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}
}

func TestAuth_CRAMMD5(t *testing.T) {
	authHandler := &testAuthHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithAuthHandler(authHandler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	err = c.Auth(ctx, smtp.CramMD5Auth("testuser", "secret"))
	if err != nil {
		t.Fatalf("Auth CRAM-MD5: %v", err)
	}
}

func TestAuth_NotAvailable(t *testing.T) {
	// Server without auth handler.
	addr, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if c.Extensions().Has(smtp.ExtAUTH) {
		t.Fatal("AUTH should not be advertised without handler")
	}
}
