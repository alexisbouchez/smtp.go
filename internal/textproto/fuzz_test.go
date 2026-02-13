package textproto

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func FuzzDotRoundTrip(f *testing.F) {
	f.Add([]byte("Hello\r\n"))
	f.Add([]byte(".leading dot\r\n"))
	f.Add([]byte("..double\r\n"))
	f.Add([]byte(""))
	f.Add([]byte("no trailing newline"))
	f.Add([]byte(".\r\n"))
	f.Add([]byte("Line1\r\n.Line2\r\n..Line3\r\n"))
	f.Add([]byte("\r\n.\r\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		var buf bytes.Buffer
		dw := newDotWriter(bufio.NewWriter(&buf))
		if _, err := dw.Write(data); err != nil {
			return
		}
		if err := dw.Close(); err != nil {
			return
		}

		dr := newDotReader(bufio.NewReader(&buf))
		result, err := io.ReadAll(dr)
		if err != nil {
			t.Fatalf("DotReader failed: %v", err)
		}

		want := string(data)
		if want != "" && !strings.HasSuffix(want, "\n") {
			want += "\r\n"
		}

		if string(result) != want {
			t.Errorf("round-trip mismatch:\n  input:  %q\n  output: %q\n  want:   %q", data, result, want)
		}
	})
}

func FuzzReadReply(f *testing.F) {
	f.Add("250 OK\r\n")
	f.Add("250-Hello\r\n250 World\r\n")
	f.Add("220 Ready\r\n")
	f.Add("550 5.1.1 User unknown\r\n")
	f.Add("250\r\n")

	f.Fuzz(func(t *testing.T, data string) {
		conn := NewConn(&fakeConn{r: strings.NewReader(data)})
		_, _ = conn.ReadReply() // Must not panic.
	})
}

// fakeConn implements net.Conn for fuzzing.
type fakeConn struct {
	r *strings.Reader
}

func (f *fakeConn) Read(b []byte) (int, error)         { return f.r.Read(b) }
func (f *fakeConn) Write(b []byte) (int, error)         { return len(b), nil }
func (f *fakeConn) Close() error                         { return nil }
func (f *fakeConn) LocalAddr() net.Addr                  { return &net.TCPAddr{} }
func (f *fakeConn) RemoteAddr() net.Addr                 { return &net.TCPAddr{} }
func (f *fakeConn) SetDeadline(t time.Time) error        { return nil }
func (f *fakeConn) SetReadDeadline(t time.Time) error    { return nil }
func (f *fakeConn) SetWriteDeadline(t time.Time) error   { return nil }
