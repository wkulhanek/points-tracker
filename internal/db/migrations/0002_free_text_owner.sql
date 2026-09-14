-- Owner started as a fixed enum (Wolfgang / Barbara / Joint) enforced by a
-- CHECK constraint. It's now free text, suggested (not restricted) via a
-- dropdown of previously-used names in the UI — so a household can add a
-- new owner just by typing their name once. SQLite has no ALTER TABLE...DROP
-- CONSTRAINT, so the CHECK is removed via the standard rebuild-and-rename
-- procedure, preserving all existing rows and ids.

CREATE TABLE accounts_new (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    provider        TEXT NOT NULL,
    account_number  TEXT,
    points_balance  INTEGER NOT NULL DEFAULT 0,
    expiration_date TEXT NOT NULL,
    does_not_expire INTEGER NOT NULL DEFAULT 0 CHECK (does_not_expire IN (0, 1)),
    owner           TEXT NOT NULL DEFAULT 'Joint',
    notes           TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO accounts_new (id, name, provider, account_number, points_balance, expiration_date, does_not_expire, owner, notes, created_at, updated_at)
    SELECT id, name, provider, account_number, points_balance, expiration_date, does_not_expire, owner, notes, created_at, updated_at
    FROM accounts;

DROP TABLE accounts;
ALTER TABLE accounts_new RENAME TO accounts;

CREATE INDEX idx_accounts_expiration_date ON accounts(expiration_date);
