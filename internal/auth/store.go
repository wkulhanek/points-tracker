package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// UserCount returns how many admin users exist (0 or 1 in normal operation).
func (s *Store) UserCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser inserts a new admin user. Only ever called by the first-boot
// bootstrap when the users table is empty.
func (s *Store) CreateUser(username, passwordHash string) error {
	_, err := s.db.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, passwordHash)
	return err
}

// UserByUsername looks up a user by username for login verification.
func (s *Store) UserByUsername(username string) (User, error) {
	var u User
	err := s.db.QueryRow(`SELECT id, username, password_hash FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// CreateSession issues a new random session token for the given user.
func (s *Store) CreateSession(userID int64, userAgent, ipAddress string) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate session token: %w", err)
	}

	sess := Session{
		ID:        token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(SessionTTL),
	}

	_, err = s.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at, user_agent, ip_address) VALUES (?, ?, ?, ?, ?)`,
		sess.ID, sess.UserID, sess.ExpiresAt.UTC().Format(time.RFC3339), userAgent, ipAddress,
	)
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

// SessionByToken looks up a non-expired session by its token.
func (s *Store) SessionByToken(token string) (Session, error) {
	var sess Session
	var expiresAt string
	err := s.db.QueryRow(`SELECT id, user_id, expires_at FROM sessions WHERE id = ?`, token).
		Scan(&sess.ID, &sess.UserID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}

	sess.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return Session{}, fmt.Errorf("parse session expiry: %w", err)
	}
	if sess.ExpiresAt.Before(time.Now()) {
		return Session{}, ErrNotFound
	}
	return sess, nil
}

// DeleteSession revokes a session (logout).
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, token)
	return err
}

// DeleteExpiredSessions prunes stale session rows; safe to call periodically.
func (s *Store) DeleteExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
	return err
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
