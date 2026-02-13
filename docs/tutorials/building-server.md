# Tutorial: Build Your First SMTP Server

This tutorial walks you through building a minimal SMTP server that receives and logs email messages. By the end, you'll have a working server you can test with the client from the [previous tutorial](sending-email.md).

## Prerequisites

- Go 1.26 or later
- Completed the [sending email tutorial](sending-email.md) (recommended)

## Step 1: Install the library

```bash
go get github.com/alexisbouchez/smtp.go
```

## Step 2: Create a message handler

The server calls handler interfaces at each stage of the SMTP conversation. The only one you need to get started is `DataHandler`, which receives the message body.

Create a file called `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/smtpserver"
)

type handler struct{}

func (h *handler) OnData(_ context.Context, from smtp.ReversePath, to []smtp.ForwardPath, r io.Reader) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	fmt.Printf("--- New message ---\n")
	fmt.Printf("From: %s\n", from.Mailbox)
	fmt.Printf("To:   ")
	for i, rcpt := range to {
		if i > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf("%s", rcpt.Mailbox)
	}
	fmt.Printf("\nBody (%d bytes):\n%s\n\n", len(body), body)

	return nil
}

func main() {
	srv := smtpserver.NewServer(
		smtpserver.WithAddr(":2525"),
		smtpserver.WithHostname("mail.example.com"),
		smtpserver.WithDataHandler(&handler{}),
	)

	fmt.Println("SMTP server listening on :2525")
	log.Fatal(srv.ListenAndServe())
}
```

## Step 3: Understand the structure

The server is built around three concepts:

1. **`NewServer`** creates the server with functional options for configuration.
2. **Handler interfaces** (like `DataHandler`) are called at each protocol stage. All are optional — the server handles the protocol automatically.
3. **`ListenAndServe`** starts accepting connections and blocks until shutdown.

The `OnData` method receives:
- `from` — the sender address from `MAIL FROM`
- `to` — the list of recipients from `RCPT TO`
- `r` — an `io.Reader` with the de-stuffed message body

## Step 4: Run the server

```bash
go run main.go
```

```
SMTP server listening on :2525
```

## Step 5: Test with the client

In another terminal, run the client from the [sending email tutorial](sending-email.md). The server will print:

```
--- New message ---
From: sender@example.com
To:   recipient@example.com
Body (98 bytes):
Subject: Hello from smtp.go
From: sender@example.com
To: recipient@example.com

This is the body of the message.
```

## Step 6: Add sender validation

Let's reject messages from a blocked domain by implementing `MailHandler`:

```go
type handler struct{}

func (h *handler) OnMail(_ context.Context, from smtp.ReversePath) error {
	if from.Mailbox.Domain == "blocked.example.com" {
		return smtp.Errorf(smtp.ReplyMailboxNotFound,
			smtp.EnhancedCodeBadSenderSyntax,
			"Sender domain not allowed")
	}
	return nil
}
```

Register it alongside the data handler:

```go
srv := smtpserver.NewServer(
    smtpserver.WithAddr(":2525"),
    smtpserver.WithHostname("mail.example.com"),
    smtpserver.WithMailHandler(&handler{}),
    smtpserver.WithDataHandler(&handler{}),
)
```

When the same struct implements multiple handler interfaces, you can pass it to multiple `With*Handler` options.

## Next steps

- [Add TLS support](../how-to/starttls.md) for encrypted connections
- [Add authentication](../how-to/authentication.md) with SASL
- [Validate recipients](../how-to/recipient-validation.md) on the server
- [Limit connections](../how-to/connection-limiting.md) for abuse protection
- [Graceful shutdown](../how-to/graceful-shutdown.md) for production deployments
- [Server API reference](../reference/server.md) for all options and handlers
