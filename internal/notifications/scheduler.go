package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wkulhanek/points-tracker/internal/accounts"
	"github.com/wkulhanek/points-tracker/internal/email"
	"github.com/wkulhanek/points-tracker/internal/preferences"
)

// SenderFactory builds the currently active email.Sender. Implemented by
// internal/email/factory.Factory; defined here as a narrow interface so
// this package doesn't need to depend on the concrete Gmail/SMTP providers.
type SenderFactory interface {
	Build() (email.Sender, error)
}

// tickInterval is deliberately hourly rather than once a day: an idempotent
// check (Evaluate never re-notifies an already-sent pair) is cheap to run
// often, and doing so makes the scheduler self-correct quickly after a
// restart or a timezone/clock change instead of possibly missing a single
// daily window.
const tickInterval = time.Hour

type Scheduler struct {
	accounts    *accounts.Service
	preferences *preferences.Store
	store       *Store
	senders     SenderFactory
}

func NewScheduler(accountsSvc *accounts.Service, prefsStore *preferences.Store, store *Store, senders SenderFactory) *Scheduler {
	return &Scheduler{accounts: accountsSvc, preferences: prefsStore, store: store, senders: senders}
}

// Run blocks, checking immediately and then every tickInterval, until ctx
// is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.checkOnce(ctx)

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkOnce(ctx)
		}
	}
}

func (s *Scheduler) checkOnce(ctx context.Context) {
	prefs, err := s.preferences.Get()
	if err != nil {
		slog.Error("notifications: load preferences", "error", err)
		return
	}

	now := time.Now().In(resolveLocation(prefs.Timezone))

	accts, err := s.accounts.List()
	if err != nil {
		slog.Error("notifications: load accounts", "error", err)
		return
	}
	thresholds, err := s.preferences.Thresholds()
	if err != nil {
		slog.Error("notifications: load thresholds", "error", err)
		return
	}
	sent, err := s.store.All()
	if err != nil {
		slog.Error("notifications: load sent notifications", "error", err)
		return
	}

	pending := Evaluate(now, accts, thresholds, sent)
	if len(pending) == 0 {
		return
	}

	if len(prefs.RecipientEmails) == 0 {
		slog.Warn("notifications: accounts are due for notification but no recipient email is configured", "count", len(pending))
		return
	}

	sender, err := s.senders.Build()
	if err != nil {
		slog.Error("notifications: no email provider available", "error", err)
		return
	}

	subject, body := composeEmail(prefs.AppDisplayName, pending)
	if err := sender.Send(ctx, prefs.RecipientEmails, subject, body); err != nil {
		// Deliberately don't record these as sent — leaving them pending
		// means the next tick retries automatically.
		slog.Error("notifications: send failed, will retry next tick", "error", err)
		return
	}

	for _, p := range pending {
		if err := s.store.Record(p); err != nil {
			slog.Error("notifications: record sent notification", "account_id", p.Account.ID, "threshold_id", p.Threshold.ID, "error", err)
		}
	}
}

// resolveLocation loads the configured timezone, falling back to the app
// default and then UTC if the stored value is ever invalid (e.g. hand-edited
// in the DB), so a bad preference value can't take the scheduler down.
func resolveLocation(tz string) *time.Location {
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	slog.Warn("notifications: invalid timezone preference, falling back to default", "timezone", tz, "default", preferences.DefaultTimezone)
	if loc, err := time.LoadLocation(preferences.DefaultTimezone); err == nil {
		return loc
	}
	return time.UTC
}

func composeEmail(appName string, pending []Pending) (subject, body string) {
	if len(pending) == 1 {
		subject = fmt.Sprintf("%s: %s expires soon", appName, pending[0].Account.Name)
	} else {
		subject = fmt.Sprintf("%s: %d accounts expiring soon", appName, len(pending))
	}

	var b strings.Builder
	b.WriteString("The following points accounts are approaching their expiration date:\n\n")
	for _, p := range pending {
		fmt.Fprintf(&b, "- %s (%s): %d points expire on %s (%d-day reminder)\n",
			p.Account.Name, p.Account.Provider, p.Account.PointsBalance,
			p.Account.ExpirationDate.Format("January 2, 2006"), p.Threshold.DaysBefore)
	}
	b.WriteString("\nUpdate the account's expiration date after renewing it, or use the points before they expire.\n")
	return subject, b.String()
}
