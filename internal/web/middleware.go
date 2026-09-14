package web

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/wkulhanek/points-tracker/internal/auth"
)

// maxBodyBytes caps every request body. The app only ever accepts small
// HTML forms, so 1 MiB is generous; without a cap a client could stream an
// unbounded body into memory (and, for multipart forms, onto disk).
const maxBodyBytes = 1 << 20

// securityHeaders applies a conservative baseline of response headers to
// every response. Static assets are exempted from Cache-Control so they can
// still be cached by the browser.
//
// CSP note: script-src/style-src allow 'unsafe-inline' because the
// templates use a couple of inline event handlers and HTMX injects a small
// inline <style>. Removing those (moving handlers into a static .js file)
// would let this drop to a strict script-src 'self' — tracked as a
// follow-up in SECURITY.md.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'self'",
			"script-src 'self' 'unsafe-inline'",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data:",
			"connect-src 'self'",
			"base-uri 'self'",
			"form-action 'self'",
			"frame-ancestors 'none'",
			"object-src 'none'",
		}, "; "))

		// HSTS is only meaningful (and only safe to send) when the browser
		// actually reached us over HTTPS, which behind the proxy is signalled
		// by X-Forwarded-Proto.
		if auth.IsSecureRequest(r) {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}

		// Authenticated pages and the login form must never be stored by the
		// browser or an intermediary.
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			h.Set("Cache-Control", "no-store")
			h.Set("Pragma", "no-cache")
		}

		next.ServeHTTP(w, r)
	})
}

// limitBody bounds the size of every request body.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// requireSameOrigin is defense-in-depth against CSRF on top of the
// SameSite=Lax session cookie. It rejects state-changing requests whose
// Origin (or, if absent, Referer) does not match the host the browser used
// to reach us. Requests with neither header are allowed through: modern
// browsers always send Origin on unsafe methods, and the SameSite=Lax
// cookie already blocks cross-site form POSTs.
//
// The allowed hosts are the request's own Host header (which a browser
// cannot forge for a cross-origin request) and the operator-configured
// BASE_URL. X-Forwarded-Host is deliberately NOT trusted here, since a
// browser can set it via fetch().
func requireSameOrigin(next http.Handler, baseURL string) http.Handler {
	baseHost := ""
	if u, err := url.Parse(baseURL); err == nil {
		baseHost = u.Hostname()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" {
			if !hostMatches(origin, r.Host, baseHost) {
				http.Error(w, "request blocked: cross-origin form submission", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if ref := r.Header.Get("Referer"); ref != "" {
			if !hostMatches(ref, r.Host, baseHost) {
				http.Error(w, "request blocked: cross-origin form submission", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// hostMatches reports whether the URL in raw (an Origin or Referer value)
// points at requestHost or baseHost.
func hostMatches(raw, requestHost, baseHost string) bool {
	if raw == "null" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	got := u.Hostname()
	for _, candidate := range []string{requestHost, baseHost} {
		if candidate == "" {
			continue
		}
		if strings.EqualFold(got, stripPort(candidate)) {
			return true
		}
	}
	return false
}

// stripPort removes a ":port" suffix from host, if present. It handles a
// bracketed IPv6 literal with a port (e.g. "[::1]:8080") correctly via
// net.SplitHostPort rather than a bare last-colon split, which would
// otherwise leave the brackets/port in place and never match the
// bracket-free, port-free value url.URL.Hostname() returns for the same
// address — causing a same-origin check to reject a legitimate request.
func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
