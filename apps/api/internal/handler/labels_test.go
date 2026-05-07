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
	"github.com/go-chi/chi/v5"
	"google.golang.org/api/gmail/v1"
)

type mockLabelService struct {
	listFunc   func() (*gmail.ListLabelsResponse, error)
	createFunc func(name string) (*gmail.Label, error)
	updateFunc func(id, name string) (*gmail.Label, error)
	deleteFunc func(id string) error
}

func (m *mockLabelService) List() (*gmail.ListLabelsResponse, error) {
	return m.listFunc()
}

func (m *mockLabelService) Create(name string) (*gmail.Label, error) {
	return m.createFunc(name)
}

func (m *mockLabelService) Update(id, name string) (*gmail.Label, error) {
	return m.updateFunc(id, name)
}

func (m *mockLabelService) Delete(id string) error {
	return m.deleteFunc(id)
}

func setupLabelHandler(m *mockLabelService) *handler.LabelHandler {
	return handler.NewLabelHandler().WithServiceFactory(func(ctx context.Context, token string) (handler.LabelService, error) {
		if token != "test-token" {
			return nil, errors.New("bad token")
		}
		return m, nil
	})
}

func TestListLabels_Success(t *testing.T) {
	mock := &mockLabelService{
		listFunc: func() (*gmail.ListLabelsResponse, error) {
			return &gmail.ListLabelsResponse{
				Labels: []*gmail.Label{
					{Id: "INBOX", Name: "INBOX"},
					{Id: "LABEL_1", Name: "MyLabel"},
				},
			}, nil
		},
	}

	h := setupLabelHandler(mock)
	r := chi.NewRouter()
	r.Get("/", h.List)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
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
	labels, ok := body["labels"].([]interface{})
	if !ok || len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %v", body["labels"])
	}
}

func TestListLabels_Unauthorized(t *testing.T) {
	h := handler.NewLabelHandler()
	r := chi.NewRouter()
	r.Get("/", h.List)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCreateLabel_Success(t *testing.T) {
	mock := &mockLabelService{
		createFunc: func(name string) (*gmail.Label, error) {
			if name != "MyLabel" {
				t.Errorf("expected name=MyLabel, got %q", name)
			}
			return &gmail.Label{Id: "LABEL_1", Name: "MyLabel"}, nil
		},
	}

	h := setupLabelHandler(mock)
	r := chi.NewRouter()
	r.Post("/", h.Create)

	body := `{"name":"MyLabel"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var label gmail.Label
	if err := json.Unmarshal(rec.Body.Bytes(), &label); err != nil {
		t.Fatal(err)
	}
	if label.Name != "MyLabel" {
		t.Fatalf("expected label name MyLabel, got %s", label.Name)
	}
}

func TestCreateLabel_MissingName(t *testing.T) {
	mock := &mockLabelService{}
	h := setupLabelHandler(mock)
	r := chi.NewRouter()
	r.Post("/", h.Create)

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateLabel_InvalidBody(t *testing.T) {
	mock := &mockLabelService{}
	h := setupLabelHandler(mock)
	r := chi.NewRouter()
	r.Post("/", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not-json"))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateLabel_Success(t *testing.T) {
	mock := &mockLabelService{
		updateFunc: func(id, name string) (*gmail.Label, error) {
			if id != "LABEL_1" {
				t.Errorf("expected id=LABEL_1, got %q", id)
			}
			if name != "RenamedLabel" {
				t.Errorf("expected name=RenamedLabel, got %q", name)
			}
			return &gmail.Label{Id: "LABEL_1", Name: "RenamedLabel"}, nil
		},
	}

	h := setupLabelHandler(mock)
	r := chi.NewRouter()
	r.Patch("/{id}", h.Update)

	body := `{"name":"RenamedLabel"}`
	req := httptest.NewRequest(http.MethodPatch, "/LABEL_1", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var label gmail.Label
	if err := json.Unmarshal(rec.Body.Bytes(), &label); err != nil {
		t.Fatal(err)
	}
	if label.Name != "RenamedLabel" {
		t.Fatalf("expected label name RenamedLabel, got %s", label.Name)
	}
}

func TestUpdateLabel_MissingName(t *testing.T) {
	mock := &mockLabelService{}
	h := setupLabelHandler(mock)
	r := chi.NewRouter()
	r.Patch("/{id}", h.Update)

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPatch, "/LABEL_1", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteLabel_Success(t *testing.T) {
	mock := &mockLabelService{
		deleteFunc: func(id string) error {
			if id != "LABEL_1" {
				t.Errorf("expected id=LABEL_1, got %q", id)
			}
			return nil
		},
	}

	h := setupLabelHandler(mock)
	r := chi.NewRouter()
	r.Delete("/{id}", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/LABEL_1", nil)
	req = withAuthToken(req)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body for 204, got %s", rec.Body.String())
	}
}

