// Package web wires together every domain package into HTTP handlers and a
// router. Deps holds everything a handler might need; handlers are
// constructed as closures over *Deps rather than methods on a God object,
// which keeps each handler file focused on one area of the app.
package web

import (
	"github.com/wkulhanek/kulhanek-points-tracker/internal/accounts"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/auth"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/email"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/email/factory"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/email/gmail"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/preferences"
)

type Deps struct {
	AuthStore     *auth.Store
	LoginLimiter  *auth.LoginLimiter
	Accounts      *accounts.Service
	Preferences   *preferences.Store
	EmailSettings *email.SettingsStore
	GmailOAuth    *gmail.OAuth // nil if GOOGLE_CLIENT_ID/SECRET aren't configured
	SenderFactory *factory.Factory
}
