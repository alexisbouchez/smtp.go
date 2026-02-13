// Package textproto implements the low-level SMTP wire protocol:
// line reading/writing, multi-line reply parsing, and dot-stuffed
// DATA streams. It sits between net.Conn and the SMTP client/server.
package textproto

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// MaxCommandLineLen is the maximum length of an SMTP command line
// including CRLF (RFC 5321 §4.5.3.1.4).
const MaxCommandLineLen = 512

// MaxTextLineLen is the maximum length of a text line in the message body
// including CRLF (RFC 5322 §2.1.1).
const MaxTextLineLen = 1000

// MaxReplyLineLen is a generous limit for reply lines to prevent memory exhaustion.
const MaxReplyLineLen = 2048

// Conn wraps a net.Conn with buffered reading and writing for SMTP protocol I/O.
type Conn struct {
	conn net.Conn
	r    *bufio.Reader
	w    *bufio.Writer
}

// NewConn creates a new protocol Conn wrapping the given network connection.
func NewConn(c net.Conn) *Conn {
	return &Conn{
		conn: c,
		r:    bufio.NewReaderSize(c, 4096),
		w:    bufio.NewWriterSize(c, 4096),
	}
}

// ReplaceConn replaces the underlying net.Conn (used after TLS upgrade)
// and resets the buffered reader/writer.
func (c *Conn) ReplaceConn(nc net.Conn) {
	c.conn = nc
	c.r = bufio.NewReaderSize(nc, 4096)
	c.w = bufio.NewWriterSize(nc, 4096)
}

// NetConn returns the underlying net.Conn.
func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// SetDeadlineFromContext sets the connection read/write deadline from a
// context's deadline. If the context has no deadline, the deadline is cleared.
func (c *Conn) SetDeadlineFromContext(ctx context.Context) {
	if dl, ok := ctx.Deadline(); ok {
		c.conn.SetDeadline(dl)
	} else {
		c.conn.SetDeadline(time.Time{})
	}
}

// SetReadDeadline sets the read deadline on the underlying connection.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline on the underlying connection.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// ReadLine reads a single \r\n-terminated line from the connection.
// The returned line does NOT include the trailing \r\n.
// Returns an error if the line exceeds maxLen bytes (including \r\n).
func (c *Conn) ReadLine(maxLen int) (string, error) {
	var line []byte
	for {
		chunk, isPrefix, err := c.r.ReadLine()
		line = append(line, chunk...)
		if err != nil {
			return "", err
		}
		if !isPrefix {
			break
		}
		// Still reading — check limit.
		if len(line) > maxLen {
			// Drain the rest of the line.
			for isPrefix {
				_, isPrefix, err = c.r.ReadLine()
				if err != nil {
					break
				}
			}
			return "", fmt.Errorf("smtp: line too long (%d bytes, max %d)", len(line), maxLen)
		}
	}
	if len(line) > maxLen-2 { // -2 for the \r\n we already consumed
		return "", fmt.Errorf("smtp: line too long (%d bytes, max %d)", len(line)+2, maxLen)
	}
	return string(line), nil
}

// WriteLine writes a line followed by \r\n and flushes the buffer.
func (c *Conn) WriteLine(line string) error {
	if _, err := c.w.WriteString(line); err != nil {
		return err
	}
	if _, err := c.w.WriteString("\r\n"); err != nil {
		return err
	}
	return c.w.Flush()
}

// WriteLines writes multiple lines, each followed by \r\n, and flushes once.
func (c *Conn) WriteLines(lines ...string) error {
	for _, line := range lines {
		if _, err := c.w.WriteString(line); err != nil {
			return err
		}
		if _, err := c.w.WriteString("\r\n"); err != nil {
			return err
		}
	}
	return c.w.Flush()
}

// Reply represents a parsed SMTP reply (RFC 5321 §4.2).
type Reply struct {
	Code  int      // Three-digit reply code.
	Lines []string // One or more reply text lines (without code or dash/space prefix).
}

// ReadReply reads a single-line or multi-line SMTP reply from the connection.
// Multi-line replies use the "code-hyphen" continuation convention (RFC 5321 §4.2).
func (c *Conn) ReadReply() (Reply, error) {
	var lines []string
	for {
		line, err := c.ReadLine(MaxReplyLineLen)
		if err != nil {
			return Reply{}, fmt.Errorf("smtp: reading reply: %w", err)
		}

		if len(line) < 3 {
			return Reply{}, errors.New("smtp: reply line too short")
		}

		code, err := strconv.Atoi(line[:3])
		if err != nil {
			return Reply{}, fmt.Errorf("smtp: invalid reply code %q: %w", line[:3], err)
		}

		if len(line) == 3 {
			// "250\r\n" with no text — final line.
			lines = append(lines, "")
			return Reply{Code: code, Lines: lines}, nil
		}

		sep := line[3]
		text := line[4:]

		switch sep {
		case '-':
			// Continuation line.
			lines = append(lines, text)
		case ' ':
			// Final line.
			lines = append(lines, text)
			return Reply{Code: code, Lines: lines}, nil
		default:
			return Reply{}, fmt.Errorf("smtp: invalid reply separator %q", sep)
		}
	}
}

// WriteReply writes a single-line or multi-line reply to the connection.
func (c *Conn) WriteReply(code int, lines ...string) error {
	if len(lines) == 0 {
		lines = []string{""}
	}
	for i, line := range lines {
		var sep byte = ' '
		if i < len(lines)-1 {
			sep = '-'
		}
		s := fmt.Sprintf("%d%c%s", code, sep, line)
		if _, err := c.w.WriteString(s); err != nil {
			return err
		}
		if _, err := c.w.WriteString("\r\n"); err != nil {
			return err
		}
	}
	return c.w.Flush()
}

// BufReader returns the underlying buffered reader. This is needed by the
// DotReader to read from the buffered stream.
func (c *Conn) BufReader() *bufio.Reader {
	return c.r
}

// BufWriter returns the underlying buffered writer.
func (c *Conn) BufWriter() *bufio.Writer {
	return c.w
}

// Cmd sends a command line and reads the reply. Convenience method for
// simple command/response exchanges.
func (c *Conn) Cmd(format string, args ...any) (Reply, error) {
	cmd := fmt.Sprintf(format, args...)
	if err := c.WriteLine(cmd); err != nil {
		return Reply{}, err
	}
	return c.ReadReply()
}

// DotReader returns an io.Reader that reads the dot-stuffed DATA body
// from the connection. It transparently removes dot-stuffing and stops
// reading at the termination sequence "\r\n.\r\n" (RFC 5321 §4.5.2).
func (c *Conn) DotReader() io.Reader {
	return newDotReader(c.r)
}

// DotWriter returns an io.WriteCloser that writes dot-stuffed DATA to
// the connection. Calling Close writes the termination sequence "\r\n.\r\n"
// and flushes the buffer (RFC 5321 §4.5.2).
func (c *Conn) DotWriter() io.WriteCloser {
	return newDotWriter(c.w)
}

// ParseEnhancedCode attempts to parse an enhanced status code from the
// beginning of a reply text line. Returns the code and remaining text,
// or a zero code and the original text if no enhanced code is found.
func ParseEnhancedCode(text string) (class, subject, detail int, rest string) {
	// Enhanced code format: "X.Y.Z rest..."
	parts := strings.SplitN(text, " ", 2)
	code := parts[0]
	rest = ""
	if len(parts) == 2 {
		rest = parts[1]
	}

	segments := strings.Split(code, ".")
	if len(segments) != 3 {
		return 0, 0, 0, text
	}

	c, err1 := strconv.Atoi(segments[0])
	s, err2 := strconv.Atoi(segments[1])
	d, err3 := strconv.Atoi(segments[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, text
	}
	if c < 2 || c > 5 {
		return 0, 0, 0, text
	}

	return c, s, d, rest
}
