package auth_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/fabstorres/dynamail/apps/api/internal/auth"
	"github.com/fabstorres/dynamail/apps/api/internal/config"
)

func TestLoginSetsStateCookieAndRedirects(t *testing.T) {
	oauthSvc := auth.NewGoogleOAuthService(&config.Config{
		GoogleOAuthClientID:     "client-id",
		GoogleOAuthClientSecret: "client-secret",
		GoogleOAuthRedirectURL:  "http://example.com/auth/callback",
	})
	handler := auth.NewHandler(&config.Config{
		GoogleOAuthClientID:     "client-id",
		GoogleOAuthClientSecret: "client-secret",
		GoogleOAuthRedirectURL:  "http://example.com/auth/callback",
		AppEnvironment:          "development",
	}, nil, nil, oauthSvc)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "accounts.google.com/o/oauth2/auth") {
		t.Fatalf("expected redirect to Google OAuth, got %q", location)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "oauth_state" || cookies[0].Value == "" {
		t.Fatalf("expected oauth_state cookie to be set, got %#v", cookies)
	}
}

func TestLoginStateCookieMatchesRedirectState(t *testing.T) {
	oauthSvc := auth.NewGoogleOAuthService(&config.Config{
		GoogleOAuthClientID:     "client-id",
		GoogleOAuthClientSecret: "client-secret",
		GoogleOAuthRedirectURL:  "http://example.com/auth/callback",
	})
	handler := auth.NewHandler(&config.Config{
		GoogleOAuthClientID:     "client-id",
		GoogleOAuthClientSecret: "client-secret",
		GoogleOAuthRedirectURL:  "http://example.com/auth/callback",
		AppEnvironment:          "development",
	}, nil, nil, oauthSvc)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	// Check status
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}

	// Find oauth_state cookie
	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
			break
		}
	}

	if stateCookie == nil || stateCookie.Value == "" {
		t.Fatal("expected oauth_state cookie to be set")
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected redirect location header")
	}

	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}

	if u.Host != "accounts.google.com" {
		t.Fatalf("unexpected redirect host: %s", u.Host)
	}

	redirectState := u.Query().Get("state")
	if redirectState == "" {
		t.Fatal("expected state query param in redirect URL")
	}

	if redirectState != stateCookie.Value {
		t.Fatalf("state mismatch: cookie=%q redirect=%q", stateCookie.Value, redirectState)
	}
}
