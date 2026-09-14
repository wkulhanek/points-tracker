// Package config loads application configuration from environment variables.
//
// Everything here is deliberately infrastructure/secret-level configuration
// (port, data directory, admin bootstrap credentials). User-editable
// behavior — app name, timezone, notification thresholds/recipients, which
// email provider is active — lives in the database instead (see
// internal/preferences and internal/email) so it can be changed from the UI
// without a restart or redeploy.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	// Port the HTTP server listens on.
	Port string
	// DataDir is where the SQLite database file (and its backups
	// subdirectory) lives. Must be writable by the process.
	DataDir string
	// BaseURL is the externally-reachable URL of this app (behind whatever
	// reverse proxy fronts it), used as the trusted origin for the
	// same-origin (CSRF) check on state-changing requests.
	BaseURL string

	// AdminUsername/AdminPassword seed the single admin user on first boot,
	// only if the users table is empty. Ignored on subsequent boots.
	AdminUsername string
	AdminPassword string

	// TrustProxy indicates that the app is reachable only through a trusted
	// reverse proxy that sets X-Forwarded-* headers. When true, the client
	// IP used for login rate limiting is taken from X-Forwarded-For; when
	// false, r.RemoteAddr is used. Defaults to true because the app is
	// designed to run behind a proxy, but it must then never be exposed
	// directly — otherwise clients can spoof X-Forwarded-For. Set
	// TRUST_PROXY=false if the app is reachable directly.
	TrustProxy bool
}

// Load reads configuration from environment variables, applying sensible
// defaults where possible, and returns an error if anything required is
// missing.
func Load() (Config, error) {
	cfg := Config{
		Port:          getEnv("PORT", "8080"),
		DataDir:       getEnv("DATA_DIR", "./data"),
		BaseURL:       getEnv("BASE_URL", "http://localhost:8080"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		TrustProxy:    !strings.EqualFold(getEnv("TRUST_PROXY", "true"), "false"),
	}

	if cfg.AdminUsername == "" || cfg.AdminPassword == "" {
		return Config{}, fmt.Errorf("ADMIN_USERNAME and ADMIN_PASSWORD must both be set (used to seed the admin account on first boot)")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
