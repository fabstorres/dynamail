package session

import "net/http"

type TokenData struct {
	SessionID string
	UserID    string
}

type SessionService interface {
	SetSession(w http.ResponseWriter, r *http.Request, tokenData *TokenData) error
	GetSession(r *http.Request) (*TokenData, error)
	DeleteSession(w http.ResponseWriter, r *http.Request) error
}
