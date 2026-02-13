package textproto

import (
	"bufio"
	"io"
)

// dotReader reads a dot-stuffed message body from an SMTP DATA stream.
// It transparently destuffs lines starting with ".." and terminates
// at the line ".\r\n" (RFC 5321 §4.5.2).
type dotReader struct {
	r     *bufio.Reader
	state int
}

const (
	dotStateBeginLine = iota // At the beginning of a line.
	dotStateLine             // In the middle of a line.
	dotStateCR               // Just saw \r.
	dotStateDot              // Saw dot at beginning of line.
	dotStateDotCR            // Saw dot then \r at beginning of line.
	dotStateEOF              // Reached termination.
)

func newDotReader(r *bufio.Reader) *dotReader {
	return &dotReader{r: r, state: dotStateBeginLine}
}

func (d *dotReader) Read(p []byte) (int, error) {
	if d.state == dotStateEOF {
		return 0, io.EOF
	}

	n := 0
	for n < len(p) {
		b, err := d.r.ReadByte()
		if err != nil {
			d.state = dotStateEOF
			if n > 0 {
				return n, nil
			}
			return 0, err
		}

		switch d.state {
		case dotStateBeginLine:
			if b == '.' {
				d.state = dotStateDot
				continue
			}
			if b == '\r' {
				d.state = dotStateCR
				p[n] = b
				n++
			} else if b == '\n' {
				// Bare LF — still a line ending, stay at begin-line.
				d.state = dotStateBeginLine
				p[n] = b
				n++
			} else {
				d.state = dotStateLine
				p[n] = b
				n++
			}

		case dotStateLine:
			if b == '\r' {
				d.state = dotStateCR
			} else if b == '\n' {
				// Bare LF — treat as line ending for robustness.
				d.state = dotStateBeginLine
			}
			p[n] = b
			n++

		case dotStateCR:
			if b == '\n' {
				d.state = dotStateBeginLine
			} else if b == '\r' {
				// Stay in CR state.
			} else {
				d.state = dotStateLine
			}
			p[n] = b
			n++

		case dotStateDot:
			if b == '\r' {
				// ".\r" — could be end marker.
				d.state = dotStateDotCR
				continue
			}
			if b == '\n' {
				// ".\n" — bare LF termination (be lenient).
				d.state = dotStateEOF
				return n, io.EOF
			}
			if b == '.' {
				// ".." → destuff to single ".".
				d.state = dotStateLine
				p[n] = '.'
				n++
			} else {
				// Dot followed by other char — not stuffing, emit the dot.
				d.state = dotStateLine
				p[n] = '.'
				n++
				if n < len(p) {
					p[n] = b
					n++
				} else {
					d.r.UnreadByte()
				}
			}

		case dotStateDotCR:
			if b == '\n' {
				// ".\r\n" — end of message.
				d.state = dotStateEOF
				return n, io.EOF
			}
			// False alarm — emit ".\r" and continue.
			d.state = dotStateLine
			if n+2 <= len(p) {
				p[n] = '.'
				n++
				p[n] = '\r'
				n++
				if n < len(p) {
					p[n] = b
					n++
				} else {
					d.r.UnreadByte()
				}
			} else {
				d.r.UnreadByte()
				p[n] = '.'
				n++
				return n, nil
			}
		}
	}
	return n, nil
}

// dotWriter writes a dot-stuffed message body to an SMTP DATA stream.
// Lines starting with "." are doubled to ".." and Close() writes the
// termination sequence ".\r\n" (RFC 5321 §4.5.2).
type dotWriter struct {
	w         *bufio.Writer
	beginLine bool
	closed    bool
}

func newDotWriter(w *bufio.Writer) *dotWriter {
	return &dotWriter{w: w, beginLine: true}
}

func (d *dotWriter) Write(p []byte) (int, error) {
	if d.closed {
		return 0, io.ErrClosedPipe
	}

	written := 0
	for _, b := range p {
		if d.beginLine && b == '.' {
			// Dot-stuff: add extra dot.
			if err := d.w.WriteByte('.'); err != nil {
				return written, err
			}
		}

		if err := d.w.WriteByte(b); err != nil {
			return written, err
		}
		written++

		d.beginLine = (b == '\n')
	}
	return written, nil
}

// Close writes the termination sequence and flushes the writer.
// If the last data written did not end with \r\n, Close adds \r\n first.
func (d *dotWriter) Close() error {
	if d.closed {
		return nil
	}
	d.closed = true

	// If we're not at the beginning of a line, we need to add \r\n first.
	if !d.beginLine {
		if _, err := d.w.WriteString("\r\n"); err != nil {
			return err
		}
	}
	if _, err := d.w.WriteString(".\r\n"); err != nil {
		return err
	}
	return d.w.Flush()
}
