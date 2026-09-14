package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type contextKey int

const userContextKey contextKey = 0

// SetSessionCookie writes the session cookie for a newly created session.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, sess Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   IsSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	})
}

// ClearSessionCookie removes the session cookie (logout).
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   IsSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// IsSecureRequest reports whether the request reached us over HTTPS, either
// directly or (the expected case in production) terminated by a reverse
// proxy that sets X-Forwarded-Proto. The Secure cookie attribute is only
// set when this is true: hardcoding it unconditionally would make the app
// impossible to log into when run directly over plain HTTP — e.g. testing
// a container locally before putting it behind a real reverse proxy —
// since browsers refuse to send Secure cookies back over an insecure
// connection. This app is still only ever *intended* to be reached through
// a TLS-terminating reverse proxy in production; this just makes that
// non-negotiable in every environment.
func IsSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// RequireAuth protects handlers behind a valid session cookie. HTMX partial
// requests get an HX-Redirect response header (the correct HTMX pattern for
// redirecting mid-interaction) instead of a normal 302, since HTMX otherwise
// swaps the login page's HTML into the calling fragment's target.
func RequireAuth(store *Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				redirectToLogin(w, r)
				return
			}

			sess, err := store.SessionByToken(cookie.Value)
			if err != nil {
				if !errors.Is(err, ErrNotFound) {
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				redirectToLogin(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, sess.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// UserIDFromContext returns the authenticated user's ID, set by RequireAuth.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userContextKey).(int64)
	return id, ok
}
