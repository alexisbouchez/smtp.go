# How to: Limit Concurrent Connections and Abuse

Protect the server from resource exhaustion and abuse.

## Limit concurrent connections

```go
srv := smtpserver.NewServer(
    smtpserver.WithMaxConnections(100),
    // ...
)
```

When the limit is reached, new connections receive `421 4.7.0 Too many connections, try again later` and are closed immediately. A value of 0 (the default) means unlimited.

## Limit invalid commands

Disconnect clients that send too many unrecognized or out-of-sequence commands:

```go
srv := smtpserver.NewServer(
    smtpserver.WithMaxInvalidCommands(5),
    // ...
)
```

After 5 invalid commands, the server sends `421 4.4.0 Too many errors, closing connection` and drops the connection. The default limit is 10.

## Limit recipients per message

Cap the number of `RCPT TO` commands in a single transaction:

```go
srv := smtpserver.NewServer(
    smtpserver.WithMaxRecipients(50),
    // ...
)
```

When exceeded, the server replies `452 5.5.3 Too many recipients`. The default is 100.

## Limit message size

Declare and enforce a maximum message size:

```go
srv := smtpserver.NewServer(
    smtpserver.WithMaxMessageSize(25 * 1024 * 1024), // 25 MB
    // ...
)
```

The server advertises the limit via the `SIZE` extension in EHLO. The default is 10 MB.

## IP-based filtering

Implement `ConnectionHandler` to reject connections by IP:

```go
type connFilter struct{}

func (f *connFilter) OnConnect(_ context.Context, addr net.Addr) error {
    tcpAddr, ok := addr.(*net.TCPAddr)
    if !ok {
        return nil
    }
    if isBlocked(tcpAddr.IP) {
        return smtp.Errorf(smtp.ReplyServiceNotAvailable,
            smtp.EnhancedCodeAuthRequired,
            "Connection refused")
    }
    return nil
}
```

```go
srv := smtpserver.NewServer(
    smtpserver.WithConnectionHandler(&connFilter{}),
    // ...
)
```

## Set timeouts

Control read and write deadlines for slow clients:

```go
srv := smtpserver.NewServer(
    smtpserver.WithReadTimeout(2 * time.Minute),
    smtpserver.WithWriteTimeout(2 * time.Minute),
    // ...
)
```

The defaults are 5 minutes each.

## See also

- [Graceful shutdown](graceful-shutdown.md) — shut down the server cleanly
- [Server API reference](../reference/server.md) — all server options and defaults
