# How to: Shut Down the Server Gracefully

Stop accepting new connections and wait for active sessions to finish.

## Basic shutdown

Use `Shutdown` with a context deadline:

```go
srv := smtpserver.NewServer(
    smtpserver.WithAddr(":25"),
    smtpserver.WithHostname("mail.example.com"),
    smtpserver.WithDataHandler(&handler{}),
)

// Start serving in a goroutine.
go func() {
    if err := srv.ListenAndServe(); err != nil {
        log.Printf("server error: %v", err)
    }
}()

// Wait for interrupt signal.
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

log.Println("Shutting down...")

// Give active sessions 30 seconds to finish.
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := srv.Shutdown(ctx); err != nil {
    log.Printf("shutdown error: %v", err)
}

log.Println("Server stopped")
```

## How it works

`Shutdown`:
1. Closes the listener — no new connections are accepted
2. Signals active sessions to close (they receive `421 Server shutting down` on their next command)
3. Waits for all active sessions to finish, or until the context deadline

If the context expires before all sessions finish, `Shutdown` returns the context error. Active connections are left open.

## Immediate close

For a hard shutdown that closes the listener without waiting:

```go
srv.Close()
```

This closes the listener immediately. Active connections will fail on their next read/write.

## See also

- [Connection limiting](connection-limiting.md) — control concurrent connections
- [Server API reference](../reference/server.md) — Shutdown and Close methods
