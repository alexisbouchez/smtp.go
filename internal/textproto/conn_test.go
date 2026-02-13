package textproto

import (
	"net"
	"strings"
	"testing"
)

func TestReadLine(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewConn(server)

	go func() {
		client.Write([]byte("EHLO example.com\r\n"))
		client.Write([]byte("QUIT\r\n"))
	}()

	line, err := conn.ReadLine(MaxCommandLineLen)
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if line != "EHLO example.com" {
		t.Errorf("ReadLine = %q, want %q", line, "EHLO example.com")
	}

	line, err = conn.ReadLine(MaxCommandLineLen)
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if line != "QUIT" {
		t.Errorf("ReadLine = %q, want %q", line, "QUIT")
	}
}

func TestReadLine_TooLong(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewConn(server)

	go func() {
		long := strings.Repeat("A", 600) + "\r\n"
		client.Write([]byte(long))
	}()

	_, err := conn.ReadLine(MaxCommandLineLen)
	if err == nil {
		t.Fatal("expected error for oversized line")
	}
	if !strings.Contains(err.Error(), "line too long") {
		t.Errorf("error = %v, want 'line too long'", err)
	}
}

func TestWriteLine(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewConn(server)

	go func() {
		conn.WriteLine("250 OK")
	}()

	buf := make([]byte, 64)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got := string(buf[:n])
	if got != "250 OK\r\n" {
		t.Errorf("got %q, want %q", got, "250 OK\r\n")
	}
}

func TestReadReply_SingleLine(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewConn(server)

	go func() {
		client.Write([]byte("250 OK\r\n"))
	}()

	reply, err := conn.ReadReply()
	if err != nil {
		t.Fatalf("ReadReply: %v", err)
	}
	if reply.Code != 250 {
		t.Errorf("Code = %d, want 250", reply.Code)
	}
	if len(reply.Lines) != 1 || reply.Lines[0] != "OK" {
		t.Errorf("Lines = %v, want [\"OK\"]", reply.Lines)
	}
}

func TestReadReply_MultiLine(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewConn(server)

	go func() {
		client.Write([]byte("250-mail.example.com Hello\r\n"))
		client.Write([]byte("250-SIZE 52428800\r\n"))
		client.Write([]byte("250-PIPELINING\r\n"))
		client.Write([]byte("250 STARTTLS\r\n"))
	}()

	reply, err := conn.ReadReply()
	if err != nil {
		t.Fatalf("ReadReply: %v", err)
	}
	if reply.Code != 250 {
		t.Errorf("Code = %d, want 250", reply.Code)
	}
	if len(reply.Lines) != 4 {
		t.Fatalf("len(Lines) = %d, want 4", len(reply.Lines))
	}
	expected := []string{
		"mail.example.com Hello",
		"SIZE 52428800",
		"PIPELINING",
		"STARTTLS",
	}
	for i, want := range expected {
		if reply.Lines[i] != want {
			t.Errorf("Lines[%d] = %q, want %q", i, reply.Lines[i], want)
		}
	}
}

func TestReadReply_NoText(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewConn(server)

	go func() {
		client.Write([]byte("250\r\n"))
	}()

	reply, err := conn.ReadReply()
	if err != nil {
		t.Fatalf("ReadReply: %v", err)
	}
	if reply.Code != 250 {
		t.Errorf("Code = %d, want 250", reply.Code)
	}
}

func TestReadReply_InvalidCode(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewConn(server)

	go func() {
		client.Write([]byte("XYZ Bad\r\n"))
	}()

	_, err := conn.ReadReply()
	if err == nil {
		t.Fatal("expected error for invalid reply code")
	}
}

func TestWriteReply(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewConn(server)

	go func() {
		conn.WriteReply(250, "mail.example.com", "SIZE 1000", "OK")
	}()

	buf := make([]byte, 256)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got := string(buf[:n])
	want := "250-mail.example.com\r\n250-SIZE 1000\r\n250 OK\r\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCmd(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	sConn := NewConn(server)
	cConn := NewConn(client)

	go func() {
		line, _ := sConn.ReadLine(MaxCommandLineLen)
		if line == "NOOP" {
			sConn.WriteReply(250, "OK")
		}
	}()

	reply, err := cConn.Cmd("NOOP")
	if err != nil {
		t.Fatalf("Cmd: %v", err)
	}
	if reply.Code != 250 {
		t.Errorf("Code = %d, want 250", reply.Code)
	}
}

func TestParseEnhancedCode(t *testing.T) {
	tests := []struct {
		text                          string
		wantClass, wantSubject, wantDetail int
		wantRest                      string
	}{
		{"2.0.0 OK", 2, 0, 0, "OK"},
		{"5.1.1 User unknown", 5, 1, 1, "User unknown"},
		{"4.4.5 System congestion", 4, 4, 5, "System congestion"},
		{"OK", 0, 0, 0, "OK"},
		{"bad.code here", 0, 0, 0, "bad.code here"},
		{"2.0.0", 2, 0, 0, ""},
		{"1.0.0 Invalid class", 0, 0, 0, "1.0.0 Invalid class"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			c, s, d, rest := ParseEnhancedCode(tt.text)
			if c != tt.wantClass || s != tt.wantSubject || d != tt.wantDetail {
				t.Errorf("ParseEnhancedCode(%q) code = %d.%d.%d, want %d.%d.%d",
					tt.text, c, s, d, tt.wantClass, tt.wantSubject, tt.wantDetail)
			}
			if rest != tt.wantRest {
				t.Errorf("ParseEnhancedCode(%q) rest = %q, want %q", tt.text, rest, tt.wantRest)
			}
		})
	}
}
