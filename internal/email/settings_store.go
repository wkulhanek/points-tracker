package email

import (
	"database/sql"
	"time"
)

type SettingsStore struct {
	db *sql.DB
}

func NewSettingsStore(db *sql.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

// Get returns the current email settings singleton.
func (s *SettingsStore) Get() (Settings, error) {
	var st Settings
	var providerType string
	var gmailRefresh, gmailAccess, gmailExpiry, gmailEmail sql.NullString
	var smtpHost, smtpUser, smtpPass, smtpSecurity, smtpFrom sql.NullString
	var smtpPort sql.NullInt64

	err := s.db.QueryRow(`
		SELECT provider_type,
		       gmail_refresh_token, gmail_access_token, gmail_token_expiry, gmail_account_email,
		       smtp_host, smtp_port, smtp_username, smtp_password, smtp_security, smtp_from_address
		FROM email_settings WHERE id = 1`).Scan(
		&providerType,
		&gmailRefresh, &gmailAccess, &gmailExpiry, &gmailEmail,
		&smtpHost, &smtpPort, &smtpUser, &smtpPass, &smtpSecurity, &smtpFrom,
	)
	if err != nil {
		return Settings{}, err
	}

	st.ProviderType = ProviderType(providerType)
	st.GmailRefreshToken = gmailRefresh.String
	st.GmailAccessToken = gmailAccess.String
	st.GmailAccountEmail = gmailEmail.String
	if gmailExpiry.Valid && gmailExpiry.String != "" {
		if t, err := time.Parse(time.RFC3339, gmailExpiry.String); err == nil {
			st.GmailTokenExpiry = t
		}
	}

	st.SMTPHost = smtpHost.String
	st.SMTPPort = int(smtpPort.Int64)
	st.SMTPUsername = smtpUser.String
	st.SMTPPassword = smtpPass.String
	st.SMTPSecurity = SMTPSecurity(smtpSecurity.String)
	st.SMTPFromAddress = smtpFrom.String

	return st, nil
}

// SaveSMTP stores SMTP settings and makes SMTP the active provider.
func (s *SettingsStore) SaveSMTP(host string, port int, username, password string, security SMTPSecurity, from string) error {
	_, err := s.db.Exec(`
		UPDATE email_settings
		SET provider_type = 'smtp', smtp_host = ?, smtp_port = ?, smtp_username = ?, smtp_password = ?,
		    smtp_security = ?, smtp_from_address = ?, updated_at = datetime('now')
		WHERE id = 1`,
		host, port, username, password, string(security), from)
	return err
}

// SaveGmailTokens stores the result of a completed OAuth exchange (or a
// refreshed access token) and makes Gmail the active provider.
func (s *SettingsStore) SaveGmailTokens(refreshToken, accessToken string, expiry time.Time, accountEmail string) error {
	_, err := s.db.Exec(`
		UPDATE email_settings
		SET provider_type = 'gmail', gmail_refresh_token = ?, gmail_access_token = ?, gmail_token_expiry = ?,
		    gmail_account_email = ?, updated_at = datetime('now')
		WHERE id = 1`,
		refreshToken, accessToken, expiry.UTC().Format(time.RFC3339), accountEmail)
	return err
}

// UpdateGmailAccessToken persists a refreshed access token without touching
// the refresh token or provider selection; called opportunistically after
// each send.
func (s *SettingsStore) UpdateGmailAccessToken(accessToken string, expiry time.Time) error {
	_, err := s.db.Exec(`
		UPDATE email_settings SET gmail_access_token = ?, gmail_token_expiry = ?, updated_at = datetime('now') WHERE id = 1`,
		accessToken, expiry.UTC().Format(time.RFC3339))
	return err
}

// DisconnectGmail clears stored Gmail tokens and, if Gmail was the active
// provider, resets it to none.
func (s *SettingsStore) DisconnectGmail() error {
	_, err := s.db.Exec(`
		UPDATE email_settings
		SET gmail_refresh_token = NULL, gmail_access_token = NULL, gmail_token_expiry = NULL, gmail_account_email = NULL,
		    provider_type = CASE WHEN provider_type = 'gmail' THEN 'none' ELSE provider_type END,
		    updated_at = datetime('now')
		WHERE id = 1`)
	return err
}
