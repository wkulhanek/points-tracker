# Points Tracker

A small, self-hosted app for tracking loyalty/rewards points accounts (airline
miles, credit card points, hotel points, ...) and getting emailed before they
expire.

- Track any number of accounts, including multiple accounts from the same
  provider (e.g. two Chase accounts).
- Update an account's points balance and expiration date in place.
- Get notified by email a configurable number of days before an account's
  points expire (30 and 7 days by default) — renewing/updating an account's
  expiration date automatically re-arms its notifications.
- Send notifications via a connected Gmail account (OAuth, no app passwords)
  or any SMTP server.
- Single built-in admin login — no external identity provider required.
- All data in a single SQLite file; automatic daily backups.

## Tech stack

Go, [templ](https://templ.guide) for server-rendered HTML, [htmx](https://htmx.org)
for interactivity, and [Tailwind CSS](https://tailwindcss.com) for styling — one
language, one UI framework, no SPA build pipeline. Data lives in SQLite via the
pure-Go [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) driver (no
CGO, so the final binary and container image are fully static).

## Local development

Requires Go 1.27+ and Node.js (only used to run `npx tailwindcss` — there's no
JS app, no `node_modules` to maintain).

```sh
cp .env.example .env    # then edit it
set -a; source .env; set +a
make run
```

`make run` regenerates the templ Go files, rebuilds the Tailwind CSS bundle,
and starts the server on `:8080` (`PORT` to change it). The admin account is
seeded from `ADMIN_USERNAME`/`ADMIN_PASSWORD` the first time it boots against
an empty database — changing those env vars afterwards has no effect.

Other targets: `make build` (produces `bin/server`), `make test`, `make vet`,
`make generate` (templ only), `make tailwind` (CSS only).

### Environment variables

| Variable | Required | Description |
|---|---|---|
| `PORT` | no (default `8080`) | HTTP listen port. Plain HTTP only — put a TLS-terminating reverse proxy in front for real deployments. |
| `DATA_DIR` | no (default `./data`) | Directory for the SQLite database and its `backups/` subdirectory. Must be writable. |
| `BASE_URL` | no (default `http://localhost:8080`) | This app's externally-reachable URL; used to build the Gmail OAuth redirect URL. |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | **yes** | Seeds the single admin account on first boot only. |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | no | Enables the "Connect Gmail" option under Email Settings. Omit both to hide it and use SMTP only. |

Everything else — app display name, timezone, notification thresholds,
notification recipient(s), which email provider is active — is stored in the
database and edited from the UI (Preferences / Email Settings pages), so it
survives without needing a restart or redeploy.

### Setting up Gmail OAuth (optional)

1. In [Google Cloud Console](https://console.cloud.google.com/apis/credentials),
   create an OAuth 2.0 Client ID of type **Web application**.
2. Add an authorized redirect URI: `<BASE_URL>/oauth/google/callback` (e.g.
   `https://points.example.com/oauth/google/callback`).
3. Set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` from that client.
4. Restart the app, then click **Connect Gmail** on the Email Settings page
   and sign in. This one-time app registration step is the only manual part —
   from then on, connecting/reconnecting Gmail is just the normal Google
   consent screen redirect, no separate credential handling needed.

The app only ever requests the `gmail.send` scope — it can send mail through
the connected account but never reads the mailbox.

## Deployment

### Container image

`Containerfile` is a 3-stage build (Tailwind CSS → Go build →
[distroless](https://github.com/GoogleContainerTools/distroless) static
nonroot runtime) producing a single static binary with no shell, package
manager, or dynamic libraries in the final image.

```sh
podman build -t quay.io/wkulhanek/points-tracker -f Containerfile .
```

### CI

`.github/workflows/build-push.yml` runs `go vet`/`go test` on every push, and
on pushes to `main` or `v*` tags, builds and pushes multi-arch
(`linux/amd64`, `linux/arm64`) images to
`quay.io/wkulhanek/points-tracker`. Requires two repo secrets:
`QUAY_USERNAME` and `QUAY_ROBOT_TOKEN` (create a robot account under the
quay.io repository's Settings → Robot Accounts, grant it write access).

### Running on Fedora 44+ (Podman Quadlet)

See `deploy/quadlet/points-tracker.container` (installation steps are
in a comment at the top of that file) and `deploy/quadlet/env.example` for the
`EnvironmentFile`. In short:

```sh
sudo mkdir -p /etc/containers/systemd /etc/points-tracker
sudo cp deploy/quadlet/points-tracker.container /etc/containers/systemd/
sudo cp deploy/quadlet/env.example /etc/points-tracker/env   # then edit it
sudo mkdir -p /var/lib/points-tracker
sudo chown 65532:65532 /var/lib/points-tracker
sudo systemctl daemon-reload
sudo systemctl start points-tracker.service
```

The unit publishes the app to `127.0.0.1:8080` only — front it with the
reverse proxy you already run on the host.

### Backups

A background task snapshots the SQLite database (via `VACUUM INTO`, so it's
always internally consistent) into `$DATA_DIR/backups/` once a day, keeping
the most recent 14 snapshots. No separate cron job needed.

## Project layout

```
cmd/server/            main.go — wires everything together and starts the server
internal/accounts/     accounts domain (CRUD + notification-reset-on-renewal logic)
internal/auth/         session-cookie login, bcrypt, login rate limiting
internal/backup/       daily SQLite snapshot scheduler
internal/config/       env var loading
internal/db/           SQLite connection + embedded migrations
internal/email/        provider-agnostic Sender interface + settings storage
internal/email/gmail/  Gmail OAuth + send-via-Gmail-API
internal/email/smtp/   plain SMTP send
internal/email/factory/  picks the active provider's Sender
internal/notifications/  pure evaluator (unit tested) + hourly scheduler
internal/preferences/  app name, timezone, thresholds, recipients
internal/templates/    templ views (compiled to Go via `make generate`)
internal/web/          HTTP handlers + router
web/                    Tailwind input/config + vendored htmx + go:embed of static/
deploy/quadlet/         Podman Quadlet unit for Fedora
```

## Design notes / known simplifications

- Single admin user, no multi-user support or account management UI.
- Gmail/SMTP credentials are stored in SQLite in plaintext — acceptable for a
  single-user, self-hosted app behind your own reverse proxy, but don't
  expose the data directory.
- No explicit CSRF token middleware; relies on `SameSite=Lax` session cookies.
- The notification scheduler checks hourly (not once a day) — cheap and
  idempotent, so it self-corrects quickly after a restart instead of
  possibly missing a once-daily window.
