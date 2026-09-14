package accounts

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("account not found")

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Input groups the user-editable fields of an account, shared by create and
// update.
type Input struct {
	Name           string
	Provider       string
	AccountNumber  string
	PointsBalance  int64
	ExpirationDate time.Time
	DoesNotExpire  bool
	Owner          Owner
	Notes          string
}

// storedExpiration returns the date to persist for an account: the real
// expiration date, or the never-expires sentinel when DoesNotExpire is set.
func (in Input) storedExpiration() time.Time {
	if in.DoesNotExpire {
		return neverExpiresDate
	}
	return in.ExpirationDate
}

// List returns all accounts ordered by soonest-expiring first, since that's
// the order the accounts list page wants to surface them in.
func (s *Store) List() ([]Account, error) {
	rows, err := s.db.Query(`
		SELECT id, name, provider, account_number, points_balance, expiration_date, does_not_expire, owner, notes, created_at, updated_at
		FROM accounts
		ORDER BY expiration_date ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Get returns a single account by ID.
func (s *Store) Get(id int64) (Account, error) {
	row := s.db.QueryRow(`
		SELECT id, name, provider, account_number, points_balance, expiration_date, does_not_expire, owner, notes, created_at, updated_at
		FROM accounts WHERE id = ?`, id)
	a, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return a, err
}

// Create inserts a new account and returns its ID.
func (s *Store) Create(in Input) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO accounts (name, provider, account_number, points_balance, expiration_date, does_not_expire, owner, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, in.Provider, in.AccountNumber, in.PointsBalance, in.storedExpiration().Format(dateLayout), in.DoesNotExpire, in.Owner, in.Notes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update overwrites an account's editable fields in place, returning the
// previous expiration date so the caller (internal/accounts.Service) can
// decide whether stale notification state needs clearing.
func (s *Store) Update(tx *sql.Tx, id int64, in Input) error {
	_, err := tx.Exec(`
		UPDATE accounts
		SET name = ?, provider = ?, account_number = ?, points_balance = ?, expiration_date = ?, does_not_expire = ?, owner = ?, notes = ?,
		    updated_at = datetime('now')
		WHERE id = ?`,
		in.Name, in.Provider, in.AccountNumber, in.PointsBalance, in.storedExpiration().Format(dateLayout), in.DoesNotExpire, in.Owner, in.Notes, id)
	return err
}

// ExpirationDate returns just the currently stored expiration date for an
// account, used by Service.Update to detect whether it's changing.
func (s *Store) ExpirationDate(tx *sql.Tx, id int64) (time.Time, error) {
	var raw string
	err := tx.QueryRow(`SELECT expiration_date FROM accounts WHERE id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrNotFound
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(dateLayout, raw)
}

// Delete removes an account (and, via ON DELETE CASCADE, its notification
// history).
func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	return err
}

// BeginTx starts a transaction for callers (Service) that need to combine
// an account update with other statements atomically.
func (s *Store) BeginTx() (*sql.Tx, error) {
	return s.db.Begin()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner) (Account, error) {
	var a Account
	var expiration, owner, createdAt, updatedAt string
	var accountNumber, notes sql.NullString
	var doesNotExpire bool

	err := row.Scan(&a.ID, &a.Name, &a.Provider, &accountNumber, &a.PointsBalance, &expiration, &doesNotExpire, &owner, &notes, &createdAt, &updatedAt)
	if err != nil {
		return Account{}, err
	}
	a.AccountNumber = accountNumber.String
	a.DoesNotExpire = doesNotExpire
	a.Owner = Owner(owner)
	a.Notes = notes.String

	a.ExpirationDate, err = time.Parse(dateLayout, expiration)
	if err != nil {
		return Account{}, fmt.Errorf("parse expiration_date: %w", err)
	}
	a.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		return Account{}, fmt.Errorf("parse created_at: %w", err)
	}
	a.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
	if err != nil {
		return Account{}, fmt.Errorf("parse updated_at: %w", err)
	}
	return a, nil
}
