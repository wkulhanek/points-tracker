package web

import (
	"net/http"

	"github.com/wkulhanek/points-tracker/internal/auth"
	webassets "github.com/wkulhanek/points-tracker/web"
)

// NewRouter builds the complete HTTP handler for the app: a single
// stdlib http.ServeMux (Go 1.22+'s method+pattern routing is enough here —
// no third-party router needed), with protected routes individually wrapped
// in auth.RequireAuth rather than mounted behind a submux, since that keeps
// each route's auth requirement visible right next to its registration.
func NewRouter(d *Deps) http.Handler {
	mux := http.NewServeMux()
	protect := auth.RequireAuth(d.AuthStore)

	// Static assets — unauthenticated, embedded into the binary.
	mux.Handle("GET /static/", http.FileServerFS(webassets.FS))

	// Auth.
	mux.HandleFunc("GET /login", handleLoginPage(d))
	mux.HandleFunc("POST /login", handleLoginSubmit(d))
	mux.HandleFunc("POST /logout", handleLogout(d))

	// Root.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/accounts", http.StatusSeeOther)
	})

	// Accounts.
	mux.Handle("GET /accounts", protect(handleAccountsList(d)))
	mux.Handle("GET /accounts/new", protect(handleAccountNewForm(d)))
	mux.Handle("POST /accounts", protect(handleAccountCreate(d)))
	mux.Handle("GET /accounts/{id}", protect(handleAccountView(d)))
	mux.Handle("GET /accounts/{id}/edit", protect(handleAccountEditForm(d)))
	mux.Handle("POST /accounts/{id}", protect(handleAccountUpdate(d)))
	mux.Handle("POST /accounts/{id}/delete", protect(handleAccountDelete(d)))

	// Email settings, including the Gmail OAuth flow. The callback is left
	// unauthenticated since it's reached via a top-level browser redirect
	// from Google rather than an in-app link; the state cookie is what
	// actually guards it.
	mux.Handle("GET /settings/email", protect(handleEmailSettingsPage(d)))
	mux.Handle("GET /oauth/google/start", protect(handleGoogleOAuthStart(d)))
	mux.HandleFunc("GET /oauth/google/callback", handleGoogleOAuthCallback(d))
	mux.Handle("POST /settings/email/gmail/disconnect", protect(handleGmailDisconnect(d)))
	mux.Handle("POST /settings/email/smtp", protect(handleSMTPSave(d)))
	mux.Handle("POST /settings/email/test", protect(handleTestEmail(d)))

	// Preferences.
	mux.Handle("GET /settings/preferences", protect(handlePreferencesPage(d)))
	mux.Handle("POST /settings/preferences/general", protect(handlePreferencesGeneralSave(d)))
	mux.Handle("POST /settings/preferences/thresholds", protect(handleThresholdAdd(d)))
	mux.Handle("POST /settings/preferences/thresholds/{id}/delete", protect(handleThresholdDelete(d)))

	// Apply cross-cutting hardening to every route: security headers, a
	// request-body cap, and a same-origin check on state-changing methods.
	return securityHeaders(limitBody(requireSameOrigin(mux, d.BaseURL)))
}
