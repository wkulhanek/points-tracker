package accountsview

import (
	"fmt"
	"strings"
	"time"

	"github.com/wkulhanek/points-tracker/internal/accounts"
)

func rowID(id int64) string {
	return fmt.Sprintf("account-row-%d", id)
}

func formatPoints(n int64) string {
	return fmt.Sprintf("%d", n)
}

// ownerOrDefault defaults a brand-new (zero-value) account's owner to
// Joint, so the "Add account" form's owner field doesn't start out blank.
func ownerOrDefault(a accounts.Account) accounts.Owner {
	if a.Owner == "" {
		return accounts.OwnerJoint
	}
	return a.Owner
}

// kindOrDefault defaults a brand-new (zero-value) account's kind to Other,
// so the "Add account" form's dropdown doesn't start out unselected.
func kindOrDefault(a accounts.Account) accounts.Kind {
	if a.Kind == "" {
		return accounts.KindOther
	}
	return a.Kind
}

// formatLastUpdated renders the points-balance "last updated" timestamp
// shown (read-only) on the account form, or an em dash for a brand-new
// account that hasn't been saved yet.
func formatLastUpdated(a accounts.Account) string {
	if a.PointsUpdatedAt.IsZero() {
		return "—"
	}
	return a.PointsUpdatedAt.Format("Jan 2, 2006 3:04 PM")
}

// expirationInputValue formats an account's expiration date for an
// <input type="date">, defaulting to one year out for a brand-new
// (zero-value) account, or a DoesNotExpire account (whose stored date is
// just a storage sentinel, not meant to be shown) — so the field starts on
// a sensible date if "Does not expire" is later unchecked.
func expirationInputValue(a accounts.Account) string {
	d := a.ExpirationDate
	if d.IsZero() || a.DoesNotExpire {
		d = time.Now().AddDate(1, 0, 0)
	}
	return d.Format("2006-01-02")
}

// sortText normalizes a text column's value for the accounts list's
// client-side column sort (see row.templ's data-sort-value attributes and
// web/static/js/sort-table.js).
func sortText(s string) string {
	return strings.ToLower(s)
}
