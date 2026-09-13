package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
)

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = c.Render(r.Context(), w)
}

// flashFromQuery reads a one-shot flash message passed via query string
// after a redirect (see redirectWithFlash). Simple and stateless — no
// server-side session storage needed for something this transient.
func flashFromQuery(r *http.Request) (kind, msg string) {
	return r.URL.Query().Get("kind"), r.URL.Query().Get("msg")
}

func redirectWithFlash(w http.ResponseWriter, r *http.Request, path, kind, msg string) {
	u := path + "?" + url.Values{"kind": {kind}, "msg": {msg}}.Encode()
	http.Redirect(w, r, u, http.StatusSeeOther)
}

func splitEmails(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if e := strings.TrimSpace(p); e != "" {
			out = append(out, e)
		}
	}
	return out
}
