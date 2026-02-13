# Architecture

This document explains the design decisions behind smtp.go's package layout, wire protocol layer, and API patterns.

## Package layout

The library is split into four packages:

```
smtp (root)          Shared types used by both client and server
├── smtpclient       SMTP client implementation
├── smtpserver       SMTP server implementation
└── internal/textproto   Wire protocol layer (not exported)
```

**Why separate packages?** Most applications need either a client or a server, not both. Separate packages keep imports minimal and make the API discoverable — `smtpclient.Dial` and `smtpserver.NewServer` are clear entry points.

**Why a shared root package?** Types like `ReplyCode`, `EnhancedCode`, `SMTPError`, address types, and SASL mechanisms are used by both sides. Placing them in the root package avoids circular imports and makes them natural to reference: `smtp.ReplyOK`, `smtp.PlainAuth(...)`.

**Why `internal/textproto`?** The wire protocol layer handles line reading, reply parsing, dot-stuffing, and buffered I/O. It's an implementation detail that both client and server depend on. Keeping it `internal` prevents external consumers from coupling to it, allowing the wire format handling to evolve independently.

## Wire protocol layer

All reads and writes go through `internal/textproto.Conn`, which wraps a `net.Conn` with:

- **Buffered I/O** — A `bufio.Reader` and `bufio.Writer` for efficient reading and writing.
- **Line protocol** — `ReadLine` and `WriteLine` handle `\r\n` termination.
- **Reply parsing** — `ReadReply` and `WriteReply` handle multi-line replies (`250-` continuation lines).
- **Dot-stuffing** — `DotReader` and `DotWriter` implement RFC 5321 &sect;4.5.2 transparently. The dot reader's state machine also tolerates bare LF (`\n` without `\r`) for robustness.
- **Deadline management** — `SetDeadlineFromContext` and `SetReadDeadline` propagate timeouts to the underlying connection.
- **Connection replacement** — `ReplaceConn` swaps the underlying `net.Conn` for STARTTLS upgrades, rebinding the buffered reader/writer to the new TLS connection.

This layer means the client and server code never deal with raw byte manipulation or line termination — they work with clean string commands and structured replies.

## Zero dependencies

The library uses only the Go standard library. This is a deliberate choice:

- **SMTP is a well-defined, stable protocol.** The stdlib provides everything needed: TCP networking, TLS, buffered I/O, crypto for SASL.
- **Fewer dependencies = fewer supply chain risks.** For a security-sensitive protocol library, minimizing the dependency tree is valuable.
- **Easier to vendor and audit.** Users can inspect the entire library without chasing transitive dependencies.

## Functional options pattern

Both the client and server use the functional options pattern for configuration:

```go
// Client
c, err := smtpclient.Dial(ctx, addr,
    smtpclient.WithLocalName("client.example.com"),
    smtpclient.WithTimeout(10 * time.Second),
)

// Server
srv := smtpserver.NewServer(
    smtpserver.WithAddr(":587"),
    smtpserver.WithHostname("mail.example.com"),
    smtpserver.WithMaxConnections(100),
)
```

This pattern offers several advantages over a config struct:

- **Sensible defaults** — Every option has a default. `smtpclient.Dial(ctx, addr)` works out of the box.
- **Forward compatibility** — New options can be added without breaking existing code.
- **Composability** — Options can be stored in slices, passed around, and combined.

MAIL and RCPT extension parameters use the same pattern with `MailOption` and `RcptOption` types, keeping the core method signatures clean while allowing protocol extensions.

## Handler interfaces

The server dispatches SMTP commands to handler interfaces rather than using a single monolithic handler:

```go
type DataHandler interface {
    OnData(ctx context.Context, from smtp.ReversePath, to []smtp.ForwardPath, r io.Reader) error
}
```

**Why separate interfaces?** Most servers only need a `DataHandler`. By keeping each command's handler as a separate interface, a simple server only needs to implement one interface. A complex server can implement as many as needed, and a single struct can satisfy multiple interfaces.

**Why all optional?** The server handles the SMTP protocol automatically. Handlers are hooks for application logic, not protocol mechanics. A server with no handlers accepts all mail and discards it — useful for testing or as a starting point.

**Why `*smtp.SMTPError` for custom replies?** Returning a plain `error` signals an internal failure (451). Returning `*smtp.SMTPError` gives the handler full control over the reply code, enhanced code, and message. This two-tier error model separates "something went wrong" from "I'm intentionally rejecting this".

## See also

- [SMTP transaction model](smtp-transaction.md) — the state machine and command ordering
- [Security considerations](security.md) — TLS, authentication, and trust model
- [Server API reference](../reference/server.md) — handler interfaces and options
