# Server API Reference

Package `smtpserver` implements an SMTP server (RFC 5321).

```
import "github.com/alexisbouchez/smtp.go/smtpserver"
```

## Creating a Server

```go
func NewServer(opts ...Option) *Server
```

Creates a new server with the given options. All options have sensible defaults.

## Server Options

### Network

| Option | Default | Description |
|--------|---------|-------------|
| `WithAddr(addr)` | `":25"` | Listen address |
| `WithHostname(name)` | `"localhost"` | Server hostname for greeting and EHLO response |
| `WithReadTimeout(d)` | `5m` | Read timeout per command |
| `WithWriteTimeout(d)` | `5m` | Write timeout per reply |

### Limits

| Option | Default | Description |
|--------|---------|-------------|
| `WithMaxMessageSize(n)` | `10 MB` | Maximum message size (advertised via SIZE) |
| `WithMaxRecipients(n)` | `100` | Maximum RCPT TO per transaction |
| `WithMaxConnections(n)` | `0` (unlimited) | Maximum concurrent connections |
| `WithMaxInvalidCommands(n)` | `10` | Invalid commands before disconnect |

### Security

| Option | Default | Description |
|--------|---------|-------------|
| `WithTLSConfig(c)` | `nil` | TLS config — enables STARTTLS when set |
| `WithSubmissionMode(bool)` | `false` | Require AUTH before MAIL FROM (RFC 6409) |

### Handlers

| Option | Description |
|--------|-------------|
| `WithConnectionHandler(h)` | Called on new TCP connections |
| `WithHeloHandler(h)` | Called on EHLO/HELO |
| `WithMailHandler(h)` | Called on MAIL FROM |
| `WithRcptHandler(h)` | Called on RCPT TO |
| `WithDataHandler(h)` | Called when message body is received |
| `WithResetHandler(h)` | Called on RSET or implicit reset |
| `WithVrfyHandler(h)` | Called on VRFY |
| `WithAuthHandler(h)` | Called on AUTH — enables AUTH extension |

### Logging

| Option | Default | Description |
|--------|---------|-------------|
| `WithLogger(l)` | `slog.Default()` | Structured logger (`log/slog`) |

## Server Lifecycle Methods

| Method | Description |
|--------|-------------|
| `ListenAndServe() error` | Listen on the configured address and serve (blocks) |
| `Serve(ln net.Listener) error` | Serve on an existing listener (blocks) |
| `Addr() net.Addr` | Returns the listener's address, or nil |
| `Shutdown(ctx) error` | Graceful shutdown: stop accepting, wait for sessions |
| `Close() error` | Immediate close: stop the listener |

## Handler Interfaces

All handlers are optional. Return `*smtp.SMTPError` for custom replies. Return a plain `error` for a generic `451` response.

### ConnectionHandler

```go
type ConnectionHandler interface {
    OnConnect(ctx context.Context, addr net.Addr) error
}
```

Called when a new TCP connection is accepted. Return an error to reject the connection.

### HeloHandler

```go
type HeloHandler interface {
    OnHelo(ctx context.Context, hostname string) error
}
```

Called when the client sends EHLO or HELO. The `hostname` is the client's self-reported identity.

### MailHandler

```go
type MailHandler interface {
    OnMail(ctx context.Context, from smtp.ReversePath) error
}
```

Called on MAIL FROM. The `from` contains the parsed sender address. A null reverse-path (`<>`) has `from.Null == true`.

### RcptHandler

```go
type RcptHandler interface {
    OnRcpt(ctx context.Context, to smtp.ForwardPath) error
}
```

Called for each RCPT TO. Return an error to reject the recipient.

### DataHandler

```go
type DataHandler interface {
    OnData(ctx context.Context, from smtp.ReversePath, to []smtp.ForwardPath, r io.Reader) error
}
```

Called when the message body is fully received (via DATA or BDAT LAST). The reader provides the de-stuffed body. Read the entire body before returning.

### AuthHandler

```go
type AuthHandler interface {
    Authenticate(ctx context.Context, mechanism string, username string, password string) error
}
```

Called for AUTH commands. When set, the server advertises `AUTH PLAIN LOGIN CRAM-MD5`. For CRAM-MD5, `password` contains `challenge:digest`.

### ResetHandler

```go
type ResetHandler interface {
    OnReset(ctx context.Context)
}
```

Called on RSET or implicit reset (EHLO re-issue, post-DATA). No return value — resets always succeed.

### VrfyHandler

```go
type VrfyHandler interface {
    OnVrfy(ctx context.Context, param string) (string, error)
}
```

Called on VRFY. Return a string result or an error. If not set, the server responds with `252 Cannot VRFY user, but will accept message`.

## Session State Machine

```
stateNew ──EHLO/HELO──> stateGreeted ──MAIL FROM──> stateMail ──RCPT TO──> stateRcpt ──DATA/BDAT LAST──> stateGreeted
                              ^                                                              |
                              └──────────────────────RSET────────────────────────────────────-┘
```

- **stateNew** — Connected, greeting sent. Only EHLO/HELO accepted.
- **stateGreeted** — EHLO/HELO received. MAIL FROM, AUTH, STARTTLS accepted.
- **stateMail** — MAIL FROM received. RCPT TO accepted.
- **stateRcpt** — At least one RCPT TO received. DATA, BDAT, more RCPT TO accepted.

STARTTLS resets to stateNew. Submission mode requires AUTH before MAIL FROM.

## Advertised Extensions

The server automatically advertises in EHLO:

| Extension | Condition |
|-----------|-----------|
| SIZE | Always (with configured max) |
| PIPELINING | Always |
| 8BITMIME | Always |
| ENHANCEDSTATUSCODES | Always |
| DSN | Always |
| SMTPUTF8 | Always |
| CHUNKING | Always |
| STARTTLS | When TLS config is set and connection is not yet TLS |
| AUTH | When AuthHandler is set and client is not yet authenticated |

## See also

- [Tutorial: Build your first server](../tutorials/building-server.md)
- [Client API reference](client.md)
- [Shared types reference](types.md)
- [SMTP transaction model](../explanation/smtp-transaction.md)
