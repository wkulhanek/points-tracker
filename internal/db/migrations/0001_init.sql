-- Initial schema for Kulhanek Points Tracker.

CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL,
    user_agent TEXT,
    ip_address TEXT
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE accounts (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    provider        TEXT NOT NULL,
    account_number  TEXT,
    points_balance  INTEGER NOT NULL DEFAULT 0,
    expiration_date TEXT NOT NULL,
    notes           TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_accounts_expiration_date ON accounts(expiration_date);

CREATE TABLE notification_thresholds (
    id          INTEGER PRIMARY KEY,
    days_before INTEGER NOT NULL UNIQUE,
    enabled     INTEGER NOT NULL DEFAULT 1
);
INSERT INTO notification_thresholds (days_before) VALUES (30), (7);

CREATE TABLE sent_notifications (
    id                       INTEGER PRIMARY KEY,
    account_id               INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    threshold_id              INTEGER NOT NULL REFERENCES notification_thresholds(id) ON DELETE CASCADE,
    notified_expiration_date TEXT NOT NULL,
    sent_at                   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (account_id, threshold_id, notified_expiration_date)
);
CREATE INDEX idx_sent_notifications_account_id ON sent_notifications(account_id);

-- Singleton row (id is always 1) holding whichever email provider is active.
-- Gmail's OAuth client id/secret are NOT stored here — they come from env vars.
CREATE TABLE email_settings (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    provider_type       TEXT NOT NULL DEFAULT 'none' CHECK (provider_type IN ('none', 'gmail', 'smtp')),

    gmail_refresh_token TEXT,
    gmail_access_token  TEXT,
    gmail_token_expiry  TEXT,
    gmail_account_email TEXT,

    smtp_host           TEXT,
    smtp_port           INTEGER,
    smtp_username        TEXT,
    smtp_password        TEXT,
    smtp_security         TEXT CHECK (smtp_security IN ('none', 'starttls', 'tls')),
    smtp_from_address     TEXT,

    updated_at           TEXT NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO email_settings (id, provider_type) VALUES (1, 'none');

-- Singleton row (id is always 1) holding app-wide preferences.
CREATE TABLE app_preferences (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    app_display_name TEXT NOT NULL DEFAULT 'Kulhanek Points Tracker',
    timezone          TEXT NOT NULL DEFAULT 'Europe/Vienna',
    recipient_emails TEXT NOT NULL DEFAULT '',
    updated_at        TEXT NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO app_preferences (id) VALUES (1);
