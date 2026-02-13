package smtpserver

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/internal/textproto"
)

// Server is an SMTP server that listens for incoming connections and
// dispatches them to handler interfaces.
type Server struct {
	addr           string
	hostname       string
	readTimeout    time.Duration
	writeTimeout   time.Duration
	maxMessageSize int64
	maxRecipients  int
	tlsConfig      *tls.Config
	logger         *slog.Logger

	connHandler  ConnectionHandler
	heloHandler  HeloHandler
	mailHandler  MailHandler
	rcptHandler  RcptHandler
	dataHandler  DataHandler
	resetHandler ResetHandler
	vrfyHandler  VrfyHandler
	authHandler    AuthHandler
	submissionMode bool

	maxConnections   int
	maxInvalidCmds   int

	listener net.Listener
	wg       sync.WaitGroup
	quit     chan struct{}
	mu       sync.Mutex
	connSem  chan struct{} // Semaphore for limiting concurrent connections.
}

// Option configures a Server.
type Option func(*Server)

// NewServer creates a new SMTP server with the given options.
func NewServer(opts ...Option) *Server {
	s := &Server{
		addr:           ":25",
		hostname:       "localhost",
		readTimeout:    5 * time.Minute,
		writeTimeout:   5 * time.Minute,
		maxMessageSize: 10 * 1024 * 1024, // 10 MB
		maxRecipients:  100,
		maxInvalidCmds: 10,
		logger:         slog.Default(),
		quit:           make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithAddr sets the listen address (e.g., ":25", ":587").
func WithAddr(addr string) Option {
	return func(s *Server) { s.addr = addr }
}

// WithHostname sets the server hostname used in the greeting banner and EHLO response.
func WithHostname(hostname string) Option {
	return func(s *Server) { s.hostname = hostname }
}

// WithReadTimeout sets the timeout for reading client commands.
func WithReadTimeout(d time.Duration) Option {
	return func(s *Server) { s.readTimeout = d }
}

// WithWriteTimeout sets the timeout for writing server replies.
func WithWriteTimeout(d time.Duration) Option {
	return func(s *Server) { s.writeTimeout = d }
}

// WithMaxMessageSize sets the maximum message size in bytes.
func WithMaxMessageSize(n int64) Option {
	return func(s *Server) { s.maxMessageSize = n }
}

// WithMaxRecipients sets the maximum number of recipients per transaction.
func WithMaxRecipients(n int) Option {
	return func(s *Server) { s.maxRecipients = n }
}

// WithTLSConfig sets the TLS configuration for STARTTLS support.
func WithTLSConfig(c *tls.Config) Option {
	return func(s *Server) { s.tlsConfig = c }
}

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) { s.logger = l }
}

// WithConnectionHandler sets the handler called on new connections.
func WithConnectionHandler(h ConnectionHandler) Option {
	return func(s *Server) { s.connHandler = h }
}

// WithHeloHandler sets the handler called on EHLO/HELO.
func WithHeloHandler(h HeloHandler) Option {
	return func(s *Server) { s.heloHandler = h }
}

// WithMailHandler sets the handler called on MAIL FROM.
func WithMailHandler(h MailHandler) Option {
	return func(s *Server) { s.mailHandler = h }
}

// WithRcptHandler sets the handler called on RCPT TO.
func WithRcptHandler(h RcptHandler) Option {
	return func(s *Server) { s.rcptHandler = h }
}

// WithDataHandler sets the handler called when a message body is received.
func WithDataHandler(h DataHandler) Option {
	return func(s *Server) { s.dataHandler = h }
}

// WithResetHandler sets the handler called on RSET.
func WithResetHandler(h ResetHandler) Option {
	return func(s *Server) { s.resetHandler = h }
}

// WithVrfyHandler sets the handler called on VRFY.
func WithVrfyHandler(h VrfyHandler) Option {
	return func(s *Server) { s.vrfyHandler = h }
}

// WithAuthHandler sets the handler called for SMTP AUTH.
// When set, the server advertises AUTH with PLAIN, LOGIN, and CRAM-MD5 mechanisms.
func WithAuthHandler(h AuthHandler) Option {
	return func(s *Server) { s.authHandler = h }
}

// WithSubmissionMode enables message submission semantics (RFC 6409).
// In submission mode, clients must authenticate before sending MAIL FROM.
// Unauthenticated MAIL FROM commands receive a 530 reply.
func WithSubmissionMode(enabled bool) Option {
	return func(s *Server) { s.submissionMode = enabled }
}

// WithMaxConnections sets the maximum number of concurrent connections.
// Zero means unlimited.
func WithMaxConnections(n int) Option {
	return func(s *Server) { s.maxConnections = n }
}

// WithMaxInvalidCommands sets the maximum number of invalid commands per
// session before the server disconnects the client. Default is 10.
func WithMaxInvalidCommands(n int) Option {
	return func(s *Server) { s.maxInvalidCmds = n }
}

// ListenAndServe starts listening on the configured address and serves
// SMTP connections. It blocks until the server is shut down.
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve accepts connections on the given listener and serves them.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	s.listener = ln
	if s.maxConnections > 0 {
		s.connSem = make(chan struct{}, s.maxConnections)
	}
	s.mu.Unlock()

	s.logger.Info("smtp server listening", "addr", ln.Addr())

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return nil
			default:
				s.logger.Error("accept error", "err", err)
				continue
			}
		}

		// Connection limiting.
		if s.connSem != nil {
			select {
			case s.connSem <- struct{}{}:
				// Acquired a slot.
			default:
				// At capacity — reject with 421.
				tc := textproto.NewConn(conn)
				tc.WriteReply(int(smtp.ReplyServiceNotAvailable), "4.7.0 Too many connections, try again later")
				conn.Close()
				continue
			}
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if s.connSem != nil {
				defer func() { <-s.connSem }()
			}
			s.handleConn(conn)
		}()
	}
}

// Addr returns the listener's address, or nil if not listening.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}

// Shutdown gracefully shuts down the server. It stops accepting new
// connections and waits for existing sessions to finish, respecting
// the context deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.quit)

	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()

	if ln != nil {
		ln.Close()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close immediately closes the listener and all connections.
func (s *Server) Close() error {
	close(s.quit)
	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()
	if ln != nil {
		return ln.Close()
	}
	return nil
}
