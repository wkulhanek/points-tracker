# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Kulhanek Points Tracker: a small, self-hosted Go app that tracks loyalty/rewards points accounts and emails a warning before they expire. Single admin user, SQLite storage, no SPA — see README.md for the full feature list and deployment instructions (Containerfile, Podman Quadlet on Fedora, CI to quay.io). This file only covers what's needed to work productively on the code itself.

## Commands

```sh
make run        # templ generate + tailwind build + go run ./cmd/server (port 8080)
make build      # same, but produces bin/server
make test       # go test ./...
make vet        # go vet ./...
make generate   # templ generate only (regenerate _templ.go from .templ files)
make tailwind   # rebuild web/static/css/app.css only
```

Requires `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars to boot (seeds the single admin user on first run against an empty DB — see `.env.example` for the full list and README.md for what each one does). Requires Go 1.27+ and Node.js (Node is only used for `npx tailwindcss`; there's no JS app or `node_modules`).

Run a single test: `go test ./internal/notifications/ -run TestEvaluate -v` (substitute the package path and test name). The three existing test files are `internal/notifications/evaluator_test.go`, `internal/web/middleware_test.go`, and `internal/db/migrate_test.go` — start there for examples of this codebase's testing style (table-driven, no mocking framework).

**After editing any `.templ` file**, you must run `make generate` (or `make run`/`make build`, which do it automatically) before `go build`/`go test` will see the change — the generated `_templ.go` files are committed to the repo, so a stale one will silently compile and run the old template. CI checks this via `templ generate && git diff --exit-code`.

## Architecture

**Package layout** (`internal/`): each business domain is its own package with its own SQLite-backed store — `accounts`, `auth`, `backup`, `config`, `db`, `email` (+ `email/gmail`, `email/smtp`, `email/factory`), `notifications`, `preferences`, `templates`, `web`. `cmd/server/main.go` is the only place that wires them together (opens the DB, constructs every store/service, starts the HTTP server and the two background schedulers, handles graceful shutdown). There's no dependency-injection framework — `internal/web.Deps` is a plain struct of everything a handler might need, and handlers are closures over `*Deps` rather than methods, so each handler file stays focused on one route group (`accounts_handlers.go`, `auth_handlers.go`, `settings_email_handlers.go`, `settings_prefs_handlers.go`).

**Request pipeline** (`internal/web/router.go`): a single `http.ServeMux` (Go 1.22+ method+pattern routing), with each protected route individually wrapped in `auth.RequireAuth` right at registration (not mounted behind a submux) so a route's auth requirement is visible next to its handler. The whole mux is then wrapped in `securityHeaders(limitBody(requireSameOrigin(mux, d.BaseURL)))` (`internal/web/middleware.go`) — security response headers, a request body size cap, and a same-origin check on state-changing methods as CSRF defense-in-depth on top of the `SameSite=Lax` session cookie. The one unauthenticated, non-static route is `GET /oauth/google/callback`, which is instead guarded by a short-lived `oauth_state` cookie compared with `crypto/subtle`.

**Notifications** (`internal/notifications/`): `evaluator.go` is a pure function — `Evaluate(now time.Time, accounts, thresholds, sent) []Pending` — with no DB or clock access, so the duplicate-prevention and reset-on-renewal logic is unit-testable without a database (see `evaluator_test.go`). `scheduler.go` wraps it in an hourly ticker (not daily — cheap and idempotent, so a restart self-corrects instead of possibly missing a once-a-day window) that loads state, calls `Evaluate`, sends via the active `email.Sender`, and records `sent_notifications` rows only on a successful send (a failed send is retried on the next tick since nothing was recorded). The trickiest invariant: `internal/accounts/service.go`'s `Update` clears stale `sent_notifications` rows in the same transaction whenever `expiration_date` changes, so renewing an account re-arms its thresholds — this is the only place notification state is reset, deliberately kept out of the evaluator itself.

**Email sending** (`internal/email/`): `sender.go` defines a single `Sender` interface; `email/factory` reads the `email_settings` singleton row's `provider_type` and returns either the `email/gmail` or `email/smtp` implementation. Both the notification scheduler and the "Test email" button send through this one interface, so there's exactly one send path per provider to reason about. Gmail auth only ever requests the `gmail.send` scope; the OAuth flow lives in `email/gmail/oauth.go`.

**Database** (`internal/db/`): `db.go` opens a single connection (`SetMaxOpenConns(1)` — SQLite has one writer anyway, so this avoids `SQLITE_BUSY` without an external pool/lock) with `foreign_keys(ON)`, WAL mode, and a busy timeout, all set via DSN pragmas. `migrate.go` applies embedded `migrations/*.sql` files in numeric order inside a `schema_migrations`-tracked transaction each — but first disables `foreign_keys` for the whole migration run (only possible outside a transaction) and runs `PRAGMA foreign_key_check` at the end before re-enabling it. That toggle exists because SQLite has no `ALTER TABLE...DROP CONSTRAINT`; a migration that needs to change a constraint has to rebuild the table via create-copy-drop-rename, and with `foreign_keys` ON, dropping a table cascades `ON DELETE CASCADE` against anything referencing it — silently deleting unrelated rows the migration never intended to touch. (`migrate_test.go` reproduces that exact scenario as a regression test.) Keep this pattern in mind for any future migration that rebuilds a table with incoming foreign-key references.

**Auth**: session tokens are `crypto/rand`-generated but only their SHA-256 hash is ever persisted (`internal/auth/store.go`) — the raw token exists only in the cookie and in-memory between generation and being set, so a leaked DB or backup snapshot can't be replayed as a live session. Login is timing-safe against username enumeration (an unknown username still runs a full bcrypt comparison against a dummy hash). The login rate limiter (`internal/auth/ratelimit.go`) is in-memory, bounded, and proxy-aware — client IP comes from `X-Forwarded-For` only when `TRUST_PROXY` is true (the default; set to `false` if the app is ever reachable directly, since otherwise a client could spoof its way past the limiter).

**Templates**: `templ` files under `internal/templates/` compile to Go; HTMX handles interactivity (partial swaps, out-of-band updates for things like the "no accounts yet" empty state) with no client-side framework. `internal/providers` and the account form's Owner field both use a suggestion pattern instead of a fixed enum/lookup table: a static Go list for provider names (autocomplete only, not a constraint), and `accounts.Store.DistinctOwners()` (grouped `COLLATE NOCASE` to dedupe case-insensitively) for owner names — either can freely accept new values the UI has never seen before.

See `SECURITY.md` for the full security posture and what's intentionally deferred (e.g. plaintext SMTP/Gmail credentials at rest, no multi-user support) versus what's still recommended before wider exposure.
