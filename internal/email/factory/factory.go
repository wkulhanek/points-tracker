// Package factory builds the currently-active email.Sender from stored
// settings. It's the one place that knows about both concrete providers
// (email/gmail and email/smtp), so the base email package itself can stay
// free of a dependency on either.
package factory

import (
	"fmt"

	"github.com/wkulhanek/kulhanek-points-tracker/internal/email"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/email/gmail"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/email/smtp"
)

type Factory struct {
	settingsStore *email.SettingsStore
	gmailOAuth    *gmail.OAuth // nil if GOOGLE_CLIENT_ID/SECRET aren't configured
}

func New(settingsStore *email.SettingsStore, gmailOAuth *gmail.OAuth) *Factory {
	return &Factory{settingsStore: settingsStore, gmailOAuth: gmailOAuth}
}

// Build returns a ready-to-use Sender for whichever provider is currently
// active, or email.ErrNoProvider if none is configured yet.
func (f *Factory) Build() (email.Sender, error) {
	st, err := f.settingsStore.Get()
	if err != nil {
		return nil, fmt.Errorf("load email settings: %w", err)
	}

	switch st.ProviderType {
	case email.ProviderGmail:
		if f.gmailOAuth == nil {
			return nil, fmt.Errorf("gmail was connected but GOOGLE_CLIENT_ID/GOOGLE_CLIENT_SECRET are not configured")
		}
		return gmail.NewSender(f.gmailOAuth, f.settingsStore), nil
	case email.ProviderSMTP:
		return smtp.NewSender(st), nil
	default:
		return nil, email.ErrNoProvider
	}
}
