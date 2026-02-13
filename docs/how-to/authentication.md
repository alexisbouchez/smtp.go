# How to: Authenticate with SASL

Authenticate SMTP clients using PLAIN, LOGIN, or CRAM-MD5 mechanisms (RFC 4954).

## Client: Authenticate

### PLAIN (recommended)

The most common mechanism. Sends credentials in a single base64-encoded exchange:

```go
err := c.Auth(ctx, smtp.PlainAuth("", "username", "password"))
```

The first argument is the authorization identity — leave it empty to let the server derive it from the username.

### LOGIN

A legacy mechanism supported by many providers. Sends username and password in separate challenge/response steps:

```go
err := c.Auth(ctx, smtp.LoginAuth("username", "password"))
```

### CRAM-MD5

A challenge-response mechanism that avoids sending the password in the clear:

```go
err := c.Auth(ctx, smtp.CramMD5Auth("username", "secret"))
```

The client computes an HMAC-MD5 digest of the server's challenge using the shared secret.

### Check server support

Before authenticating, check which mechanisms the server advertises:

```go
if c.Extensions().Has(smtp.ExtAUTH) {
    params := c.Extensions().Param(smtp.ExtAUTH)
    // params is e.g. "PLAIN LOGIN CRAM-MD5"
}
```

## Server: Handle authentication

Implement the `AuthHandler` interface:

```go
type myAuth struct{}

func (a *myAuth) Authenticate(_ context.Context, mechanism, username, password string) error {
    if username == "user" && password == "pass" {
        return nil // Success.
    }
    return smtp.Errorf(smtp.ReplyAuthFailed,
        smtp.EnhancedCodeAuthCredentials,
        "Invalid credentials")
}
```

Register it with the server:

```go
srv := smtpserver.NewServer(
    smtpserver.WithAuthHandler(&myAuth{}),
    // ...
)
```

When an `AuthHandler` is set, the server advertises `AUTH PLAIN LOGIN CRAM-MD5` in EHLO.

### CRAM-MD5 on the server

For CRAM-MD5, the `password` parameter contains `challenge:digest`. To verify, compute the expected digest from the challenge and the user's stored secret:

```go
func (a *myAuth) Authenticate(_ context.Context, mechanism, username, password string) error {
    if mechanism == "CRAM-MD5" {
        // password is "challenge:digest"
        parts := strings.SplitN(password, ":", 2)
        challenge, clientDigest := parts[0], parts[1]

        mac := hmac.New(md5.New, []byte(storedSecret))
        mac.Write([]byte(challenge))
        expected := hex.EncodeToString(mac.Sum(nil))

        if clientDigest != expected {
            return smtp.Errorf(smtp.ReplyAuthFailed,
                smtp.EnhancedCodeAuthCredentials,
                "Invalid credentials")
        }
        return nil
    }
    // Handle PLAIN/LOGIN...
}
```

## See also

- [STARTTLS](starttls.md) — encrypt the connection before authenticating
- [Message submission](message-submission.md) — combines TLS + AUTH + send
- [Security considerations](../explanation/security.md) — why PLAIN requires TLS
