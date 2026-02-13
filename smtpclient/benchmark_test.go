package smtpclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/smtpserver"
)

func TestConcurrent100Clients(t *testing.T) {
	handler := &testDataHandler{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := smtpserver.NewServer(
		smtpserver.WithHostname("test.example.com"),
		smtpserver.WithDataHandler(handler),
		smtpserver.WithReadTimeout(10*time.Second),
		smtpserver.WithWriteTimeout(10*time.Second),
	)
	go srv.Serve(ln)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	const numClients = 100
	var wg sync.WaitGroup
	wg.Add(numClients)

	errCh := make(chan error, numClients)

	for i := range numClients {
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()
			c, err := Dial(ctx, ln.Addr().String(),
				WithLocalName("test.local"),
				WithTimeout(10*time.Second),
			)
			if err != nil {
				errCh <- fmt.Errorf("client %d dial: %w", id, err)
				return
			}
			defer c.Close()

			body := fmt.Sprintf("Subject: Test %d\r\n\r\nMessage from client %d", id, id)
			err = c.SendMail(ctx,
				fmt.Sprintf("sender%d@example.com", id),
				[]string{"user@example.com"},
				strings.NewReader(body),
			)
			if err != nil {
				errCh <- fmt.Errorf("client %d send: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.messages) != numClients {
		t.Errorf("expected %d messages, got %d", numClients, len(handler.messages))
	}
}

func TestGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()

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

	// Run 10 clients, each sending a message.
	for i := range 10 {
		ctx := context.Background()
		c, err := Dial(ctx, ln.Addr().String(),
			WithLocalName("test.local"),
			WithTimeout(5*time.Second),
		)
		if err != nil {
			t.Fatalf("client %d dial: %v", i, err)
		}
		body := fmt.Sprintf("Goroutine leak test message %d", i)
		if err := c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader(body)); err != nil {
			t.Fatalf("client %d send: %v", i, err)
		}
		c.Close()
	}

	// Shut down.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// Allow goroutines to settle.
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	// Allow a small delta for runtime goroutines.
	delta := after - before
	if delta > 5 {
		t.Errorf("goroutine leak: before=%d after=%d delta=%d", before, after, delta)
	}
}

func BenchmarkSendMail(b *testing.B) {
	handler := &testDataHandler{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}

	srv := smtpserver.NewServer(
		smtpserver.WithHostname("bench.example.com"),
		smtpserver.WithDataHandler(handler),
		smtpserver.WithReadTimeout(30*time.Second),
		smtpserver.WithWriteTimeout(30*time.Second),
	)
	go srv.Serve(ln)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	ctx := context.Background()
	c, err := Dial(ctx, ln.Addr().String(), WithLocalName("bench.local"), WithTimeout(10*time.Second))
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	body := "Subject: Benchmark\r\n\r\nBenchmark message body."

	b.ResetTimer()
	for b.Loop() {
		err := c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader(body))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSendMailConcurrent(b *testing.B) {
	handler := &testDataHandler{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}

	srv := smtpserver.NewServer(
		smtpserver.WithHostname("bench.example.com"),
		smtpserver.WithDataHandler(handler),
		smtpserver.WithReadTimeout(30*time.Second),
		smtpserver.WithWriteTimeout(30*time.Second),
	)
	go srv.Serve(ln)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	body := "Subject: Benchmark\r\n\r\nConcurrent benchmark message body."

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		c, err := Dial(ctx, ln.Addr().String(), WithLocalName("bench.local"), WithTimeout(10*time.Second))
		if err != nil {
			b.Fatal(err)
		}
		defer c.Close()

		for pb.Next() {
			err := c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader(body))
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkLargeMessage(b *testing.B) {
	handler := &discardDataHandler{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}

	srv := smtpserver.NewServer(
		smtpserver.WithHostname("bench.example.com"),
		smtpserver.WithDataHandler(handler),
		smtpserver.WithMaxMessageSize(20*1024*1024),
		smtpserver.WithReadTimeout(60*time.Second),
		smtpserver.WithWriteTimeout(60*time.Second),
	)
	go srv.Serve(ln)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	ctx := context.Background()
	c, err := Dial(ctx, ln.Addr().String(), WithLocalName("bench.local"), WithTimeout(30*time.Second))
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	// 10MB message.
	largeBody := strings.Repeat("X", 10*1024*1024)

	b.ResetTimer()
	b.SetBytes(int64(len(largeBody)))
	for b.Loop() {
		err := c.SendMail(ctx, "sender@example.com", []string{"user@example.com"}, strings.NewReader(largeBody))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// discardDataHandler reads and discards message data for benchmarks.
type discardDataHandler struct{}

func (h *discardDataHandler) OnData(_ context.Context, _ smtp.ReversePath, _ []smtp.ForwardPath, r io.Reader) error {
	io.Copy(io.Discard, r)
	return nil
}
