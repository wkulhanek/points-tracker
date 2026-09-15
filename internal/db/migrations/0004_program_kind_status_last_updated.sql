-- Adds a free-text status (e.g. loyalty tier), a fixed program-kind enum,
-- and a points-balance "last updated" timestamp to accounts.
-- points_updated_at tracks only actual points_balance changes (see
-- internal/accounts/service.go), not every edit, so it starts out
-- backfilled from the existing updated_at (the closest available
-- approximation for pre-existing rows) rather than the current time.
--
-- Unlike migrations 0002/0003, no create-copy-drop-rename rebuild is
-- needed here: SQLite's ALTER TABLE ADD COLUMN supports a CHECK constraint
-- as long as its default is a constant (a non-constant default like
-- datetime('now') is rejected, hence the separate backfill UPDATE below).

ALTER TABLE accounts ADD COLUMN status TEXT NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN kind TEXT NOT NULL DEFAULT 'Other' CHECK (kind IN ('Airline', 'Hotel', 'Credit Card', 'Other'));
ALTER TABLE accounts ADD COLUMN points_updated_at TEXT NOT NULL DEFAULT '';

UPDATE accounts SET points_updated_at = updated_at;
