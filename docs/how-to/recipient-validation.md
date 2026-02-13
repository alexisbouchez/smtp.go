# How to: Validate Senders and Recipients

Reject mail from unwanted senders or to unknown recipients on the server.

## Validate senders with MailHandler

Implement `MailHandler` to inspect the `MAIL FROM` address:

```go
type handler struct{}

func (h *handler) OnMail(_ context.Context, from smtp.ReversePath) error {
    // Allow null reverse-path (bounces).
    if from.Null {
        return nil
    }

    // Block a specific domain.
    if from.Mailbox.Domain == "spam.example.com" {
        return smtp.Errorf(smtp.ReplyMailboxNotFound,
            smtp.EnhancedCodeBadSenderSyntax,
            "Sender not accepted")
    }

    return nil
}
```

## Validate recipients with RcptHandler

Implement `RcptHandler` to check each `RCPT TO` address:

```go
var validDomains = map[string]bool{
    "example.com":     true,
    "mail.example.com": true,
}

func (h *handler) OnRcpt(_ context.Context, to smtp.ForwardPath) error {
    if !validDomains[to.Mailbox.Domain] {
        return smtp.Errorf(smtp.ReplyMailboxNotFound,
            smtp.EnhancedCodeBadDest,
            "No such user here")
    }
    return nil
}
```

## Register the handlers

```go
h := &handler{}

srv := smtpserver.NewServer(
    smtpserver.WithAddr(":25"),
    smtpserver.WithHostname("mail.example.com"),
    smtpserver.WithMailHandler(h),
    smtpserver.WithRcptHandler(h),
    smtpserver.WithDataHandler(h),
)
```

A single struct can implement multiple handler interfaces.

## Error responses

Return an `*smtp.SMTPError` to control the reply code and enhanced status code sent to the client. If you return a plain `error`, the server sends a generic `451 4.4.0 Internal error`.

Common reply codes for validation:

| Code | Enhanced | Meaning |
|------|----------|---------|
| 550 | 5.1.1 | Mailbox not found |
| 550 | 5.1.7 | Bad sender address |
| 553 | 5.1.3 | Bad destination syntax |
| 551 | 5.1.6 | Mailbox moved |

## See also

- [Error handling](error-handling.md) — constructing and inspecting SMTP errors
- [Server API reference](../reference/server.md) — all handler interfaces
- [Shared types reference](../reference/types.md) — ReversePath, ForwardPath, EnhancedCode constants
