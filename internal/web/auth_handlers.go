package web

import (
	"net"
	"net/http"

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
		ip := clientIP(r)
		prefs, _ := d.Preferences.Get()

		if !d.LoginLimiter.Allow(ip) {
			w.WriteHeader(http.StatusTooManyRequests)
			render(w, r, authview.Login(prefs.AppDisplayName, "Too many attempts — try again in a minute."))
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		user, err := d.AuthStore.UserByUsername(username)
		if err != nil || !auth.VerifyPassword(user.PasswordHash, password) {
			d.LoginLimiter.RecordFailure(ip)
			w.WriteHeader(http.StatusUnauthorized)
			render(w, r, authview.Login(prefs.AppDisplayName, "Invalid username or password."))
			return
		}
		d.LoginLimiter.RecordSuccess(ip)

		sess, err := d.AuthStore.CreateSession(user.ID, r.UserAgent(), ip)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		auth.SetSessionCookie(w, sess)
		http.Redirect(w, r, "/accounts", http.StatusSeeOther)
	}
}

func handleLogout(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(auth.CookieName); err == nil {
			_ = d.AuthStore.DeleteSession(cookie.Value)
		}
		auth.ClearSessionCookie(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
