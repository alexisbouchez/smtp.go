# How to: Handle and Return SMTP Errors

Work with SMTP protocol errors on both client and server.

## Client: Check for SMTP errors

All client methods return `*smtp.SMTPError` for protocol-level failures. Use `errors.As` to inspect the error:

```go
err := c.SendMail(ctx, from, to, body)
if err != nil {
    var smtpErr *smtp.SMTPError
    if errors.As(err, &smtpErr) {
        fmt.Printf("Code: %d\n", smtpErr.Code)
        fmt.Printf("Enhanced: %s\n", smtpErr.EnhancedCode)
        fmt.Printf("Message: %s\n", smtpErr.Message)

        if smtpErr.Temporary() {
            // 4xx — retry later.
        } else {
            // 5xx — permanent failure.
        }
    } else {
        // Network error, timeout, etc.
    }
}
```

### Reply code classification

```go
code := smtpErr.Code
code.IsPositive()   // 2xx or 3xx
code.IsTransient()  // 4xx — temporary failure
code.IsPermanent()  // 5xx — permanent failure
code.Class()        // Returns 2, 3, 4, or 5
```

## Server: Return custom errors

Return `*smtp.SMTPError` from any handler to control the reply sent to the client:

```go
func (h *handler) OnRcpt(_ context.Context, to smtp.ForwardPath) error {
    if to.Mailbox.Domain != "example.com" {
        return &smtp.SMTPError{
            Code:         smtp.ReplyMailboxNotFound,
            EnhancedCode: smtp.EnhancedCodeBadDest,
            Message:      "No such user here",
        }
    }
    return nil
}
```

Or use the `Errorf` convenience function:

```go
return smtp.Errorf(smtp.ReplyMailboxNotFound,
    smtp.EnhancedCodeBadDest,
    "User %s not found", to.Mailbox)
```

### Default behavior

If a handler returns a plain `error` (not `*smtp.SMTPError`), the server sends a generic `451 4.4.0 Internal error`. Always return `*smtp.SMTPError` for intentional rejections.

## Common error patterns

### Temporary failure (retry later)

```go
return smtp.Errorf(smtp.ReplyMailboxBusy,
    smtp.EnhancedCodeTempCongestion,
    "Mailbox busy, try again later")
```

### Permanent failure (do not retry)

```go
return smtp.Errorf(smtp.ReplyMailboxNotFound,
    smtp.EnhancedCodeBadDest,
    "Mailbox does not exist")
```

### Authentication required

```go
return smtp.Errorf(smtp.ReplyAuthRequired,
    smtp.EnhancedCodeAuthRequired,
    "Authentication required")
```

### Message too large

```go
return smtp.Errorf(smtp.ReplyExceededStorage,
    smtp.EnhancedCodeMsgTooLarge,
    "Message exceeds size limit")
```

## See also

- [Shared types reference](../reference/types.md) — all reply codes and enhanced codes
- [Recipient validation](recipient-validation.md) — practical error usage in handlers
