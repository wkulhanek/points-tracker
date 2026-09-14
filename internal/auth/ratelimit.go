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
	// maxKeys caps how many distinct IPs are tracked at once so a flood of
	// spoofed client addresses can't grow the map without bound.
	maxKeys int
}

func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		attempts: make(map[string][]time.Time),
		max:      5,
		window:   time.Minute,
		maxKeys:  10000,
	}
}

// Allow reports whether another login attempt from ip is currently
// permitted, without recording one.
func (l *LoginLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recentLocked(ip)) < l.max
}

// RecordFailure records a failed login attempt from ip. If the map is
// already at its cap and cleanup can't free room (every tracked IP still
// has attempts inside the window — e.g. a sustained flood from many
// distinct/spoofed addresses), a never-before-seen ip is dropped rather
// than growing the map further: a burst that big means the limiter is
// already at capacity doing its job for the IPs it has room to track, and
// unbounded memory growth is worse than under-tracking a few more.
func (l *LoginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.attempts) >= l.maxKeys {
		l.cleanupLocked()
	}
	if _, tracked := l.attempts[ip]; !tracked && len(l.attempts) >= l.maxKeys {
		return
	}
	l.attempts[ip] = append(l.recentLocked(ip), time.Now())
}

// Cleanup drops entries whose attempts have all aged out of the window.
// Call it periodically to release memory for IPs that never log in
// successfully.
func (l *LoginLimiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanupLocked()
}

// cleanupLocked removes entries with no attempts left in the window. Caller
// must hold l.mu.
func (l *LoginLimiter) cleanupLocked() {
	for ip, times := range l.attempts {
		cutoff := time.Now().Add(-l.window)
		keep := false
		for _, t := range times {
			if t.After(cutoff) {
				keep = true
				break
			}
		}
		if !keep {
			delete(l.attempts, ip)
		}
	}
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
