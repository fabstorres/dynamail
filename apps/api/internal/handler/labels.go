package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	dynamail "github.com/fabstorres/dynamail/apps/api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// LabelService abstracts Gmail label operations for testability.
type LabelService interface {
	List() (*gmail.ListLabelsResponse, error)
	Create(name string) (*gmail.Label, error)
	Update(id, name string) (*gmail.Label, error)
	Delete(id string) error
}

type gmailLabelService struct {
	srv *gmail.Service
}

func newGmailLabelService(ctx context.Context, token string) (LabelService, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	client := oauth2.NewClient(ctx, ts)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &gmailLabelService{srv: srv}, nil
}

func (g *gmailLabelService) List() (*gmail.ListLabelsResponse, error) {
	return g.srv.Users.Labels.List("me").Do()
}

func (g *gmailLabelService) Create(name string) (*gmail.Label, error) {
	label := &gmail.Label{Name: name}
	return g.srv.Users.Labels.Create("me", label).Do()
}

func (g *gmailLabelService) Update(id, name string) (*gmail.Label, error) {
	label := &gmail.Label{Name: name}
	return g.srv.Users.Labels.Patch("me", id, label).Do()
}

func (g *gmailLabelService) Delete(id string) error {
	return g.srv.Users.Labels.Delete("me", id).Do()
}

type LabelHandler struct {
	newService func(ctx context.Context, token string) (LabelService, error)
}

func NewLabelHandler() *LabelHandler {
	return &LabelHandler{
		newService: newGmailLabelService,
	}
}

// WithServiceFactory allows injecting a mock service in tests.
func (lh *LabelHandler) WithServiceFactory(fn func(ctx context.Context, token string) (LabelService, error)) *LabelHandler {
	lh.newService = fn
	return lh
}

func (lh *LabelHandler) newGmailService(r *http.Request) (LabelService, error) {
	ctx := r.Context()
	oauthToken, ok := ctx.Value(dynamail.CtxOAuthToken).(string)
	if !ok || oauthToken == "" {
		return nil, errors.New("missing oauth token")
	}
	return lh.newService(ctx, oauthToken)
}

func (lh *LabelHandler) List(w http.ResponseWriter, r *http.Request) {
	svc, err := lh.newGmailService(r)
	if err != nil {
		if err.Error() == "missing oauth token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			log.Println("Unauthorized: no OAuth token found in context")
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			log.Println("Failed to create Gmail service:", err)
		}
		return
	}

	res, err := svc.List()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to list labels:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (lh *LabelHandler) Create(w http.ResponseWriter, r *http.Request) {
	svc, err := lh.newGmailService(r)
	if err != nil {
		if err.Error() == "missing oauth token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			log.Println("Unauthorized: no OAuth token found in context")
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			log.Println("Failed to create Gmail service:", err)
		}
		return
	}

	var reqBody struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if reqBody.Name == "" {
		http.Error(w, "missing label name", http.StatusBadRequest)
		return
	}

	label, err := svc.Create(reqBody.Name)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to create label:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(label)
}

func (lh *LabelHandler) Update(w http.ResponseWriter, r *http.Request) {
	svc, err := lh.newGmailService(r)
	if err != nil {
		if err.Error() == "missing oauth token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			log.Println("Unauthorized: no OAuth token found in context")
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			log.Println("Failed to create Gmail service:", err)
		}
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing label id", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if reqBody.Name == "" {
		http.Error(w, "missing label name", http.StatusBadRequest)
		return
	}

	label, err := svc.Update(id, reqBody.Name)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to update label:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(label)
}

func (lh *LabelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	svc, err := lh.newGmailService(r)
	if err != nil {
		if err.Error() == "missing oauth token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			log.Println("Unauthorized: no OAuth token found in context")
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			log.Println("Failed to create Gmail service:", err)
		}
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing label id", http.StatusBadRequest)
		return
	}

	if err := svc.Delete(id); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to delete label:", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
