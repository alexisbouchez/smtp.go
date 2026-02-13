package smtp

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Mailbox represents an email address as local-part@domain (RFC 5321 §4.1.2).
type Mailbox struct {
	LocalPart string
	Domain    string
}

// String returns the mailbox formatted as "local-part@domain".
func (m Mailbox) String() string {
	if m.LocalPart == "" && m.Domain == "" {
		return ""
	}
	return m.LocalPart + "@" + m.Domain
}

// IsZero reports whether the mailbox is empty.
func (m Mailbox) IsZero() bool {
	return m.LocalPart == "" && m.Domain == ""
}

// ReversePath represents the MAIL FROM path (RFC 5321 §4.1.1.2).
// A zero-value ReversePath represents the null reverse-path (<>) used for bounces.
type ReversePath struct {
	Mailbox Mailbox
	Null    bool // True for the null reverse-path <>
}

// String returns the path formatted for the wire protocol (e.g., "<user@domain>" or "<>").
func (rp ReversePath) String() string {
	if rp.Null {
		return "<>"
	}
	return "<" + rp.Mailbox.String() + ">"
}

// ForwardPath represents the RCPT TO path (RFC 5321 §4.1.1.3).
type ForwardPath struct {
	Mailbox Mailbox
}

// String returns the path formatted for the wire protocol (e.g., "<user@domain>").
func (fp ForwardPath) String() string {
	return "<" + fp.Mailbox.String() + ">"
}

// ParseMailbox parses an email address string into a Mailbox.
// It expects the format "local-part@domain" (no angle brackets).
func ParseMailbox(s string) (Mailbox, error) {
	if s == "" {
		return Mailbox{}, errors.New("smtp: empty address")
	}

	// Find the last @ — local-part may contain quoted @, but for simplicity
	// we split on the last unquoted @.
	at := strings.LastIndexByte(s, '@')
	if at < 0 {
		return Mailbox{}, errors.New("smtp: missing @ in address")
	}
	if at == 0 {
		return Mailbox{}, errors.New("smtp: empty local-part")
	}
	if at == len(s)-1 {
		return Mailbox{}, errors.New("smtp: empty domain")
	}

	local := s[:at]
	domain := s[at+1:]

	if err := validateLocalPart(local); err != nil {
		return Mailbox{}, err
	}
	if err := validateDomain(domain); err != nil {
		return Mailbox{}, err
	}

	return Mailbox{LocalPart: local, Domain: domain}, nil
}

// ParseReversePath parses a MAIL FROM path string.
// It accepts "<>" (null reverse-path) or "<local@domain>" or "local@domain".
func ParseReversePath(s string) (ReversePath, error) {
	s = strings.TrimSpace(s)

	if s == "<>" {
		return ReversePath{Null: true}, nil
	}

	// Strip angle brackets if present.
	inner := s
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		inner = s[1 : len(s)-1]
	}

	if inner == "" {
		return ReversePath{Null: true}, nil
	}

	m, err := ParseMailbox(inner)
	if err != nil {
		return ReversePath{}, err
	}
	return ReversePath{Mailbox: m}, nil
}

// ParseForwardPath parses a RCPT TO path string.
// It accepts "<local@domain>" or "local@domain".
func ParseForwardPath(s string) (ForwardPath, error) {
	s = strings.TrimSpace(s)

	inner := s
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		inner = s[1 : len(s)-1]
	}

	if inner == "" {
		return ForwardPath{}, errors.New("smtp: empty forward path")
	}

	m, err := ParseMailbox(inner)
	if err != nil {
		return ForwardPath{}, err
	}
	return ForwardPath{Mailbox: m}, nil
}

// validateLocalPart checks the local-part per RFC 5321 §4.1.2.
// Accepts dot-atom and quoted-string forms.
func validateLocalPart(local string) error {
	if local == "" {
		return errors.New("smtp: empty local-part")
	}
	if len(local) > 64 { // RFC 5321 §4.5.3.1.1
		return errors.New("smtp: local-part too long")
	}

	// Quoted-string form: starts and ends with DQUOTE.
	if len(local) >= 2 && local[0] == '"' && local[len(local)-1] == '"' {
		return validateQuotedLocalPart(local[1 : len(local)-1])
	}

	return validateDotAtom(local)
}

func validateDotAtom(s string) error {
	if s == "" {
		return errors.New("smtp: empty dot-atom")
	}
	if s[0] == '.' || s[len(s)-1] == '.' {
		return errors.New("smtp: dot-atom cannot start or end with a dot")
	}
	if strings.Contains(s, "..") {
		return errors.New("smtp: dot-atom cannot contain consecutive dots")
	}
	for _, r := range s {
		if !isDotAtomChar(r) {
			return errors.New("smtp: invalid character in local-part")
		}
	}
	return nil
}

func isDotAtomChar(r rune) bool {
	if r == '.' {
		return true
	}
	return isAtext(r)
}

// isAtext checks for RFC 5321 atext characters.
func isAtext(r rune) bool {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
		return true
	}
	switch r {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '/', '=', '?', '^', '_', '`', '{', '|', '}', '~':
		return true
	}
	return false
}

func validateQuotedLocalPart(s string) error {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' {
			i++ // Skip escaped character.
			if i >= len(s) {
				return errors.New("smtp: trailing backslash in quoted local-part")
			}
			continue
		}
		if c == '"' {
			return errors.New("smtp: unescaped quote in quoted local-part")
		}
	}
	return nil
}

// validateDomain checks the domain per RFC 5321 §4.1.2.
// Accepts DNS hostnames and IPv4/IPv6 address literals ([...]).
func validateDomain(domain string) error {
	if domain == "" {
		return errors.New("smtp: empty domain")
	}
	if len(domain) > 255 { // RFC 5321 §4.5.3.1.2
		return errors.New("smtp: domain too long")
	}

	// Address literal: [IPv4] or [IPv6:...]
	if domain[0] == '[' {
		if domain[len(domain)-1] != ']' {
			return errors.New("smtp: unclosed address literal")
		}
		return nil // Accept address literals without deep validation for now.
	}

	// DNS hostname validation.
	if domain[0] == '.' || domain[len(domain)-1] == '.' {
		return errors.New("smtp: domain cannot start or end with a dot")
	}

	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if label == "" {
			return errors.New("smtp: empty label in domain")
		}
		if len(label) > 63 {
			return errors.New("smtp: domain label too long")
		}
		if !utf8.ValidString(label) {
			return errors.New("smtp: invalid UTF-8 in domain label")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("smtp: domain label cannot start or end with hyphen")
		}
		for _, r := range label {
			if !isDomainChar(r) {
				return errors.New("smtp: invalid character in domain")
			}
		}
	}
	return nil
}

func isDomainChar(r rune) bool {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
		return true
	}
	if r == '-' {
		return true
	}
	// Allow UTF-8 for internationalized domains (RFC 6531).
	if r > 127 {
		return true
	}
	return false
}
