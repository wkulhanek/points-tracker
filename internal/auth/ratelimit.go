package auth

import (
	"sync"
	"time"
)

// LoginLimiter is a small in-memory sliding-window rate limiter for the
// login form, keyed by client IP. It's intentionally simple (no
// persistence, resets on restart) — this app has a single admin account, so
// the goal is just to slow down brute-force guessing, not to be a general
// abuse-prevention system.
type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
}

func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		attempts: make(map[string][]time.Time),
		max:      5,
		window:   time.Minute,
	}
}

// Allow reports whether another login attempt from ip is currently
// permitted, without recording one.
func (l *LoginLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recentLocked(ip)) < l.max
}

// RecordFailure records a failed login attempt from ip.
func (l *LoginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts[ip] = append(l.recentLocked(ip), time.Now())
}

// RecordSuccess clears any recorded failures for ip.
func (l *LoginLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// recentLocked returns (and stores back) only the attempts still within the
// window. Caller must hold l.mu.
func (l *LoginLimiter) recentLocked(ip string) []time.Time {
	cutoff := time.Now().Add(-l.window)
	var kept []time.Time
	for _, t := range l.attempts[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	l.attempts[ip] = kept
	return kept
}
