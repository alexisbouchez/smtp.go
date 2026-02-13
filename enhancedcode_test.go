package smtp

import "testing"

func TestEnhancedCode_String(t *testing.T) {
	tests := []struct {
		code EnhancedCode
		want string
	}{
		{EnhancedCodeOK, "2.0.0"},
		{EnhancedCodeDestValid, "2.1.5"},
		{EnhancedCodeBadDest, "5.1.1"},
		{EnhancedCodeMsgTooLarge, "5.3.4"},
		{EnhancedCodeInvalidCommand, "5.5.1"},
		{EnhancedCodeEncryptRequired, "5.7.11"},
		{EnhancedCode{4, 4, 5}, "4.4.5"},
	}
	for _, tt := range tests {
		if got := tt.code.String(); got != tt.want {
			t.Errorf("EnhancedCode%v.String() = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestEnhancedCode_IsZero(t *testing.T) {
	if !((EnhancedCode{}).IsZero()) {
		t.Error("zero EnhancedCode should be zero")
	}
	if EnhancedCodeOK.IsZero() {
		t.Error("EnhancedCodeOK should not be zero")
	}
}
