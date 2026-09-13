// Package gmail implements the email.Sender interface on top of the Gmail
// API, authenticated via OAuth2 (gmail.send scope only — this app never
// reads the connected mailbox, just sends through it).
package gmail

import (
	"context"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// OAuth wraps the Google OAuth2 config needed to run the "Connect Gmail"
// consent flow and to build a token source for sending mail afterwards.
type OAuth struct {
	config *oauth2.Config
}

// NewOAuth builds the OAuth2 config. clientID/clientSecret come from the
// Google Cloud Console app registration (env vars, not user-editable);
// baseURL is this app's externally-reachable URL, used to build the
// callback redirect.
func NewOAuth(clientID, clientSecret, baseURL string) *OAuth {
	return &OAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  baseURL + "/oauth/google/callback",
			Scopes:       []string{gmailapi.GmailSendScope},
			Endpoint:     google.Endpoint,
		},
	}
}

// Configured reports whether a Client ID/Secret were provided at all — if
// not, the "Connect Gmail" option should be hidden/disabled in the UI.
func (o *OAuth) Configured() bool {
	return o.config.ClientID != "" && o.config.ClientSecret != ""
}

// AuthCodeURL builds the URL to redirect the admin to for consent.
// AccessTypeOffline + ApprovalForce are both required to reliably receive a
// refresh token back — Google only issues one on first consent, or when
// the consent screen is explicitly forced again.
func (o *OAuth) AuthCodeURL(state string) string {
	return o.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// Exchange trades an authorization code (from the callback redirect) for a
// token pair.
func (o *OAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return o.config.Exchange(ctx, code)
}

// TokenSource returns a token source that transparently refreshes the
// access token from the given refresh token as needed.
func (o *OAuth) TokenSource(ctx context.Context, refreshToken string) oauth2.TokenSource {
	return o.config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
}

// ProfileEmail fetches the connected Google account's address, used purely
// for display ("Connected as x@gmail.com") after the OAuth callback.
func ProfileEmail(ctx context.Context, ts oauth2.TokenSource) (string, error) {
	svc, err := gmailapi.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return "", err
	}
	profile, err := svc.Users.GetProfile("me").Do()
	if err != nil {
		return "", err
	}
	return profile.EmailAddress, nil
}
