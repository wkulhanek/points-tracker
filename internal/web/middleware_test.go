package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/login", nil)
	req.Host = "example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
		t.Error("Strict-Transport-Security missing for HTTPS request")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestSecurityHeadersNoHSTSOverHTTP(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "http://example.com/login", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("Strict-Transport-Security = %q, want empty over plain HTTP", got)
	}
}

func TestSameOriginRejectsCrossSitePost(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := requireSameOrigin(next, "")

	req := httptest.NewRequest(http.MethodPost, "http://example.com/accounts", strings.NewReader("x=1"))
	req.Host = "example.com"
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestSameOriginAllowsMatchingOrigin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := requireSameOrigin(next, "")

	req := httptest.NewRequest(http.MethodPost, "http://example.com/accounts", strings.NewReader("x=1"))
	req.Host = "example.com"
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSameOriginIgnoresSafeMethods(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := requireSameOrigin(next, "")

	req := httptest.NewRequest(http.MethodGet, "http://example.com/accounts", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d for GET", rec.Code, http.StatusOK)
	}
}

func TestSameOriginRejectsSpoofedForwardedHost(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := requireSameOrigin(next, "https://points.example.com")

	req := httptest.NewRequest(http.MethodPost, "http://points.example.com/accounts", strings.NewReader("x=1"))
	req.Host = "points.example.com"
	req.Header.Set("Origin", "https://evil.example")
	// A browser can set this via fetch(); it must not influence the check.
	req.Header.Set("X-Forwarded-Host", "evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestSameOriginAllowsConfiguredBaseURL(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := requireSameOrigin(next, "https://points.example.com")

	// Behind a proxy that rewrites Host to an internal name, BASE_URL still
	// authorizes the real external origin.
	req := httptest.NewRequest(http.MethodPost, "http://backend:8080/accounts", strings.NewReader("x=1"))
	req.Host = "backend:8080"
	req.Header.Set("Origin", "https://points.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestLimitBodyRejectsOversized(t *testing.T) {
	h := limitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
	}))

	// A body larger than the cap must trigger MaxBytesReader's error on read.
	big := strings.NewReader(strings.Repeat("a", maxBodyBytes+1))
	req := httptest.NewRequest(http.MethodPost, "http://example.com/", big)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
