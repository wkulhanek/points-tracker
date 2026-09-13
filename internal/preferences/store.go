package preferences

import (
	"database/sql"
	"strings"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get returns the app-wide preferences singleton.
func (s *Store) Get() (AppPreferences, error) {
	var p AppPreferences
	var recipients string
	err := s.db.QueryRow(`SELECT app_display_name, timezone, recipient_emails FROM app_preferences WHERE id = 1`).
		Scan(&p.AppDisplayName, &p.Timezone, &recipients)
	if err != nil {
		return AppPreferences{}, err
	}
	p.RecipientEmails = splitEmails(recipients)
	return p, nil
}

// Save overwrites the app-wide preferences singleton.
func (s *Store) Save(p AppPreferences) error {
	_, err := s.db.Exec(
		`UPDATE app_preferences SET app_display_name = ?, timezone = ?, recipient_emails = ?, updated_at = datetime('now') WHERE id = 1`,
		p.AppDisplayName, p.Timezone, strings.Join(p.RecipientEmails, ","),
	)
	return err
}

// Thresholds returns all configured thresholds, soonest (smallest
// days-before) first.
func (s *Store) Thresholds() ([]Threshold, error) {
	rows, err := s.db.Query(`SELECT id, days_before, enabled FROM notification_thresholds ORDER BY days_before ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Threshold
	for rows.Next() {
		var t Threshold
		var enabled int
		if err := rows.Scan(&t.ID, &t.DaysBefore, &enabled); err != nil {
			return nil, err
		}
		t.Enabled = enabled != 0
		out = append(out, t)
	}
	return out, rows.Err()
}

// AddThreshold creates a new threshold.
func (s *Store) AddThreshold(daysBefore int) error {
	_, err := s.db.Exec(`INSERT INTO notification_thresholds (days_before) VALUES (?)`, daysBefore)
	return err
}

// UpdateThreshold changes an existing threshold's day count.
func (s *Store) UpdateThreshold(id int64, daysBefore int) error {
	_, err := s.db.Exec(`UPDATE notification_thresholds SET days_before = ? WHERE id = ?`, daysBefore, id)
	return err
}

// DeleteThreshold removes a threshold (and, via cascade, its notification
// history).
func (s *Store) DeleteThreshold(id int64) error {
	_, err := s.db.Exec(`DELETE FROM notification_thresholds WHERE id = ?`, id)
	return err
}

func splitEmails(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if e := strings.TrimSpace(p); e != "" {
			out = append(out, e)
		}
	}
	return out
}
