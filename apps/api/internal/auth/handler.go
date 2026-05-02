package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/fabstorres/dynamail/apps/api/internal/config"
	"github.com/fabstorres/dynamail/apps/api/internal/database"
	"github.com/fabstorres/dynamail/apps/api/internal/session"

	"golang.org/x/oauth2"
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
	oauth                  OAuthService
	sessions               session.SessionService
	db                     database.DatabaseService
	secureStateCookie      bool
	authSuccessRedirectURL string
}

func generateState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func NewHandler(cfg *config.Config, sessions session.SessionService, db database.DatabaseService) *AuthHandler {
	return &AuthHandler{
		oauth:                  NewGoogleOAuthService(*cfg),
		sessions:               sessions,
		db:                     db,
		secureStateCookie:      cfg.AppEnvironment != "development",
		authSuccessRedirectURL: cfg.AuthSuccessRedirectURL,
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
		Secure:   ah.secureStateCookie,
		SameSite: http.SameSiteLaxMode,
	})

	oauthURL := ah.oauth.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce)

	http.Redirect(w, r, oauthURL, http.StatusTemporaryRedirect)
}

func (ah *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", MaxAge: -1, Path: "/"})
	oauthErr := r.URL.Query().Get("error")
	if oauthErr != "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		log.Println("oauth callback rejected: " + oauthErr)
		return
	}

	state := r.URL.Query().Get("state")
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		log.Println("oauth callback rejected: missing state cookie")
		return
	}
	if state == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		log.Println("oauth callback rejected: missing state parameter")
		return
	}
	if subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		http.Error(w, "bad request", http.StatusBadRequest)
		log.Println("oauth callback rejected: state mismatch")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		log.Println("oauth callback rejected: code not found")
		return
	}

	token, err := ah.oauth.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println(err.Error())
		return
	}

	userInfo, err := ah.fetchUserInfo(r.Context(), token)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println(err.Error())
		return
	}

	var userID string
	existingUser, err := ah.db.GetUserByEmail(userInfo.Email)
	if err != nil {
		userID, err = ah.db.CreateUser(userInfo.Email, userInfo.Name)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			log.Println(err.Error())
			return
		}
	} else {
		userID = existingUser.ID
	}

	sessionID, err := ah.db.CreateSession(userID, token.AccessToken, token.RefreshToken, token.Expiry)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println(err.Error())
		return
	}

	err = ah.sessions.SetSession(w, r, &session.TokenData{
		SessionID: sessionID,
		UserID:    userID,
	})
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println(err.Error())
		return
	}

	http.Redirect(w, r, ah.authSuccessRedirectURL, http.StatusTemporaryRedirect)
}

// TODO: ensure same site lax when frontend is scaffolded
func (ah *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	data, err := ah.sessions.GetSession(r)
	if err != nil {
		log.Println(err.Error())
	}
	if err == nil && data != nil && data.SessionID != "" {
		dbSession, err := ah.db.GetSessionByID(data.SessionID)
		if err == nil {
			err = ah.oauth.Revoke(r.Context(), &oauth2.Token{AccessToken: dbSession.AccessToken})
			if err != nil {
				log.Println("failed to revoke token:", err.Error())
			}
			if err := ah.db.DeleteSessionByID(dbSession.ID); err != nil {
				log.Println(err.Error())
			}
		} else {
			log.Println(err.Error())
		}
	}

	if err := ah.sessions.DeleteSession(w, r); err != nil {
		http.Error(w, "failed to clear session", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type UserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (ah *AuthHandler) fetchUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := ah.oauth.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned %d", resp.StatusCode)
	}

	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}
