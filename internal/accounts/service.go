package accounts

import (
	"database/sql"
	"fmt"
	"time"
)

// Service wraps Store with the business rule that changing an account's
// expiration date must re-arm its notification thresholds: any
// sent_notifications rows recorded against the account's old expiration
// date are stale once the date changes, and are cleared in the same
// transaction as the update so the scheduler naturally re-notifies against
// the new date without any separate "reset" step.
type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(in Input) (int64, error) {
	return s.store.Create(in)
}

func (s *Service) List() ([]Account, error) {
	return s.store.List()
}

func (s *Service) Get(id int64) (Account, error) {
	return s.store.Get(id)
}

func (s *Service) Delete(id int64) error {
	return s.store.Delete(id)
}

// Update saves the given account fields. If the expiration date is
// changing, any notification history tied to the old date is purged in the
// same transaction, so previously-sent thresholds fire again against the
// new date (e.g. after renewing/topping-up an account).
func (s *Service) Update(id int64, in Input) error {
	tx, err := s.store.BeginTx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op if committed

	oldExpiration, err := s.store.ExpirationDate(tx, id)
	if err != nil {
		return err
	}

	if err := s.store.Update(tx, id, in); err != nil {
		return fmt.Errorf("update account: %w", err)
	}

	if !oldExpiration.Equal(in.ExpirationDate) {
		if err := clearStaleNotifications(tx, id, oldExpiration); err != nil {
			return fmt.Errorf("clear stale notifications: %w", err)
		}
	}

	return tx.Commit()
}

func clearStaleNotifications(tx *sql.Tx, accountID int64, oldExpiration time.Time) error {
	_, err := tx.Exec(
		`DELETE FROM sent_notifications WHERE account_id = ? AND notified_expiration_date = ?`,
		accountID, oldExpiration.Format(dateLayout),
	)
	return err
}
