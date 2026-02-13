package smtpclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/smtpserver"
)

// generateTestCert creates a self-signed TLS certificate for testing.
func generateTestCert(t *testing.T) tls.Certificate {
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

func TestSTARTTLS(t *testing.T) {
	cert := generateTestCert(t)
	serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}}

	handler := &testDataHandler{}
	addr, cleanup := startTestServer(t,
		smtpserver.WithTLSConfig(serverTLS),
		smtpserver.WithDataHandler(handler),
	)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// STARTTLS should be advertised.
	if !c.Extensions().Has(smtp.ExtSTARTTLS) {
		t.Fatal("STARTTLS not advertised")
	}

	// Should not be TLS yet.
	if c.IsTLS() {
		t.Fatal("should not be TLS before STARTTLS")
	}

	// Upgrade to TLS.
	clientTLS := &tls.Config{InsecureSkipVerify: true}
	if err := c.StartTLS(ctx, clientTLS); err != nil {
		t.Fatalf("StartTLS: %v", err)
	}

	if !c.IsTLS() {
		t.Fatal("should be TLS after STARTTLS")
	}

	// Extensions should still be available (re-issued EHLO).
	if !c.Extensions().Has(smtp.ExtPIPELINING) {
		t.Error("PIPELINING should still be advertised after STARTTLS")
	}

	// STARTTLS should NOT be advertised anymore (already on TLS).
	if c.Extensions().Has(smtp.ExtSTARTTLS) {
		t.Error("STARTTLS should not be advertised when already on TLS")
	}

	// Send mail over TLS.
	err = c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader("Encrypted message"))
	if err != nil {
		t.Fatalf("SendMail over TLS: %v", err)
	}

	msg := handler.lastMessage()
	if !strings.Contains(msg.Body, "Encrypted message") {
		t.Errorf("Body = %q, missing expected content", msg.Body)
	}
}

func TestSTARTTLS_NotAdvertised(t *testing.T) {
	// Server without TLS config.
	addr, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if c.Extensions().Has(smtp.ExtSTARTTLS) {
		t.Fatal("STARTTLS should not be advertised without TLS config")
	}

	// Attempting STARTTLS should fail.
	clientTLS := &tls.Config{InsecureSkipVerify: true}
	err = c.StartTLS(ctx, clientTLS)
	if err == nil {
		t.Fatal("expected STARTTLS to fail when not advertised")
	}
}

func TestSTARTTLS_AlreadyTLS(t *testing.T) {
	cert := generateTestCert(t)
	serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}}

	addr, cleanup := startTestServer(t, smtpserver.WithTLSConfig(serverTLS))
	defer cleanup()

	ctx := context.Background()
	c, err := Dial(ctx, addr, WithLocalName("test.local"))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	clientTLS := &tls.Config{InsecureSkipVerify: true}
	if err := c.StartTLS(ctx, clientTLS); err != nil {
		t.Fatalf("StartTLS: %v", err)
	}

	// Second STARTTLS should fail (503 from server).
	err = c.StartTLS(ctx, clientTLS)
	if err == nil {
		t.Fatal("expected second STARTTLS to fail")
	}
}
