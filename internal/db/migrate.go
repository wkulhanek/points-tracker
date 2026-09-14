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
//
// Foreign key enforcement is switched off for the duration (SQLite only
// allows that outside of an active transaction, hence doing it here rather
// than as a statement inside a migration file). This matters because some
// migrations rebuild a table via SQLite's standard
// create-copy-drop-rename procedure (there's no ALTER TABLE...DROP
// CONSTRAINT) — with foreign_keys ON, dropping a table cascades
// ON DELETE CASCADE against every other table that references it, silently
// wiping unrelated data the migration never intended to touch. A
// foreign_key_check after all migrations run catches any real integrity
// problem the pragma toggle would otherwise have masked.
func Migrate(conn *sql.DB) error {
	if _, err := conn.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		return fmt.Errorf("disable foreign keys for migration: %w", err)
	}
	// Restore enforcement for the rest of the process's lifetime no matter
	// how this function returns — Open() keeps this one connection alive
	// for as long as the app runs (see db.go's SetMaxOpenConns(1)), so
	// nothing else re-asserts the pragma later.
	defer conn.Exec(`PRAGMA foreign_keys=ON`) //nolint:errcheck // best-effort; a PRAGMA on a healthy connection essentially can't fail

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

	// foreign_keys was OFF for the rebuild-heavy migrations above, so
	// nothing caught a real integrity problem along the way (e.g. a
	// migration that accidentally drops rows a live FK should have
	// protected). Check now, before handing enforcement back on unlock.
	if violations, err := foreignKeyViolations(conn); err != nil {
		return fmt.Errorf("check foreign key integrity after migration: %w", err)
	} else if len(violations) > 0 {
		return fmt.Errorf("migration left %d foreign key violation(s): %v", len(violations), violations)
	}

	return nil
}

// foreignKeyViolations runs PRAGMA foreign_key_check and returns a
// human-readable description of each violation found, if any.
func foreignKeyViolations(conn *sql.DB) ([]string, error) {
	rows, err := conn.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var table, fkid, parent string
		var rowid sql.NullInt64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("table=%s rowid=%v references=%s fkid=%s", table, rowid, parent, fkid))
	}
	return out, rows.Err()
}
