package smtpclient

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/smtpserver"
)

// testDataHandler collects delivered messages for test assertions.
type testDataHandler struct {
	mu       sync.Mutex
	messages []testMessage
}

type testMessage struct {
	From smtp.ReversePath
	To   []smtp.ForwardPath
	Body string
}

func (h *testDataHandler) OnData(_ context.Context, from smtp.ReversePath, to []smtp.ForwardPath, r io.Reader) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.messages = append(h.messages, testMessage{From: from, To: to, Body: string(body)})
	h.mu.Unlock()
	return nil
}

func (h *testDataHandler) lastMessage() testMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.messages) == 0 {
		return testMessage{}
	}
	return h.messages[len(h.messages)-1]
}

// startTestServer creates a real TCP server and returns its address.
func startTestServer(t *testing.T, opts ...smtpserver.Option) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	defaults := []smtpserver.Option{
		smtpserver.WithHostname("test.example.com"),
		smtpserver.WithReadTimeout(5 * time.Second),
		smtpserver.WithWriteTimeout(5 * time.Second),
	}
	opts = append(defaults, opts...)
	srv := smtpserver.NewServer(opts...)

	go srv.Serve(ln)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}

	return ln.Addr().String(), cleanup
}

func TestDial(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// Should have extensions from EHLO.
	exts := c.Extensions()
	if exts == nil {
		t.Fatal("expected non-nil extensions after EHLO")
	}
	if !exts.Has(smtp.ExtPIPELINING) {
		t.Error("expected PIPELINING extension")
	}
}

func TestSendMail(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	body := "Subject: Test\r\n\r\nHello from the client!"
	err = c.SendMail(ctx, "sender@example.com", []string{"recipient@example.com"}, strings.NewReader(body))
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	msg := handler.lastMessage()
	if msg.From.Mailbox.String() != "sender@example.com" {
		t.Errorf("From = %q, want %q", msg.From.Mailbox.String(), "sender@example.com")
	}
	if len(msg.To) != 1 || msg.To[0].Mailbox.String() != "recipient@example.com" {
		t.Errorf("To = %v, want [recipient@example.com]", msg.To)
	}
	if !strings.Contains(msg.Body, "Hello from the client!") {
		t.Errorf("Body = %q, missing expected content", msg.Body)
	}
}

func TestSendMail_MultipleRecipients(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	recipients := []string{"alice@example.com", "bob@example.com", "carol@example.com"}
	err = c.SendMail(ctx, "sender@example.com", recipients, strings.NewReader("Subject: Multi\r\n\r\nHello all"))
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	msg := handler.lastMessage()
	if len(msg.To) != 3 {
		t.Fatalf("expected 3 recipients, got %d", len(msg.To))
	}
}

func TestStepByStep(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if err := c.Mail(ctx, "sender@example.com"); err != nil {
		t.Fatalf("Mail: %v", err)
	}
	if err := c.Rcpt(ctx, "user@example.com"); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}
	if err := c.Data(ctx, strings.NewReader("Subject: Step\r\n\r\nStep body")); err != nil {
		t.Fatalf("Data: %v", err)
	}

	msg := handler.lastMessage()
	if !strings.Contains(msg.Body, "Step body") {
		t.Errorf("Body = %q, missing expected content", msg.Body)
	}
}

func TestMultipleTransactions(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// First transaction.
	err = c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader("Message 1"))
	if err != nil {
		t.Fatalf("SendMail 1: %v", err)
	}

	// Second transaction (state auto-resets after DATA on server side).
	err = c.SendMail(ctx, "other@example.com", []string{"user@example.com"}, strings.NewReader("Message 2"))
	if err != nil {
		t.Fatalf("SendMail 2: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(handler.messages))
	}
}

func TestResetBetweenTransactions(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// Start a transaction, then abort it.
	if err := c.Mail(ctx, "sender@example.com"); err != nil {
		t.Fatalf("Mail: %v", err)
	}
	if err := c.Rcpt(ctx, "user@example.com"); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}

	// Reset.
	if err := c.Reset(ctx); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Start a new transaction.
	err = c.SendMail(ctx, "other@example.com", []string{"user@example.com"}, strings.NewReader("After reset"))
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.messages) != 1 {
		t.Fatalf("expected 1 message (first was aborted), got %d", len(handler.messages))
	}
}

func TestNoop(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if err := c.Noop(ctx); err != nil {
		t.Fatalf("Noop: %v", err)
	}
}

func TestRcptRejected(t *testing.T) {
	// Server with a handler that rejects a specific address.
	rcptHandler := &rejectRcptHandler{reject: "bad@example.com"}
	addr, cleanup := startTestServer(t, smtpserver.WithRcptHandler(rcptHandler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if err := c.Mail(ctx, "sender@example.com"); err != nil {
		t.Fatalf("Mail: %v", err)
	}

	err = c.Rcpt(ctx, "bad@example.com")
	if err == nil {
		t.Fatal("expected RCPT to be rejected")
	}
	smtpErr, ok := err.(*smtp.SMTPError)
	if !ok {
		t.Fatalf("expected *smtp.SMTPError, got %T", err)
	}
	if smtpErr.Code != smtp.ReplyMailboxNotFound {
		t.Errorf("code = %d, want %d", smtpErr.Code, smtp.ReplyMailboxNotFound)
	}
}

type rejectRcptHandler struct {
	reject string
}

func (h *rejectRcptHandler) OnRcpt(_ context.Context, to smtp.ForwardPath) error {
	if to.Mailbox.String() == h.reject {
		return &smtp.SMTPError{Code: smtp.ReplyMailboxNotFound, EnhancedCode: smtp.EnhancedCodeBadDest, Message: "User unknown"}
	}
	return nil
}

func TestDialTimeout(t *testing.T) {
	// Listen but never accept to simulate a hanging server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx := context.Background()
	_, err = Dial(ctx, ln.Addr().String(), WithTimeout(100*time.Millisecond))
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestNewClient_WithPipe(t *testing.T) {
	handler := &testDataHandler{}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := smtpserver.NewServer(
		smtpserver.WithHostname("test.example.com"),
		smtpserver.WithDataHandler(handler),
		smtpserver.WithReadTimeout(5*time.Second),
		smtpserver.WithWriteTimeout(5*time.Second),
	)
	go srv.Serve(ln)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	nc, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	c, err := NewClient(nc, "test.local")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	err = c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader("Via NewClient"))
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	msg := handler.lastMessage()
	if !strings.Contains(msg.Body, "Via NewClient") {
		t.Errorf("Body = %q, missing expected content", msg.Body)
	}
}

func TestHELO_Fallback(t *testing.T) {
	// Create a minimal server that rejects EHLO but accepts HELO.
	// We'll use net.Pipe for this custom server.
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		// Minimal HELO-only server.
		buf := make([]byte, 4096)
		serverConn.Write([]byte("220 helo-only.example.com Ready\r\n"))

		// Read EHLO.
		n, _ := serverConn.Read(buf)
		cmd := string(buf[:n])
		if strings.HasPrefix(cmd, "EHLO") {
			// Reject EHLO.
			serverConn.Write([]byte("502 5.5.1 EHLO not supported\r\n"))

			// Read HELO fallback.
			n, _ = serverConn.Read(buf)
			cmd = string(buf[:n])
			if strings.HasPrefix(cmd, "HELO") {
				serverConn.Write([]byte("250 helo-only.example.com Hello\r\n"))
			}
		}

		// Read QUIT.
		n, _ = serverConn.Read(buf)
		cmd = string(buf[:n])
		if strings.HasPrefix(cmd, "QUIT") {
			serverConn.Write([]byte("221 Bye\r\n"))
		}
		serverConn.Close()
	}()

	c, err := NewClient(clientConn, "test.local")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// HELO fallback should have succeeded; no extensions.
	if c.Extensions() != nil {
		t.Error("expected nil extensions with HELO fallback")
	}

	c.Close()
}
