package smtp

import "fmt"

// EnhancedCode represents an enhanced mail system status code as defined in
// RFC 3463. Format is class.subject.detail (e.g., 2.1.0).
type EnhancedCode struct {
	Class   int // 2 = success, 4 = transient failure, 5 = permanent failure
	Subject int // Subject sub-code
	Detail  int // Detail sub-code
}

// Common enhanced status codes (RFC 3463, RFC 5248).
var (
	EnhancedCodeOK                = EnhancedCode{2, 0, 0} // Generic success
	EnhancedCodeOtherAddress      = EnhancedCode{2, 1, 0} // Other address status (success)
	EnhancedCodeDestValid         = EnhancedCode{2, 1, 5} // Destination address valid
	EnhancedCodeOtherMailbox      = EnhancedCode{2, 2, 0} // Other mailbox status (success)
	EnhancedCodeOtherMail         = EnhancedCode{2, 6, 0} // Other mail system status (success)

	EnhancedCodeBadDest           = EnhancedCode{5, 1, 1} // Bad destination mailbox address
	EnhancedCodeBadDestSystem     = EnhancedCode{5, 1, 2} // Bad destination system address
	EnhancedCodeBadDestSyntax     = EnhancedCode{5, 1, 3} // Bad destination mailbox address syntax
	EnhancedCodeAmbiguousDest     = EnhancedCode{5, 1, 6} // Destination mailbox moved (no forwarding)
	EnhancedCodeBadSenderSyntax   = EnhancedCode{5, 1, 7} // Bad sender's mailbox address syntax
	EnhancedCodeBadSenderSystem   = EnhancedCode{5, 1, 8} // Bad sender's system address

	EnhancedCodeMailboxFull       = EnhancedCode{5, 2, 2} // Mailbox full
	EnhancedCodeMsgTooLarge       = EnhancedCode{5, 3, 4} // Message too big for system

	EnhancedCodeOtherNetwork      = EnhancedCode{4, 4, 0} // Other network/routing status (transient)
	EnhancedCodeTempCongestion    = EnhancedCode{4, 4, 5} // System congestion (transient)

	EnhancedCodeInvalidCommand    = EnhancedCode{5, 5, 1} // Invalid command
	EnhancedCodeSyntaxError       = EnhancedCode{5, 5, 2} // Syntax error
	EnhancedCodeTooManyRecipients = EnhancedCode{5, 5, 3} // Too many recipients
	EnhancedCodeInvalidParams     = EnhancedCode{5, 5, 4} // Invalid command arguments

	EnhancedCodeTempAuthFailure   = EnhancedCode{4, 7, 0} // Other security/policy status (transient)
	EnhancedCodeAuthRequired      = EnhancedCode{5, 7, 0} // Other security/policy status (permanent)
	EnhancedCodeAuthCredentials   = EnhancedCode{5, 7, 8} // Authentication credentials invalid
	EnhancedCodeEncryptRequired   = EnhancedCode{5, 7, 11} // Encryption required
)

// String returns the enhanced code formatted as "X.Y.Z" (e.g., "2.1.0").
func (e EnhancedCode) String() string {
	return fmt.Sprintf("%d.%d.%d", e.Class, e.Subject, e.Detail)
}

// IsZero reports whether the enhanced code is the zero value.
func (e EnhancedCode) IsZero() bool {
	return e.Class == 0 && e.Subject == 0 && e.Detail == 0
}
