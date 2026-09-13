// Package webassets embeds the built CSS and vendored HTMX JS directly into
// the compiled binary, so the final container image is just the one static
// binary — no separate asset directory needs to be copied alongside it.
package webassets

import "embed"

//go:embed static
var FS embed.FS
