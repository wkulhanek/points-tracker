// Package notifications decides when an account's approaching expiration
// should trigger an email, and makes sure each (account, threshold,
// expiration date) combination is only ever notified once.
package notifications

import (
	"time"

	"github.com/wkulhanek/points-tracker/internal/accounts"
	"github.com/wkulhanek/points-tracker/internal/preferences"
)

// SentNotification records that a threshold has already fired for an
// account's *current* expiration date. If the expiration date later
// changes, internal/accounts.Service clears the matching row so the
// threshold can fire again against the new date.
type SentNotification struct {
	AccountID              int64
	ThresholdID            int64
	NotifiedExpirationDate time.Time
}

// Pending is one (account, threshold) pair whose trigger date has been
// reached and that hasn't been notified yet.
type Pending struct {
	Account   accounts.Account
	Threshold preferences.Threshold
}
