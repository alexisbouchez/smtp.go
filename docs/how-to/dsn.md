# How to: Request Delivery Status Notifications

Use DSN parameters (RFC 3461) to request delivery receipts.

## MAIL FROM options

Set the envelope ID and return type on `Mail`:

```go
err := c.Mail(ctx, "sender@example.com",
    smtpclient.WithDSNReturn("HDRS"),
    smtpclient.WithDSNEnvelopeID("unique-id-123"),
)
```

| Option | Values | Description |
|--------|--------|-------------|
| `WithDSNReturn(ret)` | `"FULL"`, `"HDRS"` | Return full message or headers only in DSNs |
| `WithDSNEnvelopeID(id)` | any string | Envelope identifier echoed back in DSN |

## RCPT TO options

Set notification conditions and original recipient per recipient:

```go
err := c.Rcpt(ctx, "recipient@example.com",
    smtpclient.WithDSNNotify("SUCCESS,FAILURE"),
    smtpclient.WithDSNOriginalRecipient("rfc822;recipient@example.com"),
)
```

| Option | Values | Description |
|--------|--------|-------------|
| `WithDSNNotify(notify)` | `"SUCCESS"`, `"FAILURE"`, `"DELAY"`, `"NEVER"` | When to send DSNs (comma-separated) |
| `WithDSNOriginalRecipient(orcpt)` | `"rfc822;addr"` | Original recipient for forwarded mail |

## Complete example

```go
c, err := smtpclient.Dial(ctx, "mail.example.com:25")
if err != nil {
    return err
}
defer c.Close()

// Request headers-only DSN with an envelope ID.
err = c.Mail(ctx, "sender@example.com",
    smtpclient.WithDSNReturn("HDRS"),
    smtpclient.WithDSNEnvelopeID("msg-42"),
)
if err != nil {
    return err
}

// Notify on success and failure.
err = c.Rcpt(ctx, "recipient@example.com",
    smtpclient.WithDSNNotify("SUCCESS,FAILURE"),
)
if err != nil {
    return err
}

err = c.Data(ctx, strings.NewReader(body))
```

## Server side

The server advertises `DSN` automatically in its EHLO response. DSN parameters are parsed from MAIL/RCPT commands and passed through transparently.

## See also

- [Client API reference](../reference/client.md) — all MailOption and RcptOption functions
- [SMTP transaction model](../explanation/smtp-transaction.md) — how Mail/Rcpt/Data fit together
