package auth

import (
	"context"
	"errors"
	"net/http"
)

type contextKey int

const userContextKey contextKey = 0

// SetSessionCookie writes the session cookie for a newly created session.
// Secure is always set: the app is only ever expected to be reached through
// a TLS-terminating reverse proxy, even though it speaks plain HTTP itself.
func SetSessionCookie(w http.ResponseWriter, sess Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	})
}

// ClearSessionCookie removes the session cookie (logout).
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
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
