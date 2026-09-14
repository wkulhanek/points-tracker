package notifications

import (
	"time"

	"github.com/wkulhanek/points-tracker/internal/accounts"
	"github.com/wkulhanek/points-tracker/internal/preferences"
)

type sentKey struct {
	accountID   int64
	thresholdID int64
	expiration  string // formatted date, so time.Time's monotonic/location bits can't break equality
}

// Evaluate is a pure function (no DB, no wall-clock access) that decides
// which (account, threshold) pairs should be notified right now. A pair is
// due once `now` reaches the account's expiration date minus the
// threshold's day count, and hasn't already been recorded in `sent` for the
// account's *current* expiration date.
//
// Being pure like this makes the tricky bits — boundary conditions and the
// reset-on-renewal behavior — straightforward to unit test without a
// database or clock mocking.
func Evaluate(now time.Time, accts []accounts.Account, thresholds []preferences.Threshold, sent []SentNotification) []Pending {
	sentSet := make(map[sentKey]bool, len(sent))
	for _, sn := range sent {
		sentSet[sentKey{sn.AccountID, sn.ThresholdID, dateKey(sn.NotifiedExpirationDate)}] = true
	}

	var pending []Pending
	for _, a := range accts {
		if a.DoesNotExpire {
			continue
		}
		for _, t := range thresholds {
			if !t.Enabled {
				continue
			}
			triggerDate := a.ExpirationDate.AddDate(0, 0, -t.DaysBefore)
			if now.Before(triggerDate) {
				continue
			}
			key := sentKey{a.ID, t.ID, dateKey(a.ExpirationDate)}
			if sentSet[key] {
				continue
			}
			pending = append(pending, Pending{Account: a, Threshold: t})
		}
	}
	return pending
}

func dateKey(t time.Time) string {
	return t.Format("2006-01-02")
}
