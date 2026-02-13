package smtp

import "testing"

func TestReplyCode_Class(t *testing.T) {
	tests := []struct {
		code ReplyCode
		want int
	}{
		{ReplyOK, 2},
		{ReplyStartMailInput, 3},
		{ReplyMailboxBusy, 4},
		{ReplySyntaxError, 5},
		{ReplyServiceReady, 2},
		{ReplyServiceNotAvailable, 4},
		{ReplyTransactionFailed, 5},
	}
	for _, tt := range tests {
		if got := tt.code.Class(); got != tt.want {
			t.Errorf("ReplyCode(%d).Class() = %d, want %d", tt.code, got, tt.want)
		}
	}
}

func TestReplyCode_IsPositive(t *testing.T) {
	tests := []struct {
		code ReplyCode
		want bool
	}{
		{ReplyOK, true},
		{ReplyStartMailInput, true},
		{ReplyMailboxBusy, false},
		{ReplySyntaxError, false},
	}
	for _, tt := range tests {
		if got := tt.code.IsPositive(); got != tt.want {
			t.Errorf("ReplyCode(%d).IsPositive() = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestReplyCode_IsTransient(t *testing.T) {
	tests := []struct {
		code ReplyCode
		want bool
	}{
		{ReplyOK, false},
		{ReplyMailboxBusy, true},
		{ReplyLocalError, true},
		{ReplySyntaxError, false},
	}
	for _, tt := range tests {
		if got := tt.code.IsTransient(); got != tt.want {
			t.Errorf("ReplyCode(%d).IsTransient() = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestReplyCode_IsPermanent(t *testing.T) {
	tests := []struct {
		code ReplyCode
		want bool
	}{
		{ReplyOK, false},
		{ReplyMailboxBusy, false},
		{ReplySyntaxError, true},
		{ReplyMailboxNotFound, true},
	}
	for _, tt := range tests {
		if got := tt.code.IsPermanent(); got != tt.want {
			t.Errorf("ReplyCode(%d).IsPermanent() = %v, want %v", tt.code, got, tt.want)
		}
	}
}
