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

// expirationInputValue formats an account's expiration date for an
// <input type="date">, defaulting a brand-new (zero-value) account to one
// year out so the "Add account" form doesn't start on an already-expired
// date.
func expirationInputValue(a accounts.Account) string {
	d := a.ExpirationDate
	if d.IsZero() {
		d = time.Now().AddDate(1, 0, 0)
	}
	return d.Format("2006-01-02")
}
