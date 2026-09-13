// Package db opens the application's SQLite database and applies migrations.
package db

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (creating if necessary) the SQLite database file at
// <dataDir>/points.db, configures it for concurrent access, and applies any
// pending schema migrations.
func Open(dataDir string) (*sql.DB, error) {
	dbPath := filepath.Join(dataDir, "points.db")

	// modernc.org/sqlite accepts these as query-string pragmas on the DSN so
	// every connection in the pool gets them consistently.
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)", dbPath)

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite only supports one writer at a time; a single connection avoids
	// SQLITE_BUSY errors under concurrent requests without needing an
	// external connection pool/lock.
	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := Migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return conn, nil
}
