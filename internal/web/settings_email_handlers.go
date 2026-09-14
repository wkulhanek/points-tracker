package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wkulhanek/points-tracker/internal/auth"
	"github.com/wkulhanek/points-tracker/internal/email"
	"github.com/wkulhanek/points-tracker/internal/email/gmail"
	"github.com/wkulhanek/points-tracker/internal/templates/settings"
)

const oauthStateCookie = "oauth_state"

func handleEmailSettingsPage(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefs, _ := d.Preferences.Get()
		settings, err := d.EmailSettings.Get()
		if err != nil {
			http.Error(w, "failed to load email settings", http.StatusInternalServerError)
			return
		}
		kind, msg := flashFromQuery(r)
		gmailConfigured := d.GmailOAuth != nil && d.GmailOAuth.Configured()
		render(w, r, settingsview.Email(prefs.AppDisplayName, settings, gmailConfigured, kind, msg))
	}
}

func handleGoogleOAuthStart(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.GmailOAuth == nil || !d.GmailOAuth.Configured() {
			http.Error(w, "Gmail sign-in isn't configured on this server.", http.StatusNotFound)
			return
		}

		state, err := randomState()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookie,
			Value:    state,
			Path:     "/",
			MaxAge:   600,
			HttpOnly: true,
			Secure:   auth.IsSecureRequest(r),
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, d.GmailOAuth.AuthCodeURL(state), http.StatusSeeOther)
	}
}

func handleGoogleOAuthCallback(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.GmailOAuth == nil || !d.GmailOAuth.Configured() {
			http.Error(w, "Gmail sign-in isn't configured on this server.", http.StatusNotFound)
			return
		}

		cookie, err := r.Cookie(oauthStateCookie)
		if err != nil || cookie.Value == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(r.URL.Query().Get("state"))) != 1 {
			http.Error(w, "invalid OAuth state", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/", MaxAge: -1, Secure: auth.IsSecureRequest(r)})

		code := r.URL.Query().Get("code")
		if code == "" {
			redirectWithFlash(w, r, "/settings/email", "error", "Google sign-in was cancelled or failed.")
			return
		}

		tok, err := d.GmailOAuth.Exchange(r.Context(), code)
		if err != nil {
			redirectWithFlash(w, r, "/settings/email", "error", "Failed to complete Google sign-in.")
			return
		}

		// Google only returns a refresh token on first consent; if this is a
		// re-connect without one, fall back to whatever's already stored so we
		// don't overwrite a working refresh token with an empty string.
		refreshToken := tok.RefreshToken
		if refreshToken == "" {
			existing, _ := d.EmailSettings.Get()
			refreshToken = existing.GmailRefreshToken
		}
		if refreshToken == "" {
			redirectWithFlash(w, r, "/settings/email", "error", "Google didn't grant offline access — try disconnecting and reconnecting.")
			return
		}

		ts := d.GmailOAuth.TokenSource(r.Context(), refreshToken)
		accountEmail, _ := gmail.ProfileEmail(r.Context(), ts) // best-effort; display only

		if err := d.EmailSettings.SaveGmailTokens(refreshToken, tok.AccessToken, tok.Expiry, accountEmail); err != nil {
			http.Error(w, "failed to save Gmail connection", http.StatusInternalServerError)
			return
		}

		redirectWithFlash(w, r, "/settings/email", "success", "Gmail connected.")
	}
}

func handleGmailDisconnect(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := d.EmailSettings.DisconnectGmail(); err != nil {
			http.Error(w, "failed to disconnect Gmail", http.StatusInternalServerError)
			return
		}
		redirectWithFlash(w, r, "/settings/email", "success", "Gmail disconnected.")
	}
}

func handleSMTPSave(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			redirectWithFlash(w, r, "/settings/email", "error", "Invalid form submission.")
			return
		}

		host := strings.TrimSpace(r.FormValue("host"))
		from := strings.TrimSpace(r.FormValue("from_address"))
		if host == "" || from == "" {
			redirectWithFlash(w, r, "/settings/email", "error", "Host and from address are required.")
			return
		}

		port, err := strconv.Atoi(r.FormValue("port"))
		if err != nil || port <= 0 {
			redirectWithFlash(w, r, "/settings/email", "error", "Port must be a positive number.")
			return
		}

		security := email.SMTPSecurity(r.FormValue("security"))
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		// An empty password field means "leave the existing one unchanged" —
		// the form never echoes the real password back, so this is the only
		// way to distinguish "no change" from "clear it".
		if password == "" {
			existing, _ := d.EmailSettings.Get()
			password = existing.SMTPPassword
		}

		if err := d.EmailSettings.SaveSMTP(host, port, username, password, security, from); err != nil {
			redirectWithFlash(w, r, "/settings/email", "error", "Failed to save SMTP settings.")
			return
		}

		redirectWithFlash(w, r, "/settings/email", "success", "SMTP settings saved.")
	}
}

func handleTestEmail(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefs, err := d.Preferences.Get()
		if err != nil {
			render(w, r, settingsview.TestResult(false, "Failed to load preferences."))
			return
		}
		if len(prefs.RecipientEmails) == 0 {
			render(w, r, settingsview.TestResult(false, "Add a notification recipient on the Preferences page first."))
			return
		}

		sender, err := d.SenderFactory.Build()
		if err != nil {
			render(w, r, settingsview.TestResult(false, "No email provider is configured yet."))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		err = sender.Send(ctx, prefs.RecipientEmails, "Test email from "+prefs.AppDisplayName,
			"This is a test email from "+prefs.AppDisplayName+". If you received this, notifications are working.")
		if err != nil {
			// Log the provider error for the operator but don't echo it to the
			// browser — SMTP/Gmail errors can disclose hostnames, internal IPs,
			// or account details.
			slog.Error("test email send failed", "error", err)
			render(w, r, settingsview.TestResult(false, "Failed to send test email. Check the server logs for details."))
			return
		}

		render(w, r, settingsview.TestResult(true, "Test email sent to "+strings.Join(prefs.RecipientEmails, ", ")+"."))
	}
}

func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
