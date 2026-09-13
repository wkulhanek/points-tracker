package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wkulhanek/kulhanek-points-tracker/internal/preferences"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/templates/settings"
)

func handlePreferencesPage(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefs, err := d.Preferences.Get()
		if err != nil {
			http.Error(w, "failed to load preferences", http.StatusInternalServerError)
			return
		}
		thresholds, err := d.Preferences.Thresholds()
		if err != nil {
			http.Error(w, "failed to load thresholds", http.StatusInternalServerError)
			return
		}
		kind, msg := flashFromQuery(r)
		render(w, r, settingsview.Preferences(prefs.AppDisplayName, prefs, thresholds, kind, msg))
	}
}

func handlePreferencesGeneralSave(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			redirectWithFlash(w, r, "/settings/preferences", "error", "Invalid form submission.")
			return
		}

		appName := strings.TrimSpace(r.FormValue("app_display_name"))
		tz := strings.TrimSpace(r.FormValue("timezone"))
		if appName == "" || tz == "" {
			redirectWithFlash(w, r, "/settings/preferences", "error", "Application name and timezone are required.")
			return
		}
		if _, err := time.LoadLocation(tz); err != nil {
			redirectWithFlash(w, r, "/settings/preferences", "error", "\""+tz+"\" isn't a recognized timezone (use an IANA name like Europe/Vienna).")
			return
		}

		p := preferences.AppPreferences{
			AppDisplayName:  appName,
			Timezone:        tz,
			RecipientEmails: splitEmails(r.FormValue("recipient_emails")),
		}
		if err := d.Preferences.Save(p); err != nil {
			redirectWithFlash(w, r, "/settings/preferences", "error", "Failed to save preferences.")
			return
		}

		redirectWithFlash(w, r, "/settings/preferences", "success", "Preferences saved.")
	}
}

func handleThresholdAdd(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days, err := strconv.Atoi(r.FormValue("days_before"))
		if err != nil || days <= 0 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		if err := d.Preferences.AddThreshold(days); err != nil {
			http.Error(w, "failed to add threshold", http.StatusInternalServerError)
			return
		}

		thresholds, err := d.Preferences.Thresholds()
		if err != nil {
			http.Error(w, "failed to load thresholds", http.StatusInternalServerError)
			return
		}
		render(w, r, settingsview.ThresholdsList(thresholds))
	}
}

func handleThresholdDelete(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err := d.Preferences.DeleteThreshold(id); err != nil {
			http.Error(w, "failed to delete threshold", http.StatusInternalServerError)
			return
		}

		thresholds, err := d.Preferences.Thresholds()
		if err != nil {
			http.Error(w, "failed to load thresholds", http.StatusInternalServerError)
			return
		}
		render(w, r, settingsview.ThresholdsList(thresholds))
	}
}
