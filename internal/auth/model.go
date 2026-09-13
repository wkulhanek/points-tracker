package auth

import "time"

type User struct {
	ID           int64
	Username     string
	PasswordHash string
}

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

// SessionTTL is how long a login stays valid before requiring re-authentication.
const SessionTTL = 30 * 24 * time.Hour

// CookieName is the name of the session cookie set on successful login.
const CookieName = "session"
