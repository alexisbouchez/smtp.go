# Client API Reference

Package `smtpclient` implements an SMTP client (RFC 5321).

```
import "github.com/alexisbouchez/smtp.go/smtpclient"
```

## Connecting

### Dial

```go
func Dial(ctx context.Context, addr string, opts ...Option) (*Client, error)
```

Connects to the SMTP server at `addr`, reads the greeting, and sends EHLO. Falls back to HELO if EHLO is rejected. The `ctx` timeout applies to the entire dial + greeting + EHLO sequence.

### NewClient

```go
func NewClient(nc net.Conn, localName string) (*Client, error)
```

Wraps an existing `net.Conn` as an SMTP client. The greeting must not have been read yet.

## Dial Options

| Function | Default | Description |
|----------|---------|-------------|
| `WithLocalName(name)` | `"localhost"` | Hostname sent in EHLO |
| `WithTimeout(d)` | `30s` | Timeout for dial + greeting + EHLO |
| `WithDialer(d)` | `&net.Dialer{}` | Custom dialer for the TCP connection |
| `WithTLSConfig(c)` | `nil` | TLS config (used by `StartTLS`) |
| `WithLogger(l)` | `slog.Default()` | Structured logger |

## Client Methods

### Mail Transaction

| Method | Description |
|--------|-------------|
| `Mail(ctx, from, ...MailOption) error` | Send MAIL FROM with optional SIZE, BODY, SMTPUTF8, DSN parameters |
| `Rcpt(ctx, to, ...RcptOption) error` | Send RCPT TO with optional DSN parameters |
| `Data(ctx, r io.Reader) error` | Send DATA and stream message body (dot-stuffed) |
| `Bdat(ctx, data []byte, last bool) error` | Send a BDAT chunk (RFC 3030). Set `last=true` for the final chunk |
| `SendMail(ctx, from, to []string, r) error` | Convenience: MAIL + RCPT(s) + DATA in one call |

### TLS and Authentication

| Method | Description |
|--------|-------------|
| `StartTLS(ctx, *tls.Config) error` | Upgrade to TLS and re-issue EHLO (RFC 3207) |
| `Auth(ctx, SASLMechanism) error` | Authenticate with SASL (RFC 4954) |
| `SubmitMessage(ctx, mech, tlsCfg, from, to, r) error` | STARTTLS + AUTH + SendMail for port 587 submission |

### Session Management

| Method | Description |
|--------|-------------|
| `Reset(ctx) error` | Send RSET to abort the current transaction |
| `Noop(ctx) error` | Send NOOP as a keepalive |
| `Close() error` | Send QUIT and close the connection |

### Queries

| Method | Description |
|--------|-------------|
| `Extensions() smtp.Extensions` | Extensions from the last EHLO response (nil for HELO) |
| `ServerMaxSize() int64` | Max message size from SIZE extension (0 if not advertised) |
| `IsTLS() bool` | Whether the connection is using TLS |

## MailOption Functions

Options for the `Mail` method.

| Function | SMTP Parameter | Description |
|----------|---------------|-------------|
| `WithSize(n int64)` | `SIZE=n` | Declare message size (RFC 1870) |
| `WithBody(body string)` | `BODY=body` | `"8BITMIME"` or `"7BIT"` (RFC 6152) |
| `WithSMTPUTF8()` | `SMTPUTF8` | Internationalized addresses (RFC 6531) |
| `WithDSNReturn(ret)` | `RET=ret` | `"FULL"` or `"HDRS"` (RFC 3461) |
| `WithDSNEnvelopeID(id)` | `ENVID=id` | Envelope identifier for DSN (RFC 3461) |

## RcptOption Functions

Options for the `Rcpt` method.

| Function | SMTP Parameter | Description |
|----------|---------------|-------------|
| `WithDSNNotify(notify)` | `NOTIFY=notify` | `"SUCCESS"`, `"FAILURE"`, `"DELAY"`, or `"NEVER"` (RFC 3461) |
| `WithDSNOriginalRecipient(orcpt)` | `ORCPT=orcpt` | `"rfc822;addr"` — original recipient (RFC 3461) |

## See also

- [Tutorial: Send your first email](../tutorials/sending-email.md)
- [Server API reference](server.md)
- [Shared types reference](types.md) — SASLMechanism, Extensions, SMTPError
