package handler

import (
	"encoding/json"
	"net/http"

	"github.com/fabstorres/dynamail/apps/api/internal/config"
	"github.com/fabstorres/dynamail/apps/api/internal/database"
	"github.com/fabstorres/dynamail/apps/api/internal/session"
)

type UserHandler struct {
	cfg      *config.Config
	sessions session.SessionService
	db       database.DatabaseService
}

func NewUserHandler(cfg *config.Config, sessions session.SessionService, db database.DatabaseService) *UserHandler {
	return &UserHandler{
		cfg:      cfg,
		sessions: sessions,
		db:       db,
	}
}

func (uh *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	data, err := uh.sessions.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if data == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	user, err := uh.db.GetUserByID(data.UserID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(
		map[string]string{
			"name":  user.Name,
			"email": user.Email,
		},
	)
}
