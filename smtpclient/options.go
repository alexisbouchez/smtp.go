package smtpclient

// MailOption configures the MAIL FROM command.
type MailOption func(*mailOptions)

type mailOptions struct {
	size     int64
	body     string // "7BIT" or "8BITMIME"
	smtpUTF8 bool
	dsnRet   string // "FULL" or "HDRS"
	dsnEnvID string
}

// WithSize sets the SIZE parameter (RFC 1870).
func WithSize(n int64) MailOption {
	return func(o *mailOptions) { o.size = n }
}

// WithBody sets the BODY parameter (RFC 6152). Use "8BITMIME" or "7BIT".
func WithBody(body string) MailOption {
	return func(o *mailOptions) { o.body = body }
}

// WithSMTPUTF8 sets the SMTPUTF8 parameter (RFC 6531).
func WithSMTPUTF8() MailOption {
	return func(o *mailOptions) { o.smtpUTF8 = true }
}

// WithDSNReturn sets the RET parameter for DSN (RFC 3461). Use "FULL" or "HDRS".
func WithDSNReturn(ret string) MailOption {
	return func(o *mailOptions) { o.dsnRet = ret }
}

// WithDSNEnvelopeID sets the ENVID parameter for DSN (RFC 3461).
func WithDSNEnvelopeID(envid string) MailOption {
	return func(o *mailOptions) { o.dsnEnvID = envid }
}

// RcptOption configures the RCPT TO command.
type RcptOption func(*rcptOptions)

type rcptOptions struct {
	dsnNotify string // e.g., "SUCCESS,FAILURE,DELAY" or "NEVER"
	dsnOrcpt  string // Original recipient, e.g., "rfc822;user@example.com"
}

// WithDSNNotify sets the NOTIFY parameter for DSN (RFC 3461).
func WithDSNNotify(notify string) RcptOption {
	return func(o *rcptOptions) { o.dsnNotify = notify }
}

// WithDSNOriginalRecipient sets the ORCPT parameter for DSN (RFC 3461).
func WithDSNOriginalRecipient(orcpt string) RcptOption {
	return func(o *rcptOptions) { o.dsnOrcpt = orcpt }
}
