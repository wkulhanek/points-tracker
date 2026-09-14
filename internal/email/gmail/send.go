package gmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/wkulhanek/points-tracker/internal/email"
)

// Sender implements email.Sender by sending through the Gmail API using
// the currently stored OAuth refresh token.
type Sender struct {
	oauth         *OAuth
	settingsStore *email.SettingsStore
}

func NewSender(oauth *OAuth, settingsStore *email.SettingsStore) *Sender {
	return &Sender{oauth: oauth, settingsStore: settingsStore}
}

func (s *Sender) Send(ctx context.Context, to []string, subject, body string) error {
	st, err := s.settingsStore.Get()
	if err != nil {
		return fmt.Errorf("load email settings: %w", err)
	}
	if st.GmailRefreshToken == "" {
		return email.ErrNoProvider
	}

	ts := s.oauth.TokenSource(ctx, st.GmailRefreshToken)
	svc, err := gmailapi.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return fmt.Errorf("build gmail service: %w", err)
	}

	from := st.GmailAccountEmail
	raw := buildRawMessage(from, to, subject, body)
	if _, err := svc.Users.Messages.Send("me", &gmailapi.Message{Raw: raw}).Do(); err != nil {
		return fmt.Errorf("send via gmail api: %w", err)
	}

	// Best-effort: persist the (possibly refreshed) access token so future
	// sends can skip a round trip. Failure here doesn't affect the send that
	// just succeeded.
	if tok, tokErr := ts.Token(); tokErr == nil {
		_ = s.settingsStore.UpdateGmailAccessToken(tok.AccessToken, tok.Expiry)
	}

	return nil
}

func buildRawMessage(from string, to []string, subject, body string) string {
	sanitizedTo := make([]string, len(to))
	for i, addr := range to {
		sanitizedTo[i] = sanitizeHeader(addr)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", sanitizeHeader(from))
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(sanitizedTo, ", "))
	fmt.Fprintf(&buf, "Subject: %s\r\n", sanitizeHeader(subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)
	return base64.URLEncoding.EncodeToString(buf.Bytes())
}

// sanitizeHeader strips CR/LF so a crafted address or subject can't inject
// additional mail headers (SMTP header injection).
func sanitizeHeader(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}
