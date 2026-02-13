# How to: Transfer Messages with BDAT

Use CHUNKING/BDAT (RFC 3030) for binary message transfer without dot-stuffing.

## Client: Send with BDAT

BDAT transfers the message body in binary chunks. Unlike DATA, no dot-stuffing is applied, making it suitable for binary content:

```go
c, err := smtpclient.Dial(ctx, "mail.example.com:25")
if err != nil {
    return err
}
defer c.Close()

err = c.Mail(ctx, "sender@example.com")
if err != nil {
    return err
}
err = c.Rcpt(ctx, "recipient@example.com")
if err != nil {
    return err
}

// Send the message in one chunk.
body := []byte("Subject: Test\r\n\r\nMessage body")
err = c.Bdat(ctx, body, true)  // true = LAST chunk
```

### Multiple chunks

For large messages, send data in multiple chunks:

```go
// First chunk (not the last).
err = c.Bdat(ctx, chunk1, false)
if err != nil {
    return err
}

// Final chunk.
err = c.Bdat(ctx, chunk2, true)
```

The server accumulates all chunks and delivers them as a single message body when it receives the chunk marked `LAST`.

### Check server support

```go
if c.Extensions().Has(smtp.ExtCHUNKING) {
    // Server supports BDAT.
}
```

## Server side

BDAT is handled automatically. The server parses `BDAT <size> [LAST]` commands, accumulates the binary chunks, and delivers the complete message body to your `DataHandler` — the same handler used for DATA.

No additional server configuration is needed. The server advertises `CHUNKING` in EHLO by default.

## See also

- [Client API reference](../reference/client.md) — Bdat method signature
- [Architecture](../explanation/architecture.md) — wire protocol layer and dot-stuffing
