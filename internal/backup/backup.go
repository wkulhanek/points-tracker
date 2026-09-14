// Package backup periodically snapshots the SQLite database to a
// backups/ subdirectory of the data directory. It uses SQLite's own
// `VACUUM INTO` statement rather than shelling out to the sqlite3 CLI,
// since the container image is distroless and has neither a shell nor that
// binary available.
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	tickInterval  = 24 * time.Hour
	retainCount   = 14
	backupDirName = "backups"
	filePrefix    = "points-"
	fileSuffix    = ".db"
	fileLayout    = "20060102-150405"
)

type Scheduler struct {
	db      *sql.DB
	dataDir string
}

func NewScheduler(db *sql.DB, dataDir string) *Scheduler {
	return &Scheduler{db: db, dataDir: dataDir}
}

// Run blocks, snapshotting immediately and then every 24h, until ctx is
// cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.runOnce()

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce()
		}
	}
}

func (s *Scheduler) runOnce() {
	if err := s.Snapshot(); err != nil {
		slog.Error("backup: snapshot failed", "error", err)
		return
	}
	if err := s.prune(); err != nil {
		slog.Error("backup: prune old snapshots failed", "error", err)
	}
}

// Snapshot takes one consistent point-in-time copy of the database.
func (s *Scheduler) Snapshot() error {
	dir := filepath.Join(s.dataDir, backupDirName)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create backups dir: %w", err)
	}

	path := filepath.Join(dir, filePrefix+time.Now().UTC().Format(fileLayout)+fileSuffix)

	// VACUUM INTO requires the destination not to already exist.
	if _, err := os.Stat(path); err == nil {
		return nil // a snapshot for this timestamp already exists (e.g. re-run within the same second)
	}

	// SQLite doesn't accept bound parameters for the destination of VACUUM
	// INTO; the path is derived entirely from our own dataDir/timestamp, not
	// user input, so building the statement by hand is safe here.
	if _, err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", path)); err != nil {
		return fmt.Errorf("vacuum into %s: %w", path, err)
	}

	// Snapshots contain session hashes and stored email credentials; keep
	// them readable only by the process owner regardless of umask.
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod snapshot %s: %w", path, err)
	}

	slog.Info("backup: snapshot written", "path", path)
	return nil
}

// prune keeps only the most recent retainCount snapshots.
func (s *Scheduler) prune() error {
	dir := filepath.Join(s.dataDir, backupDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // timestamp-formatted filenames sort chronologically

	if len(names) <= retainCount {
		return nil
	}
	for _, name := range names[:len(names)-retainCount] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("remove old snapshot %s: %w", name, err)
		}
	}
	return nil
}
