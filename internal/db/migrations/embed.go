// Package migrations embeds the raw SQL migration files so they ship inside
// the compiled binary — no separate files need to be present on disk at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
