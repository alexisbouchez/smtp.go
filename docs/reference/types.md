# Shared Types Reference

Package `smtp` (root) provides shared types used by both the client and server.

```
import "github.com/alexisbouchez/smtp.go"
```

## Reply Codes

`ReplyCode` is an `int` type representing three-digit SMTP reply codes (RFC 5321 &sect;4.2).

### Constants

| Constant | Value | Meaning |
|----------|-------|---------|
| `ReplySystemStatus` | 211 | System status |
| `ReplyHelpMessage` | 214 | Help message |
| `ReplyServiceReady` | 220 | Service ready |
| `ReplyServiceClosing` | 221 | Service closing |
| `ReplyAuthOK` | 235 | Authentication successful |
| `ReplyOK` | 250 | Requested action completed |
| `ReplyUserNotLocal` | 251 | User not local; will forward |
| `ReplyCannotVRFY` | 252 | Cannot VRFY user |
| `ReplyAuthContinue` | 334 | AUTH challenge continuation |
| `ReplyStartMailInput` | 354 | Start mail input |
| `ReplyServiceNotAvailable` | 421 | Service not available |
| `ReplyMailboxBusy` | 450 | Mailbox busy |
| `ReplyLocalError` | 451 | Local error in processing |
| `ReplyInsufficientStorage` | 452 | Insufficient storage |
| `ReplyTempAuthFailure` | 454 | Temporary auth failure |
| `ReplySyntaxError` | 500 | Syntax error |
| `ReplySyntaxParamError` | 501 | Syntax error in parameters |
| `ReplyCommandNotImpl` | 502 | Command not implemented |
| `ReplyBadSequence` | 503 | Bad sequence of commands |
| `ReplyParamNotImpl` | 504 | Parameter not implemented |
| `ReplyAuthRequired` | 530 | Authentication required |
| `ReplyAuthFailed` | 535 | Authentication failed |
| `ReplyMailboxNotFound` | 550 | Mailbox not found |
| `ReplyUserNotLocalTry` | 551 | User not local, try elsewhere |
| `ReplyExceededStorage` | 552 | Exceeded storage allocation |
| `ReplyMailboxNameError` | 553 | Mailbox name not allowed |
| `ReplyTransactionFailed` | 554 | Transaction failed |
| `ReplyMailRcptParamError` | 555 | MAIL/RCPT parameter error |

### Methods

| Method | Description |
|--------|-------------|
| `Class() int` | First digit: 2, 3, 4, or 5 |
| `IsPositive() bool` | True for 2xx and 3xx |
| `IsTransient() bool` | True for 4xx (temporary failure) |
| `IsPermanent() bool` | True for 5xx (permanent failure) |

### Reply Code Classes

| Constant | Value | Meaning |
|----------|-------|---------|
| `ClassPositiveCompletion` | 2 | Success |
| `ClassPositiveIntermediate` | 3 | Intermediate (more data needed) |
| `ClassTransientNegative` | 4 | Temporary failure |
| `ClassPermanentNegative` | 5 | Permanent failure |

## Enhanced Status Codes

`EnhancedCode` is a struct with `Class`, `Subject`, and `Detail` int fields (RFC 3463). Formatted as `X.Y.Z`.

### Constants

| Variable | Value | Meaning |
|----------|-------|---------|
| `EnhancedCodeOK` | 2.0.0 | Generic success |
| `EnhancedCodeOtherAddress` | 2.1.0 | Other address status (success) |
| `EnhancedCodeDestValid` | 2.1.5 | Destination address valid |
| `EnhancedCodeOtherMailbox` | 2.2.0 | Other mailbox status (success) |
| `EnhancedCodeOtherMail` | 2.6.0 | Other mail system status (success) |
| `EnhancedCodeBadDest` | 5.1.1 | Bad destination mailbox address |
| `EnhancedCodeBadDestSystem` | 5.1.2 | Bad destination system address |
| `EnhancedCodeBadDestSyntax` | 5.1.3 | Bad destination syntax |
| `EnhancedCodeAmbiguousDest` | 5.1.6 | Destination mailbox moved |
| `EnhancedCodeBadSenderSyntax` | 5.1.7 | Bad sender's mailbox syntax |
| `EnhancedCodeBadSenderSystem` | 5.1.8 | Bad sender's system address |
| `EnhancedCodeMailboxFull` | 5.2.2 | Mailbox full |
| `EnhancedCodeMsgTooLarge` | 5.3.4 | Message too big |
| `EnhancedCodeOtherNetwork` | 4.4.0 | Network/routing status (transient) |
| `EnhancedCodeTempCongestion` | 4.4.5 | System congestion (transient) |
| `EnhancedCodeInvalidCommand` | 5.5.1 | Invalid command |
| `EnhancedCodeSyntaxError` | 5.5.2 | Syntax error |
| `EnhancedCodeTooManyRecipients` | 5.5.3 | Too many recipients |
| `EnhancedCodeInvalidParams` | 5.5.4 | Invalid command arguments |
| `EnhancedCodeTempAuthFailure` | 4.7.0 | Security status (transient) |
| `EnhancedCodeAuthRequired` | 5.7.0 | Security status (permanent) |
| `EnhancedCodeAuthCredentials` | 5.7.8 | Authentication credentials invalid |
| `EnhancedCodeEncryptRequired` | 5.7.11 | Encryption required |

### Methods

| Method | Description |
|--------|-------------|
| `String() string` | Formatted as `"X.Y.Z"` |
| `IsZero() bool` | True if all fields are zero |

## SMTPError

```go
type SMTPError struct {
    Code         ReplyCode
    EnhancedCode EnhancedCode
    Message      string
}
```

Represents an SMTP protocol error. Implements the `error` interface.

| Method | Description |
|--------|-------------|
| `Error() string` | `"smtp: 550 5.1.1 No such user"` |
| `Temporary() bool` | True for 4xx codes |
| `WireLines() string` | Formatted as SMTP wire-protocol reply lines |

### Errorf

```go
func Errorf(code ReplyCode, enhanced EnhancedCode, format string, args ...any) *SMTPError
```

Creates an `SMTPError` with a formatted message.

## Address Types

### Mailbox

```go
type Mailbox struct {
    LocalPart string
    Domain    string
}
```

An email address as `local-part@domain`.

| Method | Description |
|--------|-------------|
| `String() string` | `"user@example.com"` |
| `IsZero() bool` | True if both fields are empty |

### ReversePath

```go
type ReversePath struct {
    Mailbox Mailbox
    Null    bool
}
```

The MAIL FROM path. A null reverse-path (`<>`) has `Null == true`.

| Method | Description |
|--------|-------------|
| `String() string` | `"<user@example.com>"` or `"<>"` |

### ForwardPath

```go
type ForwardPath struct {
    Mailbox Mailbox
}
```

The RCPT TO path.

| Method | Description |
|--------|-------------|
| `String() string` | `"<user@example.com>"` |

### Parsing Functions

| Function | Description |
|----------|-------------|
| `ParseMailbox(s) (Mailbox, error)` | Parse `"user@domain"` |
| `ParseReversePath(s) (ReversePath, error)` | Parse `"<user@domain>"` or `"<>"` |
| `ParseForwardPath(s) (ForwardPath, error)` | Parse `"<user@domain>"` |

## Extensions

```go
type Extension string
type Extensions map[Extension]string
```

### Extension Constants

| Constant | Value | RFC |
|----------|-------|-----|
| `ExtSTARTTLS` | `"STARTTLS"` | 3207 |
| `ExtAUTH` | `"AUTH"` | 4954 |
| `ExtSIZE` | `"SIZE"` | 1870 |
| `ExtPIPELINING` | `"PIPELINING"` | 2920 |
| `Ext8BITMIME` | `"8BITMIME"` | 6152 |
| `ExtDSN` | `"DSN"` | 3461 |
| `ExtENHANCEDSTATUSCODES` | `"ENHANCEDSTATUSCODES"` | 2034 |
| `ExtSMTPUTF8` | `"SMTPUTF8"` | 6531 |
| `ExtCHUNKING` | `"CHUNKING"` | 3030 |

### Extensions Methods

| Method | Description |
|--------|-------------|
| `Has(ext) bool` | Check if an extension is advertised |
| `Param(ext) string` | Get the parameter string (e.g., `"PLAIN LOGIN"` for AUTH) |

### ParseEHLOResponse

```go
func ParseEHLOResponse(lines []string) Extensions
```

Parses a multi-line 250 EHLO response into an `Extensions` map.

## SASL Mechanisms

### SASLMechanism Interface

```go
type SASLMechanism interface {
    Name() string
    Start() ([]byte, error)
    Next(challenge []byte) ([]byte, error)
}
```

### Implementations

| Function | Mechanism | Description |
|----------|-----------|-------------|
| `PlainAuth(identity, username, password)` | PLAIN | RFC 4616. Identity is typically empty. |
| `LoginAuth(username, password)` | LOGIN | Legacy challenge-response mechanism. |
| `CramMD5Auth(username, secret)` | CRAM-MD5 | RFC 2195. HMAC-MD5 challenge-response. |

## See also

- [Client API reference](client.md)
- [Server API reference](server.md)
- [Error handling](../how-to/error-handling.md)
