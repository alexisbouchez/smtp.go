package smtp

import "testing"

func TestSMTPError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *SMTPError
		want string
	}{
		{
			name: "with enhanced code",
			err:  &SMTPError{Code: ReplyMailboxNotFound, EnhancedCode: EnhancedCodeBadDest, Message: "User unknown"},
			want: "smtp: 550 5.1.1 User unknown",
		},
		{
			name: "without enhanced code",
			err:  &SMTPError{Code: ReplySyntaxError, Message: "Syntax error"},
			want: "smtp: 500 Syntax error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSMTPError_Temporary(t *testing.T) {
	if !(&SMTPError{Code: ReplyMailboxBusy}).Temporary() {
		t.Error("450 should be temporary")
	}
	if (&SMTPError{Code: ReplyMailboxNotFound}).Temporary() {
		t.Error("550 should not be temporary")
	}
}

func TestSMTPError_WireLines(t *testing.T) {
	tests := []struct {
		name string
		err  *SMTPError
		want string
	}{
		{
			name: "single line with enhanced code",
			err:  &SMTPError{Code: ReplyOK, EnhancedCode: EnhancedCodeOK, Message: "OK"},
			want: "250 2.0.0 OK\r\n",
		},
		{
			name: "single line without enhanced code",
			err:  &SMTPError{Code: ReplySyntaxError, Message: "Command unrecognized"},
			want: "500 Command unrecognized\r\n",
		},
		{
			name: "multi-line",
			err:  &SMTPError{Code: ReplyOK, EnhancedCode: EnhancedCodeOK, Message: "Line 1\nLine 2\nLine 3"},
			want: "250-2.0.0 Line 1\r\n250-2.0.0 Line 2\r\n250 2.0.0 Line 3\r\n",
		},
		{
			name: "empty message",
			err:  &SMTPError{Code: ReplyOK, EnhancedCode: EnhancedCodeOK},
			want: "250 2.0.0 Error\r\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.WireLines(); got != tt.want {
				t.Errorf("WireLines() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorf(t *testing.T) {
	err := Errorf(ReplyMailboxNotFound, EnhancedCodeBadDest, "user %s not found", "bob")
	if err.Code != ReplyMailboxNotFound {
		t.Errorf("Code = %d, want %d", err.Code, ReplyMailboxNotFound)
	}
	if err.EnhancedCode != EnhancedCodeBadDest {
		t.Errorf("EnhancedCode = %v, want %v", err.EnhancedCode, EnhancedCodeBadDest)
	}
	if err.Message != "user bob not found" {
		t.Errorf("Message = %q, want %q", err.Message, "user bob not found")
	}
}
