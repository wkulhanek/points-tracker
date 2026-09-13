// Package preferences manages app-wide, user-editable settings: the
// display name shown throughout the UI, the timezone used to decide
// "today" when checking expiration thresholds, who gets notified, and the
// list of day-count thresholds themselves.
package preferences

// AppPreferences is the singleton row of app-wide settings.
type AppPreferences struct {
	AppDisplayName  string
	Timezone        string // IANA zone name, e.g. "Europe/Vienna"
	RecipientEmails []string
}

// Threshold is one "notify N days before expiration" rule.
type Threshold struct {
	ID         int64
	DaysBefore int
	Enabled    bool
}

// DefaultTimezone is used both as the migration's default column value and
// as a fallback if the stored value ever fails to parse as a valid zone.
const DefaultTimezone = "Europe/Vienna"
