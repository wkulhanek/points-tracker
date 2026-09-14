package web

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/wkulhanek/kulhanek-points-tracker/internal/auth"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/templates/auth"
)

func handleLoginPage(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefs, _ := d.Preferences.Get()
		render(w, r, authview.Login(prefs.AppDisplayName, ""))
	}
}

func handleLoginSubmit(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, d.TrustProxy)
		prefs, _ := d.Preferences.Get()

		if !d.LoginLimiter.Allow(ip) {
			slog.Warn("login rate limited", "ip", ip)
			w.WriteHeader(http.StatusTooManyRequests)
			render(w, r, authview.Login(prefs.AppDisplayName, "Too many attempts — try again in a minute."))
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		user, err := d.AuthStore.UserByUsername(username)
		// Always run a bcrypt comparison, even for an unknown username, so
		// response timing can't be used to enumerate accounts.
		hash := user.PasswordHash
		if err != nil {
			hash = auth.DummyPasswordHash
		}
		if !auth.VerifyPassword(hash, password) || err != nil {
			d.LoginLimiter.RecordFailure(ip)
			slog.Warn("login failed", "username", username, "ip", ip)
			w.WriteHeader(http.StatusUnauthorized)
			render(w, r, authview.Login(prefs.AppDisplayName, "Invalid username or password."))
			return
		}
		d.LoginLimiter.RecordSuccess(ip)
		slog.Info("login succeeded", "username", username, "ip", ip)

		sess, err := d.AuthStore.CreateSession(user.ID, r.UserAgent(), ip)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		auth.SetSessionCookie(w, r, sess)
		http.Redirect(w, r, "/accounts", http.StatusSeeOther)
	}
}

func handleLogout(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(auth.CookieName); err == nil {
			_ = d.AuthStore.DeleteSession(cookie.Value)
		}
		auth.ClearSessionCookie(w, r)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// The trusted proxy appends the real client address to
		// X-Forwarded-For, so the right-most entry is the one we can rely on;
		// anything the client sent earlier is spoofable and ignored.
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
