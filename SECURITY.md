# Security analysis — Kulhanek Points Tracker

Scope: the Go web application in this repository, deployed behind a
TLS-terminating reverse proxy, single admin account, SQLite storage.

Threat model: the app is reachable from the public internet only through the
proxy; the proxy and host are trusted; the SQLite file, backups, and env file
are trusted-but-must-be-protected at rest. Primary risks are credential
brute force, session hijacking, CSRF/XSS/clickjacking, and secret leakage
from the database or backups.

This file records what was hardened in this pass and what is still
recommended. Items prefixed **[FIXED]** are implemented; the rest are
recommendations.

---

## 1. Fixed in this pass

### 1.1 Session tokens hashed at rest — `internal/auth/store.go`
Session tokens were stored verbatim in the `sessions` table. Anyone who read
the database (or any of the 14 on-disk backups) could replay a live session
for up to 30 days. Tokens are now SHA-256-hashed before storage; the raw token
only ever lives in the cookie. (SHA-256, not bcrypt, is correct here: the
token is 256 bits of CSPRNG output and is not brute-forceable.)

> **Upgrade note:** existing sessions are invalidated by this change — users
> must log in once after deploying.

### 1.2 CSRF defense in depth — `internal/web/middleware.go`
The app relied solely on `SameSite=Lax`. Added a `requireSameOrigin`
middleware that rejects unsafe methods (POST/PUT/PATCH/DELETE) whose `Origin`
(or `Referer`) does not match the request `Host` or the configured
`BASE_URL`. `X-Forwarded-Host` is deliberately **not** trusted, because a
browser can set it via `fetch()`; `BASE_URL` is used so the check still works
when the proxy rewrites `Host`.

### 1.3 Security response headers — `internal/web/middleware.go`
Added `X-Content-Type-Options`, `X-Frame-Options: DENY`,
`Referrer-Policy`, `Cross-Origin-Opener-Policy`,
`Cross-Origin-Resource-Policy`, a baseline `Content-Security-Policy`
(`frame-ancestors 'none'`, `object-src 'none'`, `base-uri 'self'`,
`form-action 'self'`), `Cache-Control: no-store` on non-static responses, and
`Strict-Transport-Security` when the request arrived over HTTPS.

### 1.4 Request body size limit — `internal/web/middleware.go`
Every body is wrapped in `http.MaxBytesReader` (1 MiB). Previously
`ParseForm`/`ParseMultipartForm` could buffer an arbitrarily large body,
a memory/disk exhaustion vector.

### 1.5 Server timeouts — `cmd/server/main.go`
Added `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, and `MaxHeaderBytes`
alongside the existing `ReadHeaderTimeout` to blunt slowloris-style resource
exhaustion.

### 1.6 Login rate limiting behind a proxy — `internal/web/auth_handlers.go`,
`internal/config/config.go`
`clientIP` used `RemoteAddr`, which behind a proxy is always the proxy
address — so five failed logins from anyone locked out *everyone*, a trivial
denial of service. It now takes the right-most `X-Forwarded-For` entry when
`TRUST_PROXY` is enabled (default true; set `TRUST_PROXY=false` if the app is
ever directly reachable). Rate-limiter memory is also bounded and periodically
swept (`internal/auth/ratelimit.go`, cleanup goroutine in `main.go`).

### 1.7 Email header injection — `internal/email/gmail/send.go`
`buildRawMessage` interpolated the subject and recipients into raw SMTP
headers without stripping CR/LF, allowing header injection. Header values are
now sanitized.

### 1.8 Reduced information disclosure — `internal/web/settings_email_handlers.go`
The "test email" error was echoed to the browser; it can contain SMTP
hostnames, internal IPs, and credential hints. It is now logged server-side
and the user gets a generic message. The OAuth `state` comparison now uses
`subtle.ConstantTimeCompare`.

### 1.9 Account-enumeration timing — `internal/auth/password.go`,
`internal/web/auth_handlers.go`
An unknown username skipped the bcrypt check (short-circuit), so timing
leaked whether an account existed. A dummy bcrypt hash is now always
compared.

### 1.10 Weak bootstrap password rejected — `internal/auth/admin_bootstrap.go`
`ADMIN_PASSWORD` must now be at least 12 characters on first boot.

### 1.11 Backup permissions — `internal/backup/backup.go`
Snapshots (which contain session hashes and stored email credentials) are
now `chmod 0600` regardless of umask.

### 1.12 Auth event logging
Login attempts are logged with username and client IP (successes at `Info`,
failures and rate-limit hits at `Warn`), giving an audit trail in the
container/journald logs.

---

## 2. Still recommended — application

| Priority | Item | Detail |
|----------|------|--------|
| High | Cookie prefix and forced Secure | Rename the cookie to `__Host-session` and always set `Secure` in production. `IsSecureRequest` currently trusts `X-Forwarded-Proto`; add a `SECURE_COOKIES=true` override so a proxy that forgets the header can't silently downgrade to plain HTTP. |
| High | Shorter sessions + rotation | 30-day absolute TTL with no idle timeout is long. Reduce to ~7 days idle / 30 days absolute, and rotate the session token on privilege change. |
| High | Password change / MFA | There is no way to rotate the admin password after first boot. Add a password-change flow and, ideally, TOTP second factor. |
| Medium | Strict CSP | Drop `script-src 'unsafe-inline'` by moving the two inline handlers in `accounts/form.templ` into `web/static/js/` and disabling HTMX's inline style injection. Then `script-src 'self'`. |
| Medium | Encrypt secrets at rest | SMTP password and Gmail refresh token are plaintext in SQLite. Encrypt with a key from the environment (e.g. `SECRET_KEY`) or rely on full-disk encryption and restrict the volume. |
| Medium | Versioned dependencies / scanning | Add `govulncheck` and Dependabot; `google.golang.org/api` and `oauth2` are large surfaces. |
| Low | CSRF tokens | `SameSite=Lax` + origin check is solid; per-session synchronizer tokens would cover exotic same-site/subdomain cases. |
| Low | Static directory listing | `GET /static/` may enumerate `css/` and `js/`. Serve explicit subtrees if you want to eliminate it. |
| Low | Flash-message spoofing | `/accounts?kind=error&msg=...` lets a crafted link render arbitrary (escaped) text in the UI. Cosmetic/phishing only. |
| Low | SMTP SSRF | The admin can point SMTP at internal hosts. Only relevant if the admin account is compromised. |

## 3. Still recommended — deployment / operations

- **Never expose the container directly.** Keep `PublishPort=127.0.0.1:8080:8080`
  (the Quadlet unit already does; `run.sh` publishes `0.0.0.0` — change it).
  With `TRUST_PROXY=true`, direct access would let clients spoof
  `X-Forwarded-For`.
- **Proxy must overwrite, not append, forwarding headers.** Set
  `X-Forwarded-Proto` and `X-Forwarded-For` from the connection, and clear
  any client-supplied values.
- **TLS:** modern protocols only (TLS 1.2+), HSTS at the proxy, and redirect
  HTTP→HTTPS.
- **Container hardening:** add `--read-only`, `--security-opt=no-new-privileges`,
  drop all capabilities, and mount `/tmp` as `tmpfs` in the Quadlet unit.
  (The image is already distroless + non-root.)
- **Secrets:** prefer Podman/systemd secrets over a world-readable
  `EnvironmentFile`, or at minimum `chmod 600 /etc/kulhanek-points-tracker/env`.
- **Off-site backups:** the local snapshots share the volume with the DB; copy
  them somewhere else and encrypt them.
- **CI supply chain:** GitHub Actions are pinned to major tags (`@v4`), not
  commit SHAs. Pin SHAs and enable CodeQL.
- **File permissions:** verify the data volume is owned by uid 65532 with mode
  `0700` on the host.

---

## 4. Example proxy hardening (nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name points.example.com;

    ssl_protocols TLSv1.2 TLSv1.3;

    # Overwrite (never forward) client-supplied forwarding headers.
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header Host              $host;

    # Request-size cap as a second line of defense.
    client_max_body_size 1m;

    location / {
        proxy_pass http://127.0.0.1:8080;
    }
}
```

---

## 5. Verification

```
go vet ./...
go test ./...
gofmt -l .
```

The new middleware and origin logic are covered by
`internal/web/middleware_test.go` (security headers, HSTS gating, same-origin
allow/deny, spoofed `X-Forwarded-Host`, `BASE_URL` fallback, and body limit).
