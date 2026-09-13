package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wkulhanek/kulhanek-points-tracker/internal/db/migrations"
)

// Migrate applies any embedded migration files that haven't been recorded in
// the schema_migrations table yet, in ascending numeric order. Each
// migration runs in its own transaction.
func Migrate(conn *sql.DB) error {
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := map[int]bool{}
	rows, err := conn.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("query schema_migrations: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	type migration struct {
		version  int
		filename string
	}
	var pending []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			return fmt.Errorf("migration filename %q missing version prefix", e.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return fmt.Errorf("migration filename %q has non-numeric version prefix: %w", e.Name(), err)
		}
		if applied[version] {
			continue
		}
		pending = append(pending, migration{version: version, filename: e.Name()})
	}

	sort.Slice(pending, func(i, j int) bool { return pending[i].version < pending[j].version })

	for _, m := range pending {
		sqlBytes, err := migrations.FS.ReadFile(m.filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", m.filename, err)
		}

		tx, err := conn.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration %s: %w", m.filename, err)
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", m.filename, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.filename, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.filename, err)
		}
	}

	return nil
}
