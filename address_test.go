package smtp

import "testing"

func TestParseMailbox(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Mailbox
		wantErr bool
	}{
		{name: "simple", input: "user@example.com", want: Mailbox{"user", "example.com"}},
		{name: "dots in local", input: "first.last@example.com", want: Mailbox{"first.last", "example.com"}},
		{name: "subdomain", input: "user@mail.example.com", want: Mailbox{"user", "mail.example.com"}},
		{name: "plus tag", input: "user+tag@example.com", want: Mailbox{"user+tag", "example.com"}},
		{name: "quoted local", input: `"user@host"@example.com`, want: Mailbox{`"user@host"`, "example.com"}},
		{name: "ip literal domain", input: "user@[192.168.1.1]", want: Mailbox{"user", "[192.168.1.1]"}},
		{name: "empty", input: "", wantErr: true},
		{name: "no at", input: "userexample.com", wantErr: true},
		{name: "empty local", input: "@example.com", wantErr: true},
		{name: "empty domain", input: "user@", wantErr: true},
		{name: "leading dot in local", input: ".user@example.com", wantErr: true},
		{name: "trailing dot in local", input: "user.@example.com", wantErr: true},
		{name: "consecutive dots", input: "user..name@example.com", wantErr: true},
		{name: "local too long", input: string(make([]byte, 65)) + "@example.com", wantErr: true},
		{name: "domain leading dot", input: "user@.example.com", wantErr: true},
		{name: "domain trailing dot", input: "user@example.com.", wantErr: true},
		{name: "domain label leading hyphen", input: "user@-example.com", wantErr: true},
		{name: "domain label trailing hyphen", input: "user@example-.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMailbox(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMailbox(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseMailbox(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseReversePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNull bool
		wantAddr string
		wantErr  bool
	}{
		{name: "null path", input: "<>", wantNull: true},
		{name: "empty string in brackets", input: "<>", wantNull: true},
		{name: "normal path", input: "<user@example.com>", wantAddr: "user@example.com"},
		{name: "without brackets", input: "user@example.com", wantAddr: "user@example.com"},
		{name: "invalid address", input: "<invalid>", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReversePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReversePath(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Null != tt.wantNull {
				t.Errorf("Null = %v, want %v", got.Null, tt.wantNull)
			}
			if !tt.wantNull && got.Mailbox.String() != tt.wantAddr {
				t.Errorf("Mailbox = %q, want %q", got.Mailbox.String(), tt.wantAddr)
			}
		})
	}
}

func TestParseForwardPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "with brackets", input: "<user@example.com>", want: "user@example.com"},
		{name: "without brackets", input: "user@example.com", want: "user@example.com"},
		{name: "empty brackets", input: "<>", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseForwardPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseForwardPath(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Mailbox.String() != tt.want {
				t.Errorf("Mailbox = %q, want %q", got.Mailbox.String(), tt.want)
			}
		})
	}
}

func TestMailbox_String(t *testing.T) {
	if got := (Mailbox{"user", "example.com"}).String(); got != "user@example.com" {
		t.Errorf("String() = %q, want %q", got, "user@example.com")
	}
	if got := (Mailbox{}).String(); got != "" {
		t.Errorf("zero Mailbox.String() = %q, want empty", got)
	}
}

func TestReversePath_String(t *testing.T) {
	if got := (ReversePath{Null: true}).String(); got != "<>" {
		t.Errorf("null ReversePath.String() = %q, want \"<>\"", got)
	}
	rp := ReversePath{Mailbox: Mailbox{"user", "example.com"}}
	if got := rp.String(); got != "<user@example.com>" {
		t.Errorf("ReversePath.String() = %q, want \"<user@example.com>\"", got)
	}
}

func TestForwardPath_String(t *testing.T) {
	fp := ForwardPath{Mailbox: Mailbox{"user", "example.com"}}
	if got := fp.String(); got != "<user@example.com>" {
		t.Errorf("ForwardPath.String() = %q, want \"<user@example.com>\"", got)
	}
}
