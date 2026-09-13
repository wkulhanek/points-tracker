package components

import (
	"fmt"
	"time"
)

// badgeLabel/badgeClass use the server's local wall-clock time rather than
// the app's configured notification timezone preference — this is purely a
// cosmetic "how soon" indicator in the UI, unlike the notification
// scheduler (internal/notifications), which is careful to use the
// configured timezone since it controls when emails actually fire.
func badgeLabel(expiration time.Time) string {
	days := daysUntil(expiration)
	switch {
	case days < 0:
		return "Expired"
	case days == 0:
		return "Expires today"
	case days == 1:
		return "1 day left"
	default:
		return fmt.Sprintf("%d days left", days)
	}
}

func badgeClass(expiration time.Time) string {
	const base = "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium "
	days := daysUntil(expiration)
	switch {
	case days < 0:
		return base + "bg-slate-200 text-slate-600"
	case days <= 7:
		return base + "bg-red-100 text-red-700"
	case days <= 30:
		return base + "bg-amber-100 text-amber-700"
	default:
		return base + "bg-emerald-100 text-emerald-700"
	}
}

func daysUntil(expiration time.Time) int {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	exp := time.Date(expiration.Year(), expiration.Month(), expiration.Day(), 0, 0, 0, 0, now.Location())
	return int(exp.Sub(today).Hours() / 24)
}
