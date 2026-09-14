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
	DoesNotExpire  bool      // true for programs whose points never expire
	Owner          Owner
	Notes          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Owner records who an account belongs to. It's free text, not a fixed
// enum: the web layer suggests previously-used names (see
// Store.DistinctOwners) via an autocomplete list, the same way Provider
// suggests common providers, but a household can introduce a new owner
// simply by typing their name once.
type Owner string

// OwnerJoint is the default owner for a brand-new account, since most
// points programs start out shared until edited otherwise.
const OwnerJoint Owner = "Joint"

// dateLayout is the storage/parse format for ExpirationDate — a bare
// calendar date, deliberately with no time-of-day or timezone component.
// "Today" is computed separately using the user's configured preferences
// timezone (see internal/notifications).
const dateLayout = "2006-01-02"

// neverExpiresDate is the sentinel stored in expiration_date for accounts
// with DoesNotExpire = true. expiration_date stays NOT NULL so every other
// piece of code that assumes ExpirationDate is always a valid time.Time
// (sort order, the notification evaluator's date math, the UI badge) keeps
// working unchanged; DoesNotExpire is the actual source of truth for
// "never expires" and is checked explicitly wherever it matters. The
// sentinel just happens to also sort never-expiring accounts to the bottom
// of the list, which is the right behavior anyway.
var neverExpiresDate = time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
