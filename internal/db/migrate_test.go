package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/wkulhanek/points-tracker/internal/db/migrations"
)

// TestMigrateDoesNotCascadeDeleteAcrossTableRebuilds guards against a class
// of bug found in migration 0002 (fixed alongside this test): a migration
// that rebuilds a table via SQLite's create-copy-drop-rename pattern (used
// because SQLite has no ALTER TABLE...DROP CONSTRAINT) drops the old table
// as part of that rebuild. If foreign_keys enforcement is left ON across
// that DROP, any other table with an ON DELETE CASCADE reference to it — in
// this schema, sent_notifications -> accounts — has its rows silently
// deleted too, even though the migration never intended to touch them and
// the rebuilt table's own data comes through fine. Migrate must switch
// foreign_keys OFF for the duration of applying migrations specifically to
// prevent this.
func TestMigrateDoesNotCascadeDeleteAcrossTableRebuilds(t *testing.T) {
	dir := t.TempDir()
	conn, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "test.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)

	// Bootstrap straight to the original (pre-0002) schema by applying
	// 0001 directly and recording it as already-applied, so the real
	// Migrate() call below only has 0002 (and anything added later) left
	// to run — reproducing the exact "upgrading an existing install"
	// scenario the bug occurred in.
	initSQL, err := migrations.FS.ReadFile("0001_init.sql")
	if err != nil {
		t.Fatalf("read 0001_init.sql: %v", err)
	}
	if _, err := conn.Exec(string(initSQL)); err != nil {
		t.Fatalf("apply 0001_init.sql: %v", err)
	}
	if _, err := conn.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')))`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO schema_migrations (version) VALUES (1)`); err != nil {
		t.Fatalf("mark 0001 applied: %v", err)
	}

	if _, err := conn.Exec(`INSERT INTO accounts (name, provider, expiration_date, owner) VALUES ('Test', 'Chase', '2027-01-01', 'Wolfgang')`); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO sent_notifications (account_id, threshold_id, notified_expiration_date) VALUES (1, 1, '2027-01-01')`); err != nil {
		t.Fatalf("insert sent_notifications: %v", err)
	}

	if err := Migrate(conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var accountCount, notifCount int
	if err := conn.QueryRow(`SELECT count(*) FROM accounts`).Scan(&accountCount); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if err := conn.QueryRow(`SELECT count(*) FROM sent_notifications`).Scan(&notifCount); err != nil {
		t.Fatalf("count sent_notifications: %v", err)
	}

	if accountCount != 1 {
		t.Errorf("accounts survived rebuild: got %d rows, want 1", accountCount)
	}
	if notifCount != 1 {
		t.Errorf("sent_notifications did NOT survive the accounts table rebuild: got %d rows, want 1 (this is the exact bug this test guards against)", notifCount)
	}

	// Enforcement must be back on afterward for normal runtime operation.
	var fkEnabled int
	if err := conn.QueryRow(`PRAGMA foreign_keys`).Scan(&fkEnabled); err != nil {
		t.Fatalf("query foreign_keys pragma: %v", err)
	}
	if fkEnabled != 1 {
		t.Errorf("foreign_keys enforcement not restored after Migrate: got %d, want 1", fkEnabled)
	}
}
