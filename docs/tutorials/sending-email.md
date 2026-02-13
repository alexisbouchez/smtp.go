# Tutorial: Send Your First Email

This tutorial walks you through sending an email using the smtp.go client library. By the end, you'll have a working Go program that connects to an SMTP server and delivers a message.

## Prerequisites

- Go 1.26 or later
- Access to an SMTP server (we'll use `localhost:2525` for testing)

## Step 1: Install the library

```bash
go get github.com/alexisbouchez/smtp.go
```

## Step 2: Write the program

Create a file called `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexisbouchez/smtp.go/smtpclient"
)

func main() {
	ctx := context.Background()

	// Connect to the SMTP server and send EHLO.
	c, err := smtpclient.Dial(ctx, "localhost:2525",
		smtpclient.WithLocalName("myclient.example.com"),
	)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// Compose a simple message with headers and body.
	msg := "Subject: Hello from smtp.go\r\n" +
		"From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"\r\n" +
		"This is the body of the message."

	// Send the message.
	err = c.SendMail(ctx,
		"sender@example.com",
		[]string{"recipient@example.com"},
		strings.NewReader(msg),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("Message sent successfully!")
}
```

## Step 3: Understand what happens

When you run this program, the following SMTP conversation takes place:

```
S: 220 localhost ESMTP ready
C: EHLO myclient.example.com
S: 250-localhost Hello myclient.example.com
S: 250 PIPELINING
C: MAIL FROM:<sender@example.com>
S: 250 2.1.0 Originator ok
C: RCPT TO:<recipient@example.com>
S: 250 2.1.5 Recipient ok
C: DATA
S: 354 Start mail input; end with <CRLF>.<CRLF>
C: Subject: Hello from smtp.go
C: From: sender@example.com
C: To: recipient@example.com
C:
C: This is the body of the message.
C: .
S: 250 2.0.0 Message accepted
C: QUIT
S: 221 2.0.0 localhost closing connection
```

`SendMail` is a convenience method that performs `MAIL FROM`, `RCPT TO` for each recipient, and `DATA` in a single call. For finer control over each step, see the [client reference](../reference/client.md).

## Step 4: Run the program

```bash
go run main.go
```

If the server is running, you'll see:

```
Message sent successfully!
```

If the connection fails, you'll get an error like `smtp: dial localhost:2525: connect: connection refused` — make sure your SMTP server is running. You can use the server from the [next tutorial](building-server.md) for testing.

## Step 5: Send to multiple recipients

Change the `SendMail` call to pass multiple addresses:

```go
err = c.SendMail(ctx,
    "sender@example.com",
    []string{
        "alice@example.com",
        "bob@example.com",
    },
    strings.NewReader(msg),
)
```

Each recipient gets a separate `RCPT TO` command, but the message body is only transmitted once.

## Next steps

- [Build your first SMTP server](building-server.md) to test against
- [Upgrade to TLS](../how-to/starttls.md) for encrypted connections
- [Authenticate with the server](../how-to/authentication.md) using SASL
- [Client API reference](../reference/client.md) for all methods and options
