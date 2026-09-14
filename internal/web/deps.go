// Package web wires together every domain package into HTTP handlers and a
// router. Deps holds everything a handler might need; handlers are
// constructed as closures over *Deps rather than methods on a God object,
// which keeps each handler file focused on one area of the app.
package web

import (
	"github.com/wkulhanek/points-tracker/internal/accounts"
	"github.com/wkulhanek/points-tracker/internal/auth"
	"github.com/wkulhanek/points-tracker/internal/email"
	"github.com/wkulhanek/points-tracker/internal/email/factory"
	"github.com/wkulhanek/points-tracker/internal/email/gmail"
	"github.com/wkulhanek/points-tracker/internal/preferences"
)

type Deps struct {
	AuthStore     *auth.Store
	LoginLimiter  *auth.LoginLimiter
	Accounts      *accounts.Service
	Preferences   *preferences.Store
	EmailSettings *email.SettingsStore
	GmailOAuth    *gmail.OAuth // nil if GOOGLE_CLIENT_ID/SECRET aren't configured
	SenderFactory *factory.Factory
	// TrustProxy mirrors config.Config.TrustProxy: whether X-Forwarded-For
	// may be trusted to identify the client for rate limiting.
	TrustProxy bool
	// BaseURL is the externally-reachable URL, used as a trusted reference
	// for the same-origin (CSRF) check.
	BaseURL string
}
