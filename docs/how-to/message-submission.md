# How to: Submit Messages on Port 587

Use RFC 6409 message submission for authenticated email sending.

## Client: Submit a message

`SubmitMessage` combines STARTTLS, AUTH, and SendMail into a single call:

```go
c, err := smtpclient.Dial(ctx, "mail.example.com:587")
if err != nil {
    return err
}
defer c.Close()

err = c.SubmitMessage(ctx,
    smtp.PlainAuth("", "user@example.com", "password"),
    &tls.Config{ServerName: "mail.example.com"},
    "user@example.com",
    []string{"recipient@example.com"},
    strings.NewReader(body),
)
```

The method:
1. Upgrades to TLS via STARTTLS (if available and not already TLS)
2. Authenticates with the provided SASL mechanism
3. Sends the message via MAIL/RCPT/DATA

If the connection is already TLS, the STARTTLS step is skipped.

## Server: Enable submission mode

Submission mode requires clients to authenticate before sending `MAIL FROM`:

```go
srv := smtpserver.NewServer(
    smtpserver.WithAddr(":587"),
    smtpserver.WithHostname("mail.example.com"),
    smtpserver.WithTLSConfig(tlsConfig),
    smtpserver.WithAuthHandler(&myAuth{}),
    smtpserver.WithSubmissionMode(true),
    smtpserver.WithDataHandler(&handler{}),
)
```

With submission mode enabled, unauthenticated `MAIL FROM` commands receive a `530 5.7.0 Authentication required` reply.

## See also

- [STARTTLS](starttls.md) — how TLS upgrade works
- [Authentication](authentication.md) — SASL mechanism details
- [SMTP transaction model](../explanation/smtp-transaction.md) — state machine and command ordering
