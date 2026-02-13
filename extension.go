package smtp

import "strings"

// Extension represents an SMTP service extension keyword (RFC 5321 §2.2).
type Extension string

// Standard SMTP extension keywords.
const (
	ExtSTARTTLS           Extension = "STARTTLS"
	ExtAUTH               Extension = "AUTH"
	ExtSIZE               Extension = "SIZE"
	ExtPIPELINING         Extension = "PIPELINING"
	Ext8BITMIME           Extension = "8BITMIME"
	ExtDSN                Extension = "DSN"
	ExtENHANCEDSTATUSCODES Extension = "ENHANCEDSTATUSCODES"
	ExtSMTPUTF8           Extension = "SMTPUTF8"
	ExtCHUNKING           Extension = "CHUNKING"
)

// Extensions holds the set of SMTP extensions advertised in an EHLO response,
// mapped from keyword to parameters (e.g., "AUTH" → "PLAIN LOGIN").
type Extensions map[Extension]string

// Has reports whether the extension set includes the given keyword.
func (e Extensions) Has(ext Extension) bool {
	_, ok := e[ext]
	return ok
}

// Param returns the parameter string for the given extension keyword.
func (e Extensions) Param(ext Extension) string {
	return e[ext]
}

// ParseEHLOResponse parses the lines of a multi-line 250 EHLO response into
// an Extensions map. Each line after the first (the greeting) is expected to
// be "KEYWORD [params]".
func ParseEHLOResponse(lines []string) Extensions {
	exts := make(Extensions)
	for i, line := range lines {
		if i == 0 {
			continue // Skip the greeting line (hostname).
		}
		keyword, params, _ := strings.Cut(line, " ")
		exts[Extension(strings.ToUpper(keyword))] = params
	}
	return exts
}
