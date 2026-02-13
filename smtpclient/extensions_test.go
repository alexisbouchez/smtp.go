package smtpclient

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/smtpserver"
)

func TestSIZE_Advertised(t *testing.T) {
	addr, cleanup := startTestServer(t, smtpserver.WithMaxMessageSize(50*1024*1024))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if !c.Extensions().Has(smtp.ExtSIZE) {
		t.Fatal("SIZE not advertised")
	}
	if got := c.ServerMaxSize(); got != 50*1024*1024 {
		t.Errorf("ServerMaxSize = %d, want %d", got, 50*1024*1024)
	}
}

func TestSIZE_Parameter(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t,
		smtpserver.WithMaxMessageSize(50*1024*1024),
		smtpserver.WithDataHandler(handler),
	)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// Send with SIZE parameter.
	err = c.Mail(ctx, "sender@example.com", WithSize(1024))
	if err != nil {
		t.Fatalf("Mail with SIZE: %v", err)
	}
	err = c.Rcpt(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Rcpt: %v", err)
	}
	err = c.Data(ctx, strings.NewReader("Body"))
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
}

func TestBODY_8BITMIME(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if !c.Extensions().Has(smtp.Ext8BITMIME) {
		t.Fatal("8BITMIME not advertised")
	}

	err = c.Mail(ctx, "sender@example.com", WithBody("8BITMIME"))
	if err != nil {
		t.Fatalf("Mail with BODY: %v", err)
	}
	err = c.Rcpt(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Rcpt: %v", err)
	}
	err = c.Data(ctx, strings.NewReader("UTF-8 body: héllo wörld"))
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
}

func TestDSN_Parameters(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if !c.Extensions().Has(smtp.ExtDSN) {
		t.Fatal("DSN not advertised")
	}

	err = c.Mail(ctx, "sender@example.com",
		WithDSNReturn("HDRS"),
		WithDSNEnvelopeID("unique-id-123"),
	)
	if err != nil {
		t.Fatalf("Mail with DSN: %v", err)
	}

	err = c.Rcpt(ctx, "user@example.com",
		WithDSNNotify("SUCCESS,FAILURE"),
		WithDSNOriginalRecipient("rfc822;user@example.com"),
	)
	if err != nil {
		t.Fatalf("Rcpt with DSN: %v", err)
	}

	err = c.Data(ctx, strings.NewReader("DSN message"))
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
}

func TestSMTPUTF8(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if !c.Extensions().Has(smtp.ExtSMTPUTF8) {
		t.Fatal("SMTPUTF8 not advertised")
	}

	err = c.Mail(ctx, "sender@example.com", WithSMTPUTF8())
	if err != nil {
		t.Fatalf("Mail with SMTPUTF8: %v", err)
	}
	err = c.Rcpt(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Rcpt: %v", err)
	}
	err = c.Data(ctx, strings.NewReader("UTF-8 message"))
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
}

func TestBDAT(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if !c.Extensions().Has(smtp.ExtCHUNKING) {
		t.Fatal("CHUNKING not advertised")
	}

	err = c.Mail(ctx, "sender@example.com")
	if err != nil {
		t.Fatalf("Mail: %v", err)
	}
	err = c.Rcpt(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Rcpt: %v", err)
	}

	// Send in a single BDAT LAST.
	err = c.Bdat(ctx, []byte("BDAT message body"), true)
	if err != nil {
		t.Fatalf("Bdat: %v", err)
	}

	msg := handler.lastMessage()
	if !strings.Contains(msg.Body, "BDAT message body") {
		t.Errorf("Body = %q, missing expected content", msg.Body)
	}
}

func TestBDAT_MultipleChunks(t *testing.T) {
	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t, smtpserver.WithDataHandler(handler))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	err = c.Mail(ctx, "sender@example.com")
	if err != nil {
		t.Fatalf("Mail: %v", err)
	}
	err = c.Rcpt(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Rcpt: %v", err)
	}

	// Chunk 1.
	err = c.Bdat(ctx, []byte("Chunk one "), false)
	if err != nil {
		t.Fatalf("Bdat chunk 1: %v", err)
	}

	// Chunk 2 (LAST).
	err = c.Bdat(ctx, []byte("chunk two"), true)
	if err != nil {
		t.Fatalf("Bdat chunk 2: %v", err)
	}

	msg := handler.lastMessage()
	if msg.Body != "Chunk one chunk two" {
		t.Errorf("Body = %q, want %q", msg.Body, "Chunk one chunk two")
	}
}

func TestEHLO_AdvertisesAllExtensions(t *testing.T) {
	addr, cleanup := startTestServer(t,
		smtpserver.WithMaxMessageSize(10*1024*1024),
		smtpserver.WithAuthHandler(&testAuthHandler{}),
	)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	for _, ext := range []smtp.Extension{
		smtp.ExtPIPELINING,
		smtp.Ext8BITMIME,
		smtp.ExtENHANCEDSTATUSCODES,
		smtp.ExtDSN,
		smtp.ExtSMTPUTF8,
		smtp.ExtCHUNKING,
		smtp.ExtSIZE,
		smtp.ExtAUTH,
	} {
		if !c.Extensions().Has(ext) {
			t.Errorf("extension %s not advertised", ext)
		}
	}
}
