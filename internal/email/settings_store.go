package email

import "database/sql"

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
	var smtpHost, smtpUser, smtpPass, smtpSecurity, smtpFrom sql.NullString
	var smtpPort sql.NullInt64

	err := s.db.QueryRow(`
		SELECT provider_type,
		       smtp_host, smtp_port, smtp_username, smtp_password, smtp_security, smtp_from_address
		FROM email_settings WHERE id = 1`).Scan(
		&providerType,
		&smtpHost, &smtpPort, &smtpUser, &smtpPass, &smtpSecurity, &smtpFrom,
	)
	if err != nil {
		return Settings{}, err
	}

	st.ProviderType = ProviderType(providerType)
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
