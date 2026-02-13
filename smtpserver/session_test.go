package smtpserver

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexisbouchez/smtp.go"
)

// testDataHandler collects delivered messages for assertions.
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

// testRcptHandler rejects a specific address.
type testRcptHandler struct {
	reject string
}

func (h *testRcptHandler) OnRcpt(_ context.Context, to smtp.ForwardPath) error {
	if to.Mailbox.String() == h.reject {
		return &smtp.SMTPError{Code: smtp.ReplyMailboxNotFound, EnhancedCode: smtp.EnhancedCodeBadDest, Message: "User unknown"}
	}
	return nil
}

// smtpConversation is a helper for sending commands and reading replies via net.Pipe.
type smtpConversation struct {
	t      *testing.T
	reader *bufio.Reader
	writer *bufio.Writer
	conn   net.Conn
}

func newConversation(t *testing.T, conn net.Conn) *smtpConversation {
	return &smtpConversation{
		t:      t,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
		conn:   conn,
	}
}

func (c *smtpConversation) readLine() string {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := c.reader.ReadString('\n')
	if err != nil {
		c.t.Fatalf("readLine: %v", err)
	}
	return strings.TrimRight(line, "\r\n")
}

func (c *smtpConversation) readReply() (int, []string) {
	c.t.Helper()
	var lines []string
	for {
		line := c.readLine()
		if len(line) < 3 {
			c.t.Fatalf("reply line too short: %q", line)
		}
		code := 0
		fmt.Sscanf(line[:3], "%d", &code)

		if len(line) == 3 {
			lines = append(lines, "")
			return code, lines
		}

		sep := line[3]
		text := line[4:]

		lines = append(lines, text)
		if sep == ' ' {
			return code, lines
		}
	}
}

func (c *smtpConversation) expectCode(wantCode int) []string {
	c.t.Helper()
	code, lines := c.readReply()
	if code != wantCode {
		c.t.Fatalf("expected %d, got %d: %v", wantCode, code, lines)
	}
	return lines
}

func (c *smtpConversation) send(line string) {
	c.t.Helper()
	c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err := c.writer.WriteString(line + "\r\n")
	if err != nil {
		c.t.Fatalf("send: %v", err)
	}
	c.writer.Flush()
}

func (c *smtpConversation) sendData(body string) {
	c.t.Helper()
	c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	// Write each line, dot-stuffing as needed.
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, ".") {
			c.writer.WriteString(".")
		}
		c.writer.WriteString(line + "\r\n")
	}
	c.writer.WriteString(".\r\n")
	c.writer.Flush()
}

// startTestServer creates a server on a net.Pipe and returns the client side.
func startTestServer(t *testing.T, opts ...Option) (net.Conn, *Server) {
	t.Helper()
	clientConn, serverConn := net.Pipe()

	defaults := []Option{
		WithHostname("test.example.com"),
		WithReadTimeout(5 * time.Second),
		WithWriteTimeout(5 * time.Second),
	}
	opts = append(defaults, opts...)
	srv := NewServer(opts...)

	go srv.handleConn(serverConn)
	return clientConn, srv
}

func TestFullConversation(t *testing.T) {
	handler := &testDataHandler{}
	clientConn, _ := startTestServer(t, WithDataHandler(handler))
	defer clientConn.Close()

	c := newConversation(t, clientConn)

	// Read greeting.
	c.expectCode(220)

	// EHLO.
	c.send("EHLO client.example.com")
	c.expectCode(250)

	// MAIL FROM.
	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	// RCPT TO.
	c.send("RCPT TO:<recipient@example.com>")
	c.expectCode(250)

	// DATA.
	c.send("DATA")
	c.expectCode(354)

	// Message body.
	c.sendData("Subject: Test\r\n\r\nHello, World!")

	// Accept.
	c.expectCode(250)

	// Check delivered message.
	msg := handler.lastMessage()
	if msg.From.Mailbox.String() != "sender@example.com" {
		t.Errorf("From = %q, want %q", msg.From.Mailbox.String(), "sender@example.com")
	}
	if len(msg.To) != 1 || msg.To[0].Mailbox.String() != "recipient@example.com" {
		t.Errorf("To = %v, want [recipient@example.com]", msg.To)
	}
	if !strings.Contains(msg.Body, "Hello, World!") {
		t.Errorf("Body = %q, want to contain %q", msg.Body, "Hello, World!")
	}

	// QUIT.
	c.send("QUIT")
	c.expectCode(221)
}

func TestStateEnforcement_DataBeforeMail(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220) // greeting

	c.send("EHLO test")
	c.expectCode(250)

	// DATA without MAIL → 503.
	c.send("DATA")
	c.expectCode(503)
}

func TestStateEnforcement_RcptBeforeMail(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	// RCPT without MAIL → 503.
	c.send("RCPT TO:<user@example.com>")
	c.expectCode(503)
}

func TestStateEnforcement_MailBeforeEhlo(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	// MAIL without EHLO → 503.
	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(503)
}

func TestStateEnforcement_DataBeforeRcpt(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	// DATA without RCPT → 503.
	c.send("DATA")
	c.expectCode(503)
}

func TestMaxRecipients(t *testing.T) {
	clientConn, _ := startTestServer(t, WithMaxRecipients(2))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	c.send("RCPT TO:<user1@example.com>")
	c.expectCode(250)

	c.send("RCPT TO:<user2@example.com>")
	c.expectCode(250)

	// Third recipient → 452.
	c.send("RCPT TO:<user3@example.com>")
	c.expectCode(452)
}

func TestRSET(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	c.send("RCPT TO:<user@example.com>")
	c.expectCode(250)

	// RSET clears state.
	c.send("RSET")
	c.expectCode(250)

	// RCPT should fail now (no MAIL).
	c.send("RCPT TO:<user@example.com>")
	c.expectCode(503)

	// Can start fresh.
	c.send("MAIL FROM:<other@example.com>")
	c.expectCode(250)
}

func TestHELO(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("HELO test")
	c.expectCode(250)

	// Can proceed with MAIL.
	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)
}

func TestNOOP(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("NOOP")
	c.expectCode(250)
}

func TestVRFY(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("VRFY user")
	c.expectCode(252)
}

func TestUnknownCommand(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("GIBBERISH")
	c.expectCode(500)
}

func TestRcptHandlerReject(t *testing.T) {
	handler := &testRcptHandler{reject: "bad@example.com"}
	clientConn, _ := startTestServer(t, WithRcptHandler(handler))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	// This one should be rejected.
	c.send("RCPT TO:<bad@example.com>")
	c.expectCode(550)

	// This one should be accepted.
	c.send("RCPT TO:<good@example.com>")
	c.expectCode(250)
}

func TestEHLO_ReissueClearsState(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	// Re-issue EHLO → resets state.
	c.send("EHLO test2")
	c.expectCode(250)

	// RCPT should fail (MAIL was cleared).
	c.send("RCPT TO:<user@example.com>")
	c.expectCode(503)
}

func TestMultipleTransactions(t *testing.T) {
	handler := &testDataHandler{}
	clientConn, _ := startTestServer(t, WithDataHandler(handler))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	// First transaction.
	c.send("MAIL FROM:<sender1@example.com>")
	c.expectCode(250)
	c.send("RCPT TO:<user@example.com>")
	c.expectCode(250)
	c.send("DATA")
	c.expectCode(354)
	c.sendData("Message 1")
	c.expectCode(250)

	// Second transaction (no RSET needed — state auto-resets after DATA).
	c.send("MAIL FROM:<sender2@example.com>")
	c.expectCode(250)
	c.send("RCPT TO:<user@example.com>")
	c.expectCode(250)
	c.send("DATA")
	c.expectCode(354)
	c.sendData("Message 2")
	c.expectCode(250)

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(handler.messages))
	}
}

func TestEHLO_AdvertisesExtensions(t *testing.T) {
	clientConn, _ := startTestServer(t, WithMaxMessageSize(50*1024*1024))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	_, lines := c.readReply()

	found := map[string]bool{}
	for _, line := range lines {
		keyword, _, _ := strings.Cut(line, " ")
		found[keyword] = true
	}

	for _, want := range []string{"PIPELINING", "8BITMIME", "ENHANCEDSTATUSCODES"} {
		if !found[want] {
			t.Errorf("EHLO should advertise %s", want)
		}
	}

	// SIZE should include the value.
	sizeFound := false
	for _, line := range lines {
		if strings.HasPrefix(line, "SIZE ") {
			sizeFound = true
		}
	}
	if !sizeFound {
		t.Error("EHLO should advertise SIZE with value")
	}
}

func TestConcurrentSessions(t *testing.T) {
	handler := &testDataHandler{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	srv := NewServer(
		WithHostname("test.example.com"),
		WithDataHandler(handler),
		WithReadTimeout(5*time.Second),
		WithWriteTimeout(5*time.Second),
	)

	go srv.Serve(ln)
	defer srv.Close()

	const numClients = 5
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := range numClients {
		go func(id int) {
			defer wg.Done()

			conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
			if err != nil {
				t.Errorf("client %d dial: %v", id, err)
				return
			}
			defer conn.Close()

			c := newConversation(t, conn)
			c.expectCode(220)
			c.send(fmt.Sprintf("EHLO client%d", id))
			c.expectCode(250)
			c.send(fmt.Sprintf("MAIL FROM:<sender%d@example.com>", id))
			c.expectCode(250)
			c.send(fmt.Sprintf("RCPT TO:<user@example.com>"))
			c.expectCode(250)
			c.send("DATA")
			c.expectCode(354)
			c.sendData(fmt.Sprintf("Message from client %d", id))
			c.expectCode(250)
			c.send("QUIT")
			c.expectCode(221)
		}(i)
	}

	wg.Wait()

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.messages) != numClients {
		t.Errorf("expected %d messages, got %d", numClients, len(handler.messages))
	}
}

func TestGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(
		WithHostname("test.example.com"),
		WithReadTimeout(5*time.Second),
		WithWriteTimeout(5*time.Second),
	)

	serveDone := make(chan struct{})
	go func() {
		srv.Serve(ln)
		close(serveDone)
	}()

	// Connect a client.
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	c := newConversation(t, conn)
	c.expectCode(220)

	// Shutdown server.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown: %v", err)
	}

	// Serve should have returned.
	select {
	case <-serveDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after Shutdown")
	}
}

func TestNullReversePath(t *testing.T) {
	handler := &testDataHandler{}
	clientConn, _ := startTestServer(t, WithDataHandler(handler))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	// Null reverse path for bounces.
	c.send("MAIL FROM:<>")
	c.expectCode(250)

	c.send("RCPT TO:<user@example.com>")
	c.expectCode(250)

	c.send("DATA")
	c.expectCode(354)
	c.sendData("Bounce message")
	c.expectCode(250)

	msg := handler.lastMessage()
	if !msg.From.Null {
		t.Error("expected null reverse path")
	}
}

func TestMaxInvalidCommands_Disconnect(t *testing.T) {
	clientConn, _ := startTestServer(t, WithMaxInvalidCommands(3))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("GIBBERISH1")
	c.expectCode(500)

	c.send("GIBBERISH2")
	c.expectCode(500)

	// Third invalid command triggers disconnect.
	c.send("GIBBERISH3")
	c.expectCode(500) // The error reply.
	c.expectCode(421) // Followed by disconnect notice.
}

func TestNULByte_Rejected(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO\x00test")
	c.expectCode(500)
}

func TestMaxConnections(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(
		WithHostname("test.example.com"),
		WithReadTimeout(5*time.Second),
		WithWriteTimeout(5*time.Second),
		WithMaxConnections(2),
	)

	go srv.Serve(ln)
	defer srv.Close()

	// First two connections should succeed.
	conn1, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()
	c1 := newConversation(t, conn1)
	c1.expectCode(220)

	conn2, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()
	c2 := newConversation(t, conn2)
	c2.expectCode(220)

	// Third connection should be rejected with 421.
	conn3, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn3.Close()
	c3 := newConversation(t, conn3)
	c3.expectCode(421)

	// Close one connection, then a new one should succeed.
	c1.send("QUIT")
	c1.expectCode(221)
	conn1.Close()

	// Give the server time to release the slot.
	time.Sleep(100 * time.Millisecond)

	conn4, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn4.Close()
	c4 := newConversation(t, conn4)
	c4.expectCode(220)
}

func TestSubmissionMode_RejectsUnauthenticated(t *testing.T) {
	clientConn, _ := startTestServer(t,
		WithAuthHandler(&testAuthHandler{}),
		WithSubmissionMode(true),
	)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	// MAIL without AUTH → 530 in submission mode.
	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(530)
}

// testAuthHandler for server-side tests.
type testAuthHandler struct{}

func (h *testAuthHandler) Authenticate(_ context.Context, mechanism, username, password string) error {
	if username == "testuser" && (password == "testpass" || mechanism == "CRAM-MD5") {
		return nil
	}
	return &smtp.SMTPError{Code: smtp.ReplyAuthFailed, EnhancedCode: smtp.EnhancedCodeAuthCredentials, Message: "Bad credentials"}
}

func TestSubmissionMode_AuthThenMail(t *testing.T) {
	clientConn, _ := startTestServer(t,
		WithAuthHandler(&testAuthHandler{}),
		WithSubmissionMode(true),
		WithDataHandler(&testDataHandler{}),
	)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	// Authenticate.
	c.send("AUTH PLAIN AHRlc3R1c2VyAHRlc3RwYXNz") // \x00testuser\x00testpass
	c.expectCode(235)

	// Now MAIL should succeed.
	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	c.send("RCPT TO:<user@example.com>")
	c.expectCode(250)

	c.send("DATA")
	c.expectCode(354)
	c.sendData("Auth-then-mail test")
	c.expectCode(250)
}

func TestAUTH_PLAIN_ServerSide(t *testing.T) {
	clientConn, _ := startTestServer(t, WithAuthHandler(&testAuthHandler{}))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	lines := c.expectCode(250)

	// Check AUTH is advertised.
	found := false
	for _, l := range lines {
		if strings.HasPrefix(l, "AUTH ") {
			found = true
		}
	}
	if !found {
		t.Error("AUTH not advertised in EHLO")
	}

	// PLAIN auth with initial response.
	c.send("AUTH PLAIN AHRlc3R1c2VyAHRlc3RwYXNz") // \x00testuser\x00testpass
	c.expectCode(235)
}

func TestAUTH_PLAIN_BadCreds(t *testing.T) {
	clientConn, _ := startTestServer(t, WithAuthHandler(&testAuthHandler{}))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("AUTH PLAIN AHRlc3R1c2VyAHdyb25ncGFzcw==") // \x00testuser\x00wrongpass
	c.expectCode(535)
}

func TestAUTH_LOGIN_ServerSide(t *testing.T) {
	clientConn, _ := startTestServer(t, WithAuthHandler(&testAuthHandler{}))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("AUTH LOGIN")
	c.expectCode(334) // Username:

	c.send("dGVzdHVzZXI=") // testuser
	c.expectCode(334)       // Password:

	c.send("dGVzdHBhc3M=") // testpass
	c.expectCode(235)
}

func TestAUTH_CRAMMD5_ServerSide(t *testing.T) {
	clientConn, _ := startTestServer(t, WithAuthHandler(&testAuthHandler{}))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("AUTH CRAM-MD5")
	c.expectCode(334) // Challenge in base64.

	// Send a fake response: "testuser digest" base64-encoded.
	// Our test handler accepts any CRAM-MD5 for testuser.
	c.send("dGVzdHVzZXIgZmFrZWRpZ2VzdA==") // "testuser fakedigest"
	c.expectCode(235)
}

func TestAUTH_Cancel(t *testing.T) {
	clientConn, _ := startTestServer(t, WithAuthHandler(&testAuthHandler{}))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("AUTH LOGIN")
	c.expectCode(334) // Username:

	// Cancel with *.
	c.send("*")
	c.expectCode(501)
}

func TestAUTH_NotAvailable(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("AUTH PLAIN AHRlc3R1c2VyAHRlc3RwYXNz")
	c.expectCode(502) // Not implemented.
}

func TestAUTH_BeforeEHLO(t *testing.T) {
	clientConn, _ := startTestServer(t, WithAuthHandler(&testAuthHandler{}))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("AUTH PLAIN AHRlc3R1c2VyAHRlc3RwYXNz")
	c.expectCode(503) // Bad sequence.
}

func TestAUTH_AlreadyAuthenticated(t *testing.T) {
	clientConn, _ := startTestServer(t, WithAuthHandler(&testAuthHandler{}))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("AUTH PLAIN AHRlc3R1c2VyAHRlc3RwYXNz")
	c.expectCode(235)

	// Second AUTH → 503.
	c.send("AUTH PLAIN AHRlc3R1c2VyAHRlc3RwYXNz")
	c.expectCode(503)
}

func TestBDAT_ServerSide(t *testing.T) {
	handler := &testDataHandler{}
	clientConn, _ := startTestServer(t, WithDataHandler(handler))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	c.send("RCPT TO:<user@example.com>")
	c.expectCode(250)

	// BDAT with LAST.
	data := "Hello via BDAT"
	c.send(fmt.Sprintf("BDAT %d LAST", len(data)))
	c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	c.writer.WriteString(data)
	c.writer.Flush()
	c.expectCode(250)

	msg := handler.lastMessage()
	if msg.Body != "Hello via BDAT" {
		t.Errorf("Body = %q, want %q", msg.Body, "Hello via BDAT")
	}
}

func TestBDAT_MultipleChunks_ServerSide(t *testing.T) {
	handler := &testDataHandler{}
	clientConn, _ := startTestServer(t, WithDataHandler(handler))
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	c.send("RCPT TO:<user@example.com>")
	c.expectCode(250)

	// Chunk 1.
	chunk1 := "Part one "
	c.send(fmt.Sprintf("BDAT %d", len(chunk1)))
	c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	c.writer.WriteString(chunk1)
	c.writer.Flush()
	c.expectCode(250)

	// Chunk 2 (LAST).
	chunk2 := "part two"
	c.send(fmt.Sprintf("BDAT %d LAST", len(chunk2)))
	c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	c.writer.WriteString(chunk2)
	c.writer.Flush()
	c.expectCode(250)

	msg := handler.lastMessage()
	if msg.Body != "Part one part two" {
		t.Errorf("Body = %q, want %q", msg.Body, "Part one part two")
	}
}

func TestBDAT_BeforeRcpt(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("BDAT 5 LAST")
	c.expectCode(503) // Bad sequence.
}

func TestSTARTTLS_NotConfigured(t *testing.T) {
	// Server without TLS config.
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("STARTTLS")
	c.expectCode(502) // Not implemented.
}

func TestSTARTTLS_ServerSide(t *testing.T) {
	cert := generateTestCertServer(t)
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	handler := &testDataHandler{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(
		WithHostname("test.example.com"),
		WithTLSConfig(tlsConfig),
		WithDataHandler(handler),
		WithReadTimeout(5*time.Second),
		WithWriteTimeout(5*time.Second),
	)
	go srv.Serve(ln)
	defer srv.Close()

	nc, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	c := newConversation(t, nc)
	c.expectCode(220)

	c.send("EHLO test")
	lines := c.expectCode(250)

	// Check STARTTLS advertised.
	found := false
	for _, l := range lines {
		if l == "STARTTLS" {
			found = true
		}
	}
	if !found {
		t.Fatal("STARTTLS not advertised")
	}

	c.send("STARTTLS")
	c.expectCode(220)

	// Upgrade to TLS.
	tlsConn := tls.Client(nc, &tls.Config{InsecureSkipVerify: true})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("TLS handshake: %v", err)
	}

	// Replace the conversation's reader/writer with the TLS connection.
	c2 := newConversation(t, tlsConn)

	// Re-EHLO is required after STARTTLS.
	c2.send("EHLO test")
	lines = c2.expectCode(250)

	// STARTTLS should NOT appear (already on TLS).
	for _, l := range lines {
		if l == "STARTTLS" {
			t.Error("STARTTLS should not be advertised on TLS connection")
		}
	}

	// Send mail over TLS.
	c2.send("MAIL FROM:<sender@example.com>")
	c2.expectCode(250)
	c2.send("RCPT TO:<user@example.com>")
	c2.expectCode(250)
	c2.send("DATA")
	c2.expectCode(354)
	c2.sendData("TLS message")
	c2.expectCode(250)

	msg := handler.lastMessage()
	if !strings.Contains(msg.Body, "TLS message") {
		t.Errorf("Body = %q, want to contain TLS message", msg.Body)
	}
}

// generateTestCertServer creates a self-signed TLS certificate for testing.
func generateTestCertServer(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.example.com"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"test.example.com", "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  key,
	}
}

func TestDoubleMAIL(t *testing.T) {
	clientConn, _ := startTestServer(t)
	defer clientConn.Close()

	c := newConversation(t, clientConn)
	c.expectCode(220)

	c.send("EHLO test")
	c.expectCode(250)

	c.send("MAIL FROM:<sender@example.com>")
	c.expectCode(250)

	// Second MAIL without RSET → 503.
	c.send("MAIL FROM:<other@example.com>")
	c.expectCode(503)
}
