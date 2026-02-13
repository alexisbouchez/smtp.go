# How to: Upgrade Connections to TLS

Encrypt SMTP connections using STARTTLS (RFC 3207).

## Client: Upgrade to TLS

After connecting, check for STARTTLS support and upgrade:

```go
c, err := smtpclient.Dial(ctx, "mail.example.com:25")
if err != nil {
    return err
}
defer c.Close()

if c.Extensions().Has(smtp.ExtSTARTTLS) {
    err := c.StartTLS(ctx, &tls.Config{
        ServerName: "mail.example.com",
    })
    if err != nil {
        return err
    }
}

// Connection is now encrypted. Continue with Mail/Rcpt/Data.
```

`StartTLS` sends the STARTTLS command, performs the TLS handshake, and automatically re-issues EHLO to refresh the server's extension list.

Check the connection state with `c.IsTLS()`.

## Server: Enable STARTTLS

Pass a `*tls.Config` to the server:

```go
cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
if err != nil {
    log.Fatal(err)
}

srv := smtpserver.NewServer(
    smtpserver.WithAddr(":25"),
    smtpserver.WithHostname("mail.example.com"),
    smtpserver.WithTLSConfig(&tls.Config{
        Certificates: []tls.Certificate{cert},
    }),
    smtpserver.WithDataHandler(&handler{}),
)
```

When a TLS config is set, the server advertises `STARTTLS` in its EHLO response. After a successful upgrade, the session state resets and the client must re-issue EHLO.

## See also

- [Authentication](authentication.md) — typically performed after STARTTLS
- [Message submission](message-submission.md) — combines STARTTLS + AUTH in one call
- [Security considerations](../explanation/security.md) — why STARTTLS matters
