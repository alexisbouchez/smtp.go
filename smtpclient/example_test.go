package smtpclient_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/alexisbouchez/smtp.go"
	"github.com/alexisbouchez/smtp.go/smtpclient"
)

func Example() {
	ctx := context.Background()
	c, err := smtpclient.Dial(ctx, "mail.example.com:25",
		smtpclient.WithLocalName("client.example.com"),
		smtpclient.WithTimeout(30*time.Second),
	)
	if err != nil {
		fmt.Println("dial error:", err)
		return
	}
	defer c.Close()

	body := "Subject: Hello\r\n\r\nHello from smtp.go!"
	err = c.SendMail(ctx,
		"sender@example.com",
		[]string{"recipient@example.com"},
		strings.NewReader(body),
	)
	if err != nil {
		fmt.Println("send error:", err)
		return
	}
	fmt.Println("Message sent!")
}

func ExampleClient_StartTLS() {
	ctx := context.Background()
	c, err := smtpclient.Dial(ctx, "mail.example.com:25")
	if err != nil {
		fmt.Println("dial error:", err)
		return
	}
	defer c.Close()

	if c.Extensions().Has(smtp.ExtSTARTTLS) {
		err = c.StartTLS(ctx, &tls.Config{ServerName: "mail.example.com"})
		if err != nil {
			fmt.Println("STARTTLS error:", err)
			return
		}
		fmt.Println("TLS:", c.IsTLS())
	}
}

func ExampleClient_Auth() {
	ctx := context.Background()
	c, err := smtpclient.Dial(ctx, "mail.example.com:587")
	if err != nil {
		fmt.Println("dial error:", err)
		return
	}
	defer c.Close()

	err = c.Auth(ctx, smtp.PlainAuth("", "user@example.com", "password"))
	if err != nil {
		fmt.Println("auth error:", err)
		return
	}
	fmt.Println("Authenticated!")
}

func ExampleClient_SubmitMessage() {
	ctx := context.Background()
	c, err := smtpclient.Dial(ctx, "mail.example.com:587",
		smtpclient.WithLocalName("client.example.com"),
	)
	if err != nil {
		fmt.Println("dial error:", err)
		return
	}
	defer c.Close()

	body := "Subject: Submission\r\n\r\nSent via message submission."
	err = c.SubmitMessage(ctx,
		smtp.PlainAuth("", "user@example.com", "password"),
		&tls.Config{ServerName: "mail.example.com"},
		"user@example.com",
		[]string{"recipient@example.com"},
		strings.NewReader(body),
	)
	if err != nil {
		fmt.Println("submit error:", err)
		return
	}
	fmt.Println("Message submitted!")
}
