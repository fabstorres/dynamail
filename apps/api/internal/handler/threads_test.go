package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fabstorres/dynamail/apps/api/internal/handler"
	dynamail "github.com/fabstorres/dynamail/apps/api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"google.golang.org/api/gmail/v1"
)

type mockThreadService struct {
	listFunc   func(q string, labelIds []string, maxResults int64, pageToken string) (*gmail.ListThreadsResponse, error)
	getFunc    func(id string) (*gmail.Thread, error)
	modifyFunc func(id string, req *gmail.ModifyThreadRequest) (*gmail.Thread, error)
	trashFunc  func(id string) (*gmail.Thread, error)
}

func (m *mockThreadService) List(q string, labelIds []string, maxResults int64, pageToken string) (*gmail.ListThreadsResponse, error) {
	return m.listFunc(q, labelIds, maxResults, pageToken)
}
func (m *mockThreadService) Get(id string) (*gmail.Thread, error) {
	return m.getFunc(id)
}
func (m *mockThreadService) Modify(id string, req *gmail.ModifyThreadRequest) (*gmail.Thread, error) {
	return m.modifyFunc(id, req)
}
func (m *mockThreadService) Trash(id string) (*gmail.Thread, error) {
	return m.trashFunc(id)
}

func withAuthToken(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), dynamail.CtxOAuthToken, "test-token")
	return r.WithContext(ctx)
}

func setupThreadHandler(m *mockThreadService) *handler.ThreadHandler {
	return handler.NewThreadHandler().WithServiceFactory(func(ctx context.Context, token string) (handler.ThreadService, error) {
		if token != "test-token" {
			return nil, errors.New("bad token")
		}
		return m, nil
	})
}

func TestListThreads_Success(t *testing.T) {
	mock := &mockThreadService{
		listFunc: func(q string, labelIds []string, maxResults int64, pageToken string) (*gmail.ListThreadsResponse, error) {
			if q != "subject:hello" {
				t.Errorf("expected q=subject:hello, got %q", q)
			}
			if len(labelIds) != 1 || labelIds[0] != "INBOX" {
				t.Errorf("expected labelIds=[INBOX], got %v", labelIds)
			}
			if maxResults != 10 {
				t.Errorf("expected maxResults=10, got %d", maxResults)
			}
			if pageToken != "token123" {
				t.Errorf("expected pageToken=token123, got %q", pageToken)
			}
			return &gmail.ListThreadsResponse{
				Threads: []*gmail.Thread{
					{Id: "t1", Snippet: "Hello"},
				},
				NextPageToken: "next456",
			}, nil
		},
	}

	h := setupThreadHandler(mock)
	r := chi.NewRouter()
	r.Get("/", h.List)

	req := httptest.NewRequest(http.MethodGet, "/?q=subject:hello&label=INBOX&maxResults=10&pageToken=token123", nil)
	req = withAuthToken(req)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	threads, ok := body["threads"].([]interface{})
	if !ok || len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %v", body["threads"])
	}
	if body["nextPageToken"] != "next456" {
		t.Fatalf("expected nextPageToken next456, got %v", body["nextPageToken"])
	}
}

func TestListThreads_Unauthorized(t *testing.T) {
	h := handler.NewThreadHandler()
	r := chi.NewRouter()
	r.Get("/", h.List)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestListThreads_InvalidMaxResults(t *testing.T) {
	h := setupThreadHandler(&mockThreadService{})
	r := chi.NewRouter()
	r.Get("/", h.List)

	req := httptest.NewRequest(http.MethodGet, "/?maxResults=abc", nil)
	req = withAuthToken(req)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetThread_Success(t *testing.T) {
	mock := &mockThreadService{
		getFunc: func(id string) (*gmail.Thread, error) {
			if id != "thread123" {
				t.Errorf("expected id=thread123, got %q", id)
			}
			return &gmail.Thread{Id: "thread123", Snippet: "Hello"}, nil
		},
	}

	h := setupThreadHandler(mock)
	r := chi.NewRouter()
	r.Get("/{id}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/thread123", nil)
	req = withAuthToken(req)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var thread gmail.Thread
	if err := json.Unmarshal(rec.Body.Bytes(), &thread); err != nil {
		t.Fatal(err)
	}
	if thread.Id != "thread123" {
		t.Fatalf("expected thread id thread123, got %s", thread.Id)
	}
}

func TestModifyThread_Success(t *testing.T) {
	mock := &mockThreadService{
		modifyFunc: func(id string, req *gmail.ModifyThreadRequest) (*gmail.Thread, error) {
			if id != "thread123" {
				t.Errorf("expected id=thread123, got %q", id)
			}
			if len(req.AddLabelIds) != 1 || req.AddLabelIds[0] != "LABEL_1" {
				t.Errorf("expected addLabelIds=[LABEL_1], got %v", req.AddLabelIds)
			}
			if len(req.RemoveLabelIds) != 1 || req.RemoveLabelIds[0] != "INBOX" {
				t.Errorf("expected removeLabelIds=[INBOX], got %v", req.RemoveLabelIds)
			}
			return &gmail.Thread{Id: "thread123"}, nil
		},
	}

	h := setupThreadHandler(mock)
	r := chi.NewRouter()
	r.Patch("/{id}", h.Modify)

	body := `{"addLabelIds":["LABEL_1"],"removeLabelIds":["INBOX"]}`
	req := httptest.NewRequest(http.MethodPatch, "/thread123", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestModifyThread_InvalidBody(t *testing.T) {
	h := setupThreadHandler(&mockThreadService{})
	r := chi.NewRouter()
	r.Patch("/{id}", h.Modify)

	req := httptest.NewRequest(http.MethodPatch, "/thread123", strings.NewReader("not-json"))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTrashThread_Success(t *testing.T) {
	mock := &mockThreadService{
		trashFunc: func(id string) (*gmail.Thread, error) {
			if id != "thread123" {
				t.Errorf("expected id=thread123, got %q", id)
			}
			return &gmail.Thread{Id: "thread123"}, nil
		},
	}

	h := setupThreadHandler(mock)
	r := chi.NewRouter()
	r.Delete("/{id}/trash", h.Trash)

	req := httptest.NewRequest(http.MethodDelete, "/thread123/trash", nil)
	req = withAuthToken(req)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
