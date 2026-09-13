// Package email defines the provider-agnostic Sender interface used by
// both the notification scheduler and the "Test e-mail" button, plus the
// settings storage shared by the two concrete providers (Gmail OAuth and
// plain SMTP) in the gmail/ and smtp/ subpackages.
package email

import "time"

type ProviderType string

const (
	ProviderNone  ProviderType = "none"
	ProviderGmail ProviderType = "gmail"
	ProviderSMTP  ProviderType = "smtp"
)

type SMTPSecurity string

const (
	SMTPSecurityNone     SMTPSecurity = "none"
	SMTPSecurityStartTLS SMTPSecurity = "starttls"
	SMTPSecurityTLS      SMTPSecurity = "tls"
)

// Settings is the singleton row describing whichever email provider is
// currently configured. Only the fields for the active ProviderType are
// meaningful; the other provider's fields are left as their zero value.
type Settings struct {
	ProviderType ProviderType

	GmailRefreshToken string
	GmailAccessToken  string
	GmailTokenExpiry  time.Time
	GmailAccountEmail string

	SMTPHost        string
	SMTPPort        int
	SMTPUsername    string
	SMTPPassword    string
	SMTPSecurity    SMTPSecurity
	SMTPFromAddress string
}
