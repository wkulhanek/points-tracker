package notifications

import (
	"database/sql"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// All returns every recorded sent-notification row.
func (s *Store) All() ([]SentNotification, error) {
	rows, err := s.db.Query(`SELECT account_id, threshold_id, notified_expiration_date FROM sent_notifications`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SentNotification
	for rows.Next() {
		var sn SentNotification
		var raw string
		if err := rows.Scan(&sn.AccountID, &sn.ThresholdID, &raw); err != nil {
			return nil, err
		}
		sn.NotifiedExpirationDate, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// Record marks a (account, threshold, expiration date) combination as
// notified. Safe to call even if already recorded (e.g. a race between two
// scheduler ticks) since the underlying column has a UNIQUE constraint.
func (s *Store) Record(p Pending) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO sent_notifications (account_id, threshold_id, notified_expiration_date) VALUES (?, ?, ?)`,
		p.Account.ID, p.Threshold.ID, p.Account.ExpirationDate.Format("2006-01-02"),
	)
	return err
}
