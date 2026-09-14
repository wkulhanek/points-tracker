-- Gmail OAuth support was removed (SMTP only now). Drops the gmail_*
-- columns and narrows the provider_type CHECK accordingly. SQLite has no
-- ALTER TABLE...DROP COLUMN combined with changing a CHECK constraint, so
-- the CHECK is removed via the standard rebuild-and-rename procedure,
-- preserving all existing rows and ids. email_settings has no incoming
-- foreign-key references, so the rebuild's DROP TABLE can't cascade-delete
-- anything else.

CREATE TABLE email_settings_new (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    provider_type       TEXT NOT NULL DEFAULT 'none' CHECK (provider_type IN ('none', 'smtp')),

    smtp_host           TEXT,
    smtp_port           INTEGER,
    smtp_username        TEXT,
    smtp_password        TEXT,
    smtp_security         TEXT CHECK (smtp_security IN ('none', 'starttls', 'tls')),
    smtp_from_address     TEXT,

    updated_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO email_settings_new (id, provider_type, smtp_host, smtp_port, smtp_username, smtp_password, smtp_security, smtp_from_address, updated_at)
    SELECT id,
           CASE WHEN provider_type = 'gmail' THEN 'none' ELSE provider_type END,
           smtp_host, smtp_port, smtp_username, smtp_password, smtp_security, smtp_from_address, updated_at
    FROM email_settings;

DROP TABLE email_settings;
ALTER TABLE email_settings_new RENAME TO email_settings;
