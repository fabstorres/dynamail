package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net/http"

	"github.com/fabstorres/dynamail/apps/api/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var gmailScopes = []string{
	"https://www.googleapis.com/auth/gmail.readonly",
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/gmail.compose",
	"https://www.googleapis.com/auth/gmail.send",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

type AuthHandler struct {
	oauthConfig *oauth2.Config
}

func generateState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func NewHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.GoogleOAuthClientID,
			ClientSecret: cfg.GoogleOAuthClientSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  cfg.GoogleOAuthRedirectURL,
			Scopes:       gmailScopes,
		},
	}
}

func (ah *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println(err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	oauthURL := ah.oauthConfig.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce)

	http.Redirect(w, r, oauthURL, http.StatusTemporaryRedirect)
}

func (ah *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	state := r.URL.Query().Get("state")
	if err != nil || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		http.Error(w, "bad request", http.StatusBadRequest)
		log.Println(err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{Name: "oauth_state", MaxAge: -1, Path: "/"})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		log.Println("code not found")
		return
	}

	// TODO: get token and create a session
	_, err = ah.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println(err.Error())
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
