package textproto

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestDotWriter_Basic(t *testing.T) {
	var buf bytes.Buffer
	w := newDotWriter(bufio.NewWriter(&buf))

	_, err := w.Write([]byte("Hello, World!\r\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := buf.String()
	want := "Hello, World!\r\n.\r\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDotWriter_StuffsLeadingDots(t *testing.T) {
	var buf bytes.Buffer
	w := newDotWriter(bufio.NewWriter(&buf))

	_, err := w.Write([]byte(".leading dot\r\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := buf.String()
	want := "..leading dot\r\n.\r\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDotWriter_NoTrailingCRLF(t *testing.T) {
	var buf bytes.Buffer
	w := newDotWriter(bufio.NewWriter(&buf))

	_, err := w.Write([]byte("no trailing newline"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := buf.String()
	// Close should add \r\n before the terminator.
	want := "no trailing newline\r\n.\r\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDotWriter_EmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	w := newDotWriter(bufio.NewWriter(&buf))
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := buf.String()
	// Empty message: just the terminator.
	want := ".\r\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDotWriter_MultipleDotsOnLine(t *testing.T) {
	var buf bytes.Buffer
	w := newDotWriter(bufio.NewWriter(&buf))

	_, err := w.Write([]byte("...three dots\r\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := buf.String()
	// Only the first dot gets stuffed.
	want := "....three dots\r\n.\r\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDotReader_Basic(t *testing.T) {
	input := "Hello, World!\r\n.\r\n"
	r := newDotReader(bufio.NewReader(strings.NewReader(input)))

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	want := "Hello, World!\r\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", data, want)
	}
}

func TestDotReader_Destuffs(t *testing.T) {
	input := "..leading dot\r\n.\r\n"
	r := newDotReader(bufio.NewReader(strings.NewReader(input)))

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	want := ".leading dot\r\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", data, want)
	}
}

func TestDotReader_EmptyMessage(t *testing.T) {
	input := ".\r\n"
	r := newDotReader(bufio.NewReader(strings.NewReader(input)))

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("got %q, want empty", data)
	}
}

func TestDotRoundTrip(t *testing.T) {
	messages := []string{
		"Simple message\r\n",
		".starts with dot\r\n",
		"..double dot\r\n",
		"...triple\r\n",
		"Line 1\r\n.Line 2\r\n..Line 3\r\n",
		"", // empty message
		"no trailing crlf",
		"Multi\r\nLine\r\nMessage\r\n",
		".\r\n", // a line that is just a dot
	}

	for _, msg := range messages {
		t.Run("", func(t *testing.T) {
			// Write through dot writer.
			var buf bytes.Buffer
			dw := newDotWriter(bufio.NewWriter(&buf))
			if _, err := dw.Write([]byte(msg)); err != nil {
				t.Fatalf("DotWriter.Write: %v", err)
			}
			if err := dw.Close(); err != nil {
				t.Fatalf("DotWriter.Close: %v", err)
			}

			// Read back through dot reader.
			dr := newDotReader(bufio.NewReader(&buf))
			data, err := io.ReadAll(dr)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}

			// For messages without trailing \r\n, the writer adds one.
			want := msg
			if want != "" && !strings.HasSuffix(want, "\n") {
				want += "\r\n"
			}

			if string(data) != want {
				t.Errorf("round-trip failed:\n  input:  %q\n  output: %q\n  want:   %q", msg, data, want)
			}
		})
	}
}

func TestDotReader_SmallBuffer(t *testing.T) {
	// Read with a very small buffer to exercise partial reads.
	input := "..stuffed\r\nLine 2\r\n.\r\n"
	r := newDotReader(bufio.NewReader(strings.NewReader(input)))

	var result []byte
	buf := make([]byte, 3) // Intentionally small.
	for {
		n, err := r.Read(buf)
		result = append(result, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
	}

	want := ".stuffed\r\nLine 2\r\n"
	if string(result) != want {
		t.Errorf("got %q, want %q", result, want)
	}
}
