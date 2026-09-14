// Package smtp implements the email.Sender interface as a plain SMTP
// client, for people not using Gmail.
package smtp

import (
	"context"
	"fmt"

	gomail "github.com/wneessen/go-mail"

	"github.com/wkulhanek/points-tracker/internal/email"
)

// Sender implements email.Sender by delivering through a configured SMTP
// server.
type Sender struct {
	settings email.Settings
}

func NewSender(settings email.Settings) *Sender {
	return &Sender{settings: settings}
}

func (s *Sender) Send(ctx context.Context, to []string, subject, body string) error {
	if s.settings.SMTPHost == "" {
		return email.ErrNoProvider
	}

	m := gomail.NewMsg()
	if err := m.From(s.settings.SMTPFromAddress); err != nil {
		return fmt.Errorf("set from address: %w", err)
	}
	if err := m.To(to...); err != nil {
		return fmt.Errorf("set to address(es): %w", err)
	}
	m.Subject(subject)
	m.SetBodyString(gomail.TypeTextPlain, body)

	opts := []gomail.Option{
		gomail.WithPort(s.settings.SMTPPort),
	}
	if s.settings.SMTPUsername != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(s.settings.SMTPUsername),
			gomail.WithPassword(s.settings.SMTPPassword),
		)
	}

	switch s.settings.SMTPSecurity {
	case email.SMTPSecurityTLS:
		opts = append(opts, gomail.WithSSL())
	case email.SMTPSecurityStartTLS:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	default:
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	}

	client, err := gomail.NewClient(s.settings.SMTPHost, opts...)
	if err != nil {
		return fmt.Errorf("build smtp client: %w", err)
	}

	if err := client.DialAndSendWithContext(ctx, m); err != nil {
		return fmt.Errorf("send via smtp: %w", err)
	}
	return nil
}
