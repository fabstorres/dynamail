package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fabstorres/dynamail/apps/api/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type OAuthService interface {
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	Client(ctx context.Context, token *oauth2.Token) *http.Client
	Revoke(ctx context.Context, token *oauth2.Token) error
}

type GoogleOAuthService struct {
	client *oauth2.Config
}

func NewGoogleOAuthService(cfg config.Config) *GoogleOAuthService {
	return &GoogleOAuthService{client: &oauth2.Config{
		ClientID:     cfg.GoogleOAuthClientID,
		ClientSecret: cfg.GoogleOAuthClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  cfg.GoogleOAuthRedirectURL,
		Scopes:       gmailScopes,
	}}
}

func (g *GoogleOAuthService) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return g.client.AuthCodeURL(state, opts...)
}

func (g *GoogleOAuthService) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return g.client.Exchange(ctx, code, opts...)
}

func (g *GoogleOAuthService) Client(ctx context.Context, token *oauth2.Token) *http.Client {
	return g.client.Client(ctx, token)
}

func (g *GoogleOAuthService) Revoke(ctx context.Context, token *oauth2.Token) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/revoke?token="+token.AccessToken, nil)
	if err != nil {
		return err
	}
	var httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("token revoke returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
