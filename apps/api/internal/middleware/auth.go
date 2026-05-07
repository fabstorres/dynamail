package middleware

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/fabstorres/dynamail/apps/api/internal/auth"
	"github.com/fabstorres/dynamail/apps/api/internal/database"
	"github.com/fabstorres/dynamail/apps/api/internal/session"
	"golang.org/x/oauth2"
)

const CtxOAuthToken = "oauth_token"

type AuthMiddleware interface {
	Handle(next http.Handler) http.Handler
}

type SessionAuthMiddleware struct {
	sessions session.SessionService
	db       database.DatabaseService
	oauth    auth.OAuthService
}

func NewSessionAuthMiddleware(sessions session.SessionService, db database.DatabaseService, oauth auth.OAuthService) *SessionAuthMiddleware {
	return &SessionAuthMiddleware{
		sessions: sessions,
		db:       db,
		oauth:    oauth,
	}
}

func (sam *SessionAuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := sam.sessions.GetSession(r)
		if err != nil || session == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			if err != nil {
				log.Printf("auth middleware: %v", err)
			}
			return
		}

		userSession, err := sam.db.GetSessionByID(session.SessionID)
		if err != nil || userSession == nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			if err != nil {
				log.Printf("getSessionByID: %v", err)
			}
			return
		}

		expiry, err := time.Parse(time.RFC3339, userSession.Expiry)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			log.Printf("failed to parse session expiry: %v", err)
			return
		}

		token := &oauth2.Token{
			AccessToken:  userSession.AccessToken,
			RefreshToken: userSession.RefreshToken,
			Expiry:       expiry,
		}

		needsRefresh := time.Now().After(expiry.Add(-30 * time.Minute))
		if needsRefresh {
			newToken, err := sam.oauth.RefreshToken(r.Context(), token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				log.Printf("token refresh failed: %v", err)
				return
			}

			err = sam.db.UpsertSession(userSession.ID, userSession.UserID, newToken.AccessToken, newToken.RefreshToken, newToken.Expiry)
			if err != nil {
				log.Printf("failed to update session after refresh: %v", err)
			}
			userSession.AccessToken = newToken.AccessToken
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, "oauth_token", userSession.AccessToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
