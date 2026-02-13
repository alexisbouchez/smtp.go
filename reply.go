package smtp

// ReplyCode represents a three-digit SMTP reply code as defined in RFC 5321 §4.2.
type ReplyCode int

// Reply code classes (RFC 5321 §4.2.1).
const (
	ClassPositiveCompletion  = 2 // 2xx
	ClassPositiveIntermediate = 3 // 3xx
	ClassTransientNegative   = 4 // 4xx
	ClassPermanentNegative   = 5 // 5xx
)

// Standard SMTP reply codes (RFC 5321 §4.2.2, §4.2.3).
const (
	// 2xx — Positive completion.
	ReplySystemStatus       ReplyCode = 211
	ReplyHelpMessage        ReplyCode = 214
	ReplyServiceReady       ReplyCode = 220
	ReplyServiceClosing     ReplyCode = 221
	ReplyAuthOK             ReplyCode = 235
	ReplyOK                 ReplyCode = 250
	ReplyUserNotLocal       ReplyCode = 251
	ReplyCannotVRFY         ReplyCode = 252

	// 3xx — Positive intermediate.
	ReplyAuthContinue       ReplyCode = 334
	ReplyStartMailInput     ReplyCode = 354

	// 4xx — Transient negative completion.
	ReplyServiceNotAvailable ReplyCode = 421
	ReplyMailboxBusy         ReplyCode = 450
	ReplyLocalError          ReplyCode = 451
	ReplyInsufficientStorage ReplyCode = 452
	ReplyTempAuthFailure     ReplyCode = 454

	// 5xx — Permanent negative completion.
	ReplySyntaxError         ReplyCode = 500
	ReplySyntaxParamError    ReplyCode = 501
	ReplyCommandNotImpl      ReplyCode = 502
	ReplyBadSequence         ReplyCode = 503
	ReplyParamNotImpl        ReplyCode = 504
	ReplyAuthRequired        ReplyCode = 530
	ReplyAuthFailed          ReplyCode = 535
	ReplyMailboxNotFound     ReplyCode = 550
	ReplyUserNotLocalTry     ReplyCode = 551
	ReplyExceededStorage     ReplyCode = 552
	ReplyMailboxNameError    ReplyCode = 553
	ReplyTransactionFailed   ReplyCode = 554
	ReplyMailRcptParamError  ReplyCode = 555
)

// Class returns the reply class (first digit): 2, 3, 4, or 5.
func (c ReplyCode) Class() int {
	return int(c) / 100
}

// IsPositive returns true for 2xx and 3xx reply codes.
func (c ReplyCode) IsPositive() bool {
	cl := c.Class()
	return cl == ClassPositiveCompletion || cl == ClassPositiveIntermediate
}

// IsTransient returns true for 4xx reply codes (temporary failures).
func (c ReplyCode) IsTransient() bool {
	return c.Class() == ClassTransientNegative
}

// IsPermanent returns true for 5xx reply codes (permanent failures).
func (c ReplyCode) IsPermanent() bool {
	return c.Class() == ClassPermanentNegative
}
