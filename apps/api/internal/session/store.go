package session

import (
	"net/http"

	"github.com/fabstorres/dynamail/apps/api/internal/config"
	"github.com/gorilla/sessions"
)

type Store struct {
	store *sessions.CookieStore
}

type TokenData struct {
	SessionID string
	UserID    string
}

func NewStore(cfg *config.Config) *Store {
	s := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	s.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   cfg.AppEnvironment != "development",
		SameSite: http.SameSiteLaxMode,
	}
	return &Store{store: s}
}

func (s *Store) Set(w http.ResponseWriter, r *http.Request, tokenData *TokenData) error {
	session, err := s.store.Get(r, "session")
	if err != nil {
		return err
	}
	session.Values["session_id"] = tokenData.SessionID
	session.Values["user_id"] = tokenData.UserID
	return s.store.Save(r, w, session)
}

func (s *Store) Get(r *http.Request) (*TokenData, error) {
	session, err := s.store.Get(r, "session")
	if err != nil {
		return nil, nil
	}
	if session.IsNew {
		return nil, nil
	}
	sessionID, ok := session.Values["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, nil
	}
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		return nil, nil
	}

	tokenData := &TokenData{
		SessionID: sessionID,
		UserID:    userID,
	}
	return tokenData, nil
}

func (s *Store) Delete(w http.ResponseWriter, r *http.Request) error {
	session, err := s.store.Get(r, "session")
	if err != nil {
		session, err = s.store.New(r, "session")
		if err != nil {
			return err
		}
	}
	session.Options.MaxAge = -1
	return s.store.Save(r, w, session)
}
