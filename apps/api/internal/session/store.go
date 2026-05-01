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
		Secure:   cfg.AppEnviroment != "development",
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
		return nil, err
	}
	if session.IsNew {
		return nil, nil
	}
	// TODO: add a validation for session_id and user_id
	tokenData := &TokenData{
		SessionID: session.Values["session_id"].(string),
		UserID:    session.Values["user_id"].(string),
	}
	return tokenData, nil
}

func (s *Store) Delete(w http.ResponseWriter, r *http.Request) error {
	session, err := s.store.Get(r, "session")
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	return s.store.Save(r, w, session)
}
