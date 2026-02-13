# smtp.go

A comprehensive, zero-dependency SMTP client and server library for Go.

```
go get github.com/alexisbouchez/smtp.go
```

## Features

- Full RFC 5321 implementation — client and server
- Zero external dependencies — stdlib only
- STARTTLS (RFC 3207) — encrypted connections
- SASL Authentication (RFC 4954) — PLAIN, LOGIN, CRAM-MD5
- Message Submission (RFC 6409) — port 587 with required auth
- DSN (RFC 3461) — delivery status notifications
- CHUNKING/BDAT (RFC 3030) — binary message transfer
- 8BITMIME, SIZE, PIPELINING, SMTPUTF8, Enhanced Status Codes
- Graceful shutdown with context deadlines
- Connection limiting and abuse protection

## Quick Start

**Send a message:**

```go
c, err := smtpclient.Dial(ctx, "mail.example.com:25")
if err != nil { panic(err) }
defer c.Close()

err = c.SendMail(ctx, "sender@example.com",
    []string{"recipient@example.com"},
    strings.NewReader("Subject: Hello\r\n\r\nHello from smtp.go!"))
```

**Run a server:**

```go
srv := smtpserver.NewServer(
    smtpserver.WithAddr(":2525"),
    smtpserver.WithHostname("mail.example.com"),
    smtpserver.WithDataHandler(&handler{}),
)
log.Fatal(srv.ListenAndServe())
```

## Documentation

|  | Learning | Doing |
|---|---------|-------|
| **Practical** | [Tutorial: Send your first email](docs/tutorials/sending-email.md) | [How to: Upgrade to TLS](docs/how-to/starttls.md) |
|  | [Tutorial: Build your first server](docs/tutorials/building-server.md) | [How to: Authenticate with SASL](docs/how-to/authentication.md) |
|  |  | [How to: Submit messages (port 587)](docs/how-to/message-submission.md) |
|  |  | [How to: Request DSN](docs/how-to/dsn.md) |
|  |  | [How to: Transfer with BDAT](docs/how-to/chunking.md) |
|  |  | [How to: Validate recipients](docs/how-to/recipient-validation.md) |
|  |  | [How to: Limit connections](docs/how-to/connection-limiting.md) |
|  |  | [How to: Graceful shutdown](docs/how-to/graceful-shutdown.md) |
|  |  | [How to: Handle errors](docs/how-to/error-handling.md) |
| **Theoretical** | [Explanation: Architecture](docs/explanation/architecture.md) | [Reference: Client API](docs/reference/client.md) |
|  | [Explanation: SMTP transactions](docs/explanation/smtp-transaction.md) | [Reference: Server API](docs/reference/server.md) |
|  | [Explanation: Security](docs/explanation/security.md) | [Reference: Shared types](docs/reference/types.md) |

## RFCs

This library implements or references: [RFC 5321](rfcs/) (SMTP), [RFC 5322](rfcs/) (Message Format), [RFC 3207](rfcs/) (STARTTLS), [RFC 4954](rfcs/) (AUTH), [RFC 6409](rfcs/) (Submission), [RFC 1870](rfcs/) (SIZE), [RFC 2920](rfcs/) (PIPELINING), [RFC 6152](rfcs/) (8BITMIME), [RFC 3461](rfcs/) (DSN), [RFC 2034](rfcs/) (Enhanced Codes), [RFC 3463](rfcs/) (Status Codes), [RFC 6531](rfcs/) (SMTPUTF8), [RFC 3030](rfcs/) (CHUNKING), [RFC 4616](rfcs/) (SASL PLAIN), [RFC 2195](rfcs/) (CRAM-MD5).

## License

MIT
