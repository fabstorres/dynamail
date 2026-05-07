package handler

import (
	"encoding/json"
	"log"
	"net/http"

	dynamail "github.com/fabstorres/dynamail/apps/api/internal/middleware"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type ThreadHandler struct{}

func (th *ThreadHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	oauthToken, ok := ctx.Value(dynamail.CtxOAuthToken).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		log.Println("Unauthorized: no OAuth token found in context")
		return
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: oauthToken})
	client := oauth2.NewClient(ctx, ts)

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to create Gmail service:", err)
		return
	}

	threads, err := srv.Users.Threads.List("me").Do()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to list threads:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(threads.Threads)
}
