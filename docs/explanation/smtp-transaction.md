# The SMTP Transaction Model

This document explains how SMTP conversations work and how the server's state machine enforces command ordering.

## The conversation model

An SMTP session is a sequence of commands and replies over a single TCP connection. The server speaks first with a greeting, and the client responds with commands:

```
S: 220 mail.example.com ESMTP ready        ← Greeting
C: EHLO client.example.com                  ← Client identifies itself
S: 250-mail.example.com Hello client...     ← Server lists extensions
S: 250 PIPELINING
C: MAIL FROM:<sender@example.com>           ← Start a transaction
S: 250 2.1.0 Originator ok
C: RCPT TO:<recipient@example.com>          ← Add a recipient
S: 250 2.1.5 Recipient ok
C: DATA                                      ← Begin message transfer
S: 354 Start mail input
C: Subject: Hello                            ← Message body
C: ...                                       ← (dot-stuffed)
C: .                                         ← End of message
S: 250 2.0.0 Message accepted
C: QUIT                                      ← End session
S: 221 2.0.0 Closing connection
```

This sequence — EHLO, MAIL, RCPT, DATA — is the core SMTP transaction. Multiple transactions can occur on a single connection.

## EHLO vs HELO

EHLO (Extended HELO) is the modern greeting command. The server responds with its hostname and a list of supported extensions:

```
250-mail.example.com Hello client
250-SIZE 10485760
250-STARTTLS
250-AUTH PLAIN LOGIN
250 PIPELINING
```

HELO is the original RFC 821 command. It receives a simple `250` response with no extensions. The client library tries EHLO first and falls back to HELO if the server rejects it.

## Extension negotiation

Extensions are advertised in the EHLO response as keyword lines. Each keyword may have parameters (e.g., `SIZE 10485760` or `AUTH PLAIN LOGIN CRAM-MD5`).

The client checks for extensions before using them:

```go
if c.Extensions().Has(smtp.ExtSTARTTLS) {
    c.StartTLS(ctx, tlsConfig)
}
```

After STARTTLS upgrades the connection, both sides reset state, and the client must re-issue EHLO. The server may advertise different extensions after TLS (e.g., removing STARTTLS, adding AUTH).

## The state machine

The server enforces a strict command ordering through a state machine:

```
                    ┌──── STARTTLS ────┐
                    v                  │
stateNew ──EHLO/HELO──> stateGreeted ──┤
                             │         │
                         MAIL FROM     │
                             v         │
                         stateMail     │
                             │         │
                          RCPT TO      │
                             v         │
                         stateRcpt     │
                             │         │
                        DATA/BDAT LAST │
                             │         │
                             └─────────┘
                        (back to stateGreeted)
```

| State | Allowed commands |
|-------|-----------------|
| **stateNew** | EHLO, HELO |
| **stateGreeted** | MAIL FROM, AUTH, STARTTLS, VRFY, NOOP, RSET, QUIT |
| **stateMail** | RCPT TO, RSET, NOOP, QUIT |
| **stateRcpt** | RCPT TO, DATA, BDAT, RSET, NOOP, QUIT |

Commands sent out of order receive `503 Bad sequence of commands`.

### Submission mode

When submission mode is enabled (`WithSubmissionMode(true)`), the server adds an additional constraint: MAIL FROM requires prior successful AUTH. Unauthenticated clients receive `530 5.7.0 Authentication required`.

### Resets

The transaction state resets to `stateGreeted` in these situations:

- **RSET command** — explicit reset
- **After DATA/BDAT LAST** — successful or failed delivery
- **EHLO/HELO re-issue** — implicit reset
- **STARTTLS** — resets all the way to stateNew

The `ResetHandler` is called on each reset, allowing the application to clean up per-transaction state.

## Multiple transactions

A single connection can carry multiple transactions. After DATA completes, the session returns to `stateGreeted` and the client can issue another MAIL FROM:

```
C: MAIL FROM:<alice@example.com>
S: 250 OK
C: RCPT TO:<bob@example.com>
S: 250 OK
C: DATA
S: 354 Start mail input
C: ...
C: .
S: 250 Message accepted
C: MAIL FROM:<alice@example.com>    ← Second transaction
S: 250 OK
...
```

## DATA vs BDAT

DATA uses dot-stuffing: the message body is terminated by a line containing only a period (`.`). Lines beginning with a period in the original message are doubled (`..`). The wire protocol layer handles this transparently.

BDAT (RFC 3030) sends binary chunks with explicit sizes. No dot-stuffing is needed. The final chunk is marked with `LAST`:

```
C: BDAT 1000
C: [1000 bytes of data]
S: 250 OK
C: BDAT 500 LAST
C: [500 bytes of data]
S: 250 Message accepted
```

Both DATA and BDAT deliver to the same `DataHandler`.

## See also

- [Architecture](architecture.md) — package layout and design decisions
- [Security considerations](security.md) — where TLS and AUTH fit in the transaction
- [Server API reference](../reference/server.md) — state machine and handler interfaces
