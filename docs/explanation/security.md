# Security Considerations

This document explains the security model of smtp.go: how TLS, authentication, and submission mode work together to protect email in transit.

## STARTTLS vs implicit TLS

SMTP supports two approaches to encryption:

**STARTTLS (RFC 3207)** upgrades an existing plaintext connection to TLS. The client connects on the standard port (25 or 587), negotiates capabilities via EHLO, and then issues the STARTTLS command. After a successful TLS handshake, both sides reset state and the client re-issues EHLO over the encrypted channel.

**Implicit TLS** (port 465) establishes TLS immediately on connection, before any SMTP commands are exchanged. This library does not provide a built-in implicit TLS mode, but you can achieve it by wrapping a `tls.Listener` and passing it to `Server.Serve()`, or by connecting the client through a `tls.Conn` and using `NewClient`.

STARTTLS is the standard approach for MTA-to-MTA communication (port 25) and message submission (port 587).

## Why PLAIN authentication requires TLS

SASL PLAIN sends the username and password in base64 encoding (not encryption). Without TLS, credentials are transmitted in the clear and can be intercepted by anyone on the network.

Best practice: always upgrade to TLS before authenticating. `SubmitMessage` does this automatically — it calls `StartTLS` before `Auth`.

The server's submission mode (`WithSubmissionMode(true)`) enforces that clients authenticate before sending mail, but it does not enforce TLS. For production deployments, configure TLS and consider rejecting AUTH attempts on non-TLS connections in your `AuthHandler`.

## SASL mechanisms compared

| Mechanism | Sends password in clear? | Requires TLS? | Server needs plaintext password? |
|-----------|:------------------------:|:-------------:|:--------------------------------:|
| PLAIN | Yes (base64 only) | Strongly recommended | No (receives it directly) |
| LOGIN | Yes (base64 only) | Strongly recommended | No (receives it directly) |
| CRAM-MD5 | No (challenge-response) | Optional | Yes (must compute HMAC) |

**PLAIN** is the simplest and most widely supported. Safe over TLS.

**LOGIN** is a legacy mechanism with the same security properties as PLAIN. Supported for compatibility with older clients.

**CRAM-MD5** uses HMAC-MD5 so the password never crosses the wire, but the server must store or have access to the plaintext password to verify the digest. It does not protect against replay attacks or provide forward secrecy. TLS is still recommended.

## Submission mode as a security boundary

Message submission (RFC 6409, port 587) is designed for mail user agents (MUAs) submitting mail to their outbound server. Submission mode enforces:

1. **Authentication required** — unauthenticated MAIL FROM receives `530`.
2. **Typically paired with TLS** — submission servers should have STARTTLS configured.

This prevents open relay behavior: only authenticated users can inject messages into the mail system.

```go
srv := smtpserver.NewServer(
    smtpserver.WithAddr(":587"),
    smtpserver.WithTLSConfig(tlsConfig),
    smtpserver.WithAuthHandler(authHandler),
    smtpserver.WithSubmissionMode(true),
)
```

## Connection-level protection

The server provides several mechanisms to limit abuse at the connection level:

- **`WithMaxConnections(n)`** — limits concurrent TCP connections
- **`WithMaxInvalidCommands(n)`** — disconnects clients sending too many bad commands
- **`WithMaxRecipients(n)`** — limits recipients per transaction
- **`WithMaxMessageSize(n)`** — limits message body size
- **`ConnectionHandler`** — allows IP-based filtering at connection time

These are protocol-level defenses. For production systems, also consider network-level protections (firewalls, rate limiting, fail2ban) and DNS-based authentication (SPF, DKIM, DMARC).

## Session state reset after TLS

After a successful STARTTLS handshake, the server resets all session state: the client must re-issue EHLO, and any prior authentication or transaction state is cleared. This is mandated by RFC 3207 &sect;4.2 and ensures that pre-TLS state (which could have been tampered with by a MITM) is not carried into the secure session.

The server may advertise different extensions after TLS. For example, it removes STARTTLS (already active) and may add AUTH (now that the connection is encrypted).

## See also

- [How to: STARTTLS](../how-to/starttls.md) — client and server setup
- [How to: Authentication](../how-to/authentication.md) — SASL mechanism usage
- [How to: Message submission](../how-to/message-submission.md) — port 587 workflow
- [How to: Connection limiting](../how-to/connection-limiting.md) — abuse protection
