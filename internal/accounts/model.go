package accounts

import "time"

// Account is one tracked points/rewards account. Multiple accounts may
// share the same Provider (e.g. two different Chase accounts).
type Account struct {
	ID             int64
	Name           string
	Provider       string
	AccountNumber  string
	PointsBalance  int64
	ExpirationDate time.Time // date only, no time-of-day component
	Notes          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// dateLayout is the storage/parse format for ExpirationDate — a bare
// calendar date, deliberately with no time-of-day or timezone component.
// "Today" is computed separately using the user's configured preferences
// timezone (see internal/notifications).
const dateLayout = "2006-01-02"
