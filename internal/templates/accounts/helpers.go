package accountsview

import (
	"fmt"
	"time"

	"github.com/wkulhanek/kulhanek-points-tracker/internal/accounts"
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
