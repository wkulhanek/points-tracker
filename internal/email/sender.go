package email

import (
	"context"
	"errors"
)

// ErrNoProvider is returned by a Sender factory when no email provider has
// been configured yet (both Settings-based UI flows and the scheduler
// treat this as "nothing to do / show a setup prompt", not a hard error).
var ErrNoProvider = errors.New("no email provider configured")

// Sender is implemented by both the Gmail and SMTP providers (see the
// gmail/ and smtp/ subpackages) so the notification scheduler and the
// "Test e-mail" button can send through whichever one is currently active
// without caring which it is.
type Sender interface {
	Send(ctx context.Context, to []string, subject, body string) error
}
