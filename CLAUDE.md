# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`smtp.go` is a comprehensive SMTP library for Go 1.26, providing both client and server implementations. Module path: `github.com/alexisbouchez/smtp.go`. Licensed under MIT. Zero external dependencies — stdlib only.

## Build & Test Commands

```bash
go build ./...                         # Build all packages
go test ./...                          # Run all tests
go test ./... -race                    # Run tests with race detector
go test -run TestName ./smtpclient     # Run a single test in a package
go test -v -count=1 ./...             # Verbose, no cache
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out  # Coverage report
go test -bench=. ./smtpclient          # Run benchmarks
go test -fuzz=FuzzDotRoundTrip -fuzztime=10s ./internal/textproto  # Fuzz tests
go vet ./...                           # Static analysis
```

## Architecture

### Package Layout

- **Root package (`smtp`)** — Shared types: `ReplyCode`, `EnhancedCode`, `SMTPError`, `Mailbox`/`ReversePath`/`ForwardPath`, `Extension`/`Extensions`, and SASL mechanisms (`PlainAuth`, `LoginAuth`, `CramMD5Auth`).
- **`smtpclient`** — SMTP client. `Dial()` connects + EHLO; `Mail()`/`Rcpt()`/`Data()` for transactions; `StartTLS()` for TLS upgrade; `Auth()` for SASL; `Bdat()` for CHUNKING; `SendMail()` convenience; `SubmitMessage()` for RFC 6409 submission. Functional options: `MailOption` (SIZE, BODY, SMTPUTF8, DSN) and `RcptOption` (DSN NOTIFY/ORCPT).
- **`smtpserver`** — SMTP server. `NewServer()` with functional `Option`s; handler interfaces for each command; per-connection session state machine; `WithSubmissionMode()` for RFC 6409; `WithMaxConnections()` for connection limiting; `WithMaxInvalidCommands()` for abuse protection; graceful `Shutdown(ctx)`.
- **`internal/textproto`** — Wire protocol: `Conn` wraps `net.Conn` with buffered I/O, `ReadLine`/`WriteLine`, `ReadReply`/`WriteReply` (multi-line), `DotReader`/`DotWriter` (RFC 5321 §4.5.2), `ParseEnhancedCode`.

### Server Handler Interfaces

Defined in `smtpserver/handler.go`. All take `context.Context` as first arg:

| Interface | Method | When called |
|-----------|--------|-------------|
| `ConnectionHandler` | `OnConnect(ctx, net.Addr)` | New TCP connection |
| `HeloHandler` | `OnHelo(ctx, hostname)` | EHLO/HELO |
| `MailHandler` | `OnMail(ctx, ReversePath)` | MAIL FROM |
| `RcptHandler` | `OnRcpt(ctx, ForwardPath)` | RCPT TO |
| `DataHandler` | `OnData(ctx, from, to[], io.Reader)` | DATA/BDAT body received |
| `AuthHandler` | `Authenticate(ctx, mechanism, user, pass)` | AUTH |
| `ResetHandler` | `OnReset(ctx)` | RSET or implicit reset |
| `VrfyHandler` | `OnVrfy(ctx, param)` | VRFY |

### Server Session State Machine

`stateNew` → `stateGreeted` (EHLO/HELO) → `stateMail` (MAIL FROM) → `stateRcpt` (RCPT TO) → `stateData` (DATA) → back to `stateGreeted`. State enforced: MAIL requires EHLO, RCPT requires MAIL, DATA requires RCPT. Submission mode additionally requires AUTH before MAIL.

### SMTP Extensions (EHLO keywords)

All advertised by the server and detected by the client:

| Extension | RFC | Description |
|-----------|-----|-------------|
| STARTTLS | 3207 | TLS upgrade via `StartTLS()` |
| AUTH | 4954 | SASL authentication (PLAIN, LOGIN, CRAM-MD5) |
| SIZE | 1870 | Message size declaration (`WithSize()`) |
| PIPELINING | 2920 | Command batching (advertised) |
| 8BITMIME | 6152 | 8-bit MIME transport (`WithBody("8BITMIME")`) |
| DSN | 3461 | Delivery status notifications (`WithDSNReturn()`, `WithDSNNotify()`) |
| ENHANCEDSTATUSCODES | 2034 | Enhanced error codes in all replies |
| SMTPUTF8 | 6531 | Internationalized email (`WithSMTPUTF8()`) |
| CHUNKING | 3030 | BDAT command (`Bdat()`) |

### Wire Protocol Layer (`internal/textproto`)

All reads and writes go through this layer. It handles:
- `\r\n` line termination (RFC 5321 §2.3.8)
- Dot-stuffing/destuffing for DATA (RFC 5321 §4.5.2), with bare LF tolerance
- Multi-line reply parsing (`250-` continuation lines)
- Read/write deadlines via `SetDeadlineFromContext()`
- `ReplaceConn()` for TLS upgrade

### Testing Conventions

- Table-driven tests with descriptive subtest names.
- `net.Pipe()` for pure in-process tests (server session_test.go).
- Real TCP with ephemeral ports (`:0`) for client↔server integration tests.
- `startTestServer(t, ...opts)` helper in both test files for quick setup.
- `smtpConversation` helper (server tests) for scripted command/response.
- Fuzz tests for wire protocol layer (dot round-trip, reply parsing).
- Benchmarks: single-message latency, concurrent throughput, large message transfer.
- 117 tests, 3 benchmarks, 2 fuzz targets, 3 testable examples.

## RFCs

The `rfcs/` directory contains the full text of every RFC this library implements. Always reference the RFC text when implementing or reviewing protocol behavior. Key RFCs:

- **RFC 5321** — SMTP protocol (the core spec, start here)
- **RFC 5322** — Internet message format (email headers, structure)
- **RFC 3207** — STARTTLS extension
- **RFC 4954** — AUTH extension
- **RFC 6409** — Message submission (port 587 behavior)

## Maintaining This File

CLAUDE.md must stay in sync with the actual codebase at all times. After completing any phase of work (see TODO.txt) or making significant architectural changes, review and update this file. A stale CLAUDE.md is a bug — fix it before moving on.

## TODO.txt

`TODO.txt` at the repo root is the master roadmap. All 16 phases are complete.

## Code Style

- Standard `gofmt` formatting.
- Exported types/functions get doc comments starting with the identifier name.
- Error format: `fmt.Errorf("smtp: <context>: %w", err)`.
- `SMTPError` for protocol errors with reply code + enhanced code + message.
- `log/slog` for structured logging, configurable via `WithLogger()`.
