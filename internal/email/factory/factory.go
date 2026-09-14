// Package factory builds the currently-active email.Sender from stored
// settings. It's the one place that knows about the concrete provider
// (email/smtp), so the base email package itself can stay free of a
// dependency on it.
package factory

import (
	"fmt"

	"github.com/wkulhanek/points-tracker/internal/email"
	"github.com/wkulhanek/points-tracker/internal/email/smtp"
)

type Factory struct {
	settingsStore *email.SettingsStore
}

func New(settingsStore *email.SettingsStore) *Factory {
	return &Factory{settingsStore: settingsStore}
}

// Build returns a ready-to-use Sender for whichever provider is currently
// active, or email.ErrNoProvider if none is configured yet.
func (f *Factory) Build() (email.Sender, error) {
	st, err := f.settingsStore.Get()
	if err != nil {
		return nil, fmt.Errorf("load email settings: %w", err)
	}

	switch st.ProviderType {
	case email.ProviderSMTP:
		return smtp.NewSender(st), nil
	default:
		return nil, email.ErrNoProvider
	}
}
