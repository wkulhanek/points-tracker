package web

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wkulhanek/points-tracker/internal/email"
	"github.com/wkulhanek/points-tracker/internal/templates/settings"
)

func handleEmailSettingsPage(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefs, _ := d.Preferences.Get()
		settings, err := d.EmailSettings.Get()
		if err != nil {
			http.Error(w, "failed to load email settings", http.StatusInternalServerError)
			return
		}
		kind, msg := flashFromQuery(r)
		render(w, r, settingsview.Email(prefs.AppDisplayName, settings, kind, msg))
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
			// browser — SMTP errors can disclose hostnames, internal IPs, or
			// account details.
			slog.Error("test email send failed", "error", err)
			render(w, r, settingsview.TestResult(false, "Failed to send test email. Check the server logs for details."))
			return
		}

		render(w, r, settingsview.TestResult(true, "Test email sent to "+strings.Join(prefs.RecipientEmails, ", ")+"."))
	}
}
