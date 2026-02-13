package smtp

import (
	"fmt"
	"strings"
)

// SMTPError represents an SMTP protocol error with a reply code,
// optional enhanced status code, and human-readable message.
type SMTPError struct {
	Code         ReplyCode
	EnhancedCode EnhancedCode
	Message      string
}

// Error implements the error interface.
func (e *SMTPError) Error() string {
	if !e.EnhancedCode.IsZero() {
		return fmt.Sprintf("smtp: %d %s %s", e.Code, e.EnhancedCode, e.Message)
	}
	return fmt.Sprintf("smtp: %d %s", e.Code, e.Message)
}

// Temporary reports whether the error represents a transient failure (4xx).
func (e *SMTPError) Temporary() bool {
	return e.Code.IsTransient()
}

// WireLines returns the error formatted as SMTP wire-protocol reply lines.
// Multi-line messages (containing newlines) are formatted with continuation
// lines using the "code-SP" / "code-hyphen" convention (RFC 5321 §4.2).
func (e *SMTPError) WireLines() string {
	msg := e.Message
	if msg == "" {
		msg = "Error"
	}

	lines := strings.Split(msg, "\n")
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%d", e.Code)
		if i < len(lines)-1 {
			b.WriteByte('-')
		} else {
			b.WriteByte(' ')
		}
		if !e.EnhancedCode.IsZero() {
			fmt.Fprintf(&b, "%s ", e.EnhancedCode)
		}
		b.WriteString(line)
		b.WriteString("\r\n")
	}
	return b.String()
}

// Errorf creates an SMTPError with a formatted message.
func Errorf(code ReplyCode, enhanced EnhancedCode, format string, args ...any) *SMTPError {
	return &SMTPError{
		Code:         code,
		EnhancedCode: enhanced,
		Message:      fmt.Sprintf(format, args...),
	}
}
