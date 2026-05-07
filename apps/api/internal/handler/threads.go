package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	dynamail "github.com/fabstorres/dynamail/apps/api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// ThreadService abstracts Gmail thread operations for testability.
type ThreadService interface {
	List(q string, labelIds []string, maxResults int64, pageToken string) (*gmail.ListThreadsResponse, error)
	Get(id string) (*gmail.Thread, error)
	Modify(id string, req *gmail.ModifyThreadRequest) (*gmail.Thread, error)
	Trash(id string) (*gmail.Thread, error)
}

type gmailThreadService struct {
	srv *gmail.Service
}

func newGmailThreadService(ctx context.Context, token string) (ThreadService, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	client := oauth2.NewClient(ctx, ts)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &gmailThreadService{srv: srv}, nil
}

func (g *gmailThreadService) List(q string, labelIds []string, maxResults int64, pageToken string) (*gmail.ListThreadsResponse, error) {
	call := g.srv.Users.Threads.List("me")
	if q != "" {
		call = call.Q(q)
	}
	if len(labelIds) > 0 {
		call = call.LabelIds(labelIds...)
	}
	if maxResults > 0 {
		call = call.MaxResults(maxResults)
	}
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	return call.Do()
}

func (g *gmailThreadService) Get(id string) (*gmail.Thread, error) {
	return g.srv.Users.Threads.Get("me", id).Do()
}

func (g *gmailThreadService) Modify(id string, req *gmail.ModifyThreadRequest) (*gmail.Thread, error) {
	return g.srv.Users.Threads.Modify("me", id, req).Do()
}

func (g *gmailThreadService) Trash(id string) (*gmail.Thread, error) {
	return g.srv.Users.Threads.Trash("me", id).Do()
}

type ThreadHandler struct {
	newService func(ctx context.Context, token string) (ThreadService, error)
}

func NewThreadHandler() *ThreadHandler {
	return &ThreadHandler{
		newService: newGmailThreadService,
	}
}

// WithServiceFactory allows injecting a mock service in tests.
func (th *ThreadHandler) WithServiceFactory(fn func(ctx context.Context, token string) (ThreadService, error)) *ThreadHandler {
	th.newService = fn
	return th
}

func (th *ThreadHandler) newGmailService(r *http.Request) (ThreadService, error) {
	ctx := r.Context()
	oauthToken, ok := ctx.Value(dynamail.CtxOAuthToken).(string)
	if !ok || oauthToken == "" {
		return nil, errors.New("missing oauth token")
	}
	return th.newService(ctx, oauthToken)
}

func (th *ThreadHandler) List(w http.ResponseWriter, r *http.Request) {
	svc, err := th.newGmailService(r)
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

	q := r.URL.Query().Get("q")
	label := r.URL.Query().Get("label")
	maxResultsStr := r.URL.Query().Get("maxResults")
	pageToken := r.URL.Query().Get("pageToken")

	var maxResults int64
	if maxResultsStr != "" {
		parsed, err := strconv.ParseInt(maxResultsStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid maxResults", http.StatusBadRequest)
			return
		}
		maxResults = parsed
	}

	var labelIds []string
	if label != "" {
		labelIds = []string{label}
	}

	res, err := svc.List(q, labelIds, maxResults, pageToken)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to list threads:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"threads":       res.Threads,
		"nextPageToken": res.NextPageToken,
	})
}

func (th *ThreadHandler) Get(w http.ResponseWriter, r *http.Request) {
	svc, err := th.newGmailService(r)
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
		http.Error(w, "missing thread id", http.StatusBadRequest)
		return
	}

	thread, err := svc.Get(id)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to get thread:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(thread)
}

func (th *ThreadHandler) Modify(w http.ResponseWriter, r *http.Request) {
	svc, err := th.newGmailService(r)
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
		http.Error(w, "missing thread id", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		AddLabelIds    []string `json:"addLabelIds"`
		RemoveLabelIds []string `json:"removeLabelIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	modifyReq := &gmail.ModifyThreadRequest{
		AddLabelIds:    reqBody.AddLabelIds,
		RemoveLabelIds: reqBody.RemoveLabelIds,
	}

	thread, err := svc.Modify(id, modifyReq)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to modify thread:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(thread)
}

func (th *ThreadHandler) Trash(w http.ResponseWriter, r *http.Request) {
	svc, err := th.newGmailService(r)
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
		http.Error(w, "missing thread id", http.StatusBadRequest)
		return
	}

	thread, err := svc.Trash(id)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to trash thread:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(thread)
}
