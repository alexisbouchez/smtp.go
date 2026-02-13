package smtp

import "testing"

func TestParseEHLOResponse(t *testing.T) {
	lines := []string{
		"mail.example.com Hello",
		"SIZE 52428800",
		"PIPELINING",
		"AUTH PLAIN LOGIN CRAM-MD5",
		"STARTTLS",
		"8BITMIME",
		"ENHANCEDSTATUSCODES",
		"DSN",
		"SMTPUTF8",
		"CHUNKING",
	}

	exts := ParseEHLOResponse(lines)

	if !exts.Has(ExtSIZE) {
		t.Error("expected SIZE extension")
	}
	if exts.Param(ExtSIZE) != "52428800" {
		t.Errorf("SIZE param = %q, want %q", exts.Param(ExtSIZE), "52428800")
	}

	if !exts.Has(ExtPIPELINING) {
		t.Error("expected PIPELINING extension")
	}
	if exts.Param(ExtPIPELINING) != "" {
		t.Errorf("PIPELINING param = %q, want empty", exts.Param(ExtPIPELINING))
	}

	if !exts.Has(ExtAUTH) {
		t.Error("expected AUTH extension")
	}
	if exts.Param(ExtAUTH) != "PLAIN LOGIN CRAM-MD5" {
		t.Errorf("AUTH param = %q, want %q", exts.Param(ExtAUTH), "PLAIN LOGIN CRAM-MD5")
	}

	for _, ext := range []Extension{ExtSTARTTLS, Ext8BITMIME, ExtENHANCEDSTATUSCODES, ExtDSN, ExtSMTPUTF8, ExtCHUNKING} {
		if !exts.Has(ext) {
			t.Errorf("expected %s extension", ext)
		}
	}
}

func TestParseEHLOResponse_CaseInsensitive(t *testing.T) {
	lines := []string{
		"hostname",
		"size 1000",
		"Pipelining",
		"starttls",
	}
	exts := ParseEHLOResponse(lines)

	if !exts.Has(ExtSIZE) {
		t.Error("expected SIZE (case-insensitive)")
	}
	if !exts.Has(ExtPIPELINING) {
		t.Error("expected PIPELINING (case-insensitive)")
	}
	if !exts.Has(ExtSTARTTLS) {
		t.Error("expected STARTTLS (case-insensitive)")
	}
}

func TestExtensions_Has_Missing(t *testing.T) {
	exts := Extensions{}
	if exts.Has(ExtSTARTTLS) {
		t.Error("empty Extensions should not have STARTTLS")
	}
}
