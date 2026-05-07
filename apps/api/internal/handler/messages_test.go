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

type mockMessageService struct {
	getFunc           func(id string) (*gmail.Message, error)
	getAttachmentFunc func(messageId, attachmentId string) ([]byte, string, string, error)
	sendFunc          func(to []string, subject, body string) (*gmail.Message, error)
	replyFunc         func(originalID, body string) (*gmail.Message, error)
	forwardFunc       func(originalID string, to []string, note string) (*gmail.Message, error)
}

func (m *mockMessageService) Get(id string) (*gmail.Message, error) {
	return m.getFunc(id)
}

func (m *mockMessageService) GetAttachment(messageId, attachmentId string) ([]byte, string, string, error) {
	return m.getAttachmentFunc(messageId, attachmentId)
}

func (m *mockMessageService) Send(to []string, subject, body string) (*gmail.Message, error) {
	return m.sendFunc(to, subject, body)
}

func (m *mockMessageService) Reply(originalID, body string) (*gmail.Message, error) {
	return m.replyFunc(originalID, body)
}

func (m *mockMessageService) Forward(originalID string, to []string, note string) (*gmail.Message, error) {
	return m.forwardFunc(originalID, to, note)
}

func setupMessageHandler(m *mockMessageService) *handler.MessageHandler {
	return handler.NewMessageHandler().WithServiceFactory(func(ctx context.Context, token string) (handler.MessageService, error) {
		if token != "test-token" {
			return nil, errors.New("bad token")
		}
		return m, nil
	})
}

func TestGetMessage_Success(t *testing.T) {
	mock := &mockMessageService{
		getFunc: func(id string) (*gmail.Message, error) {
			if id != "msg123" {
				t.Errorf("expected id=msg123, got %q", id)
			}
			return &gmail.Message{Id: "msg123", Snippet: "Hello"}, nil
		},
	}

	h := setupMessageHandler(mock)
	r := chi.NewRouter()
	r.Get("/{id}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/msg123", nil)
	req = withAuthToken(req)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var msg gmail.Message
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Id != "msg123" {
		t.Fatalf("expected message id msg123, got %s", msg.Id)
	}
}

func TestGetMessage_Unauthorized(t *testing.T) {
	h := handler.NewMessageHandler()
	r := chi.NewRouter()
	r.Get("/{id}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/msg123", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGetAttachment_Success(t *testing.T) {
	mock := &mockMessageService{
		getAttachmentFunc: func(messageId, attachmentId string) ([]byte, string, string, error) {
			if messageId != "msg123" {
				t.Errorf("expected messageId=msg123, got %q", messageId)
			}
			if attachmentId != "att456" {
				t.Errorf("expected attachmentId=att456, got %q", attachmentId)
			}
			return []byte("file content"), "report.pdf", "application/pdf", nil
		},
	}

	h := setupMessageHandler(mock)
	r := chi.NewRouter()
	r.Get("/{id}/attachments/{attId}", h.GetAttachment)

	req := httptest.NewRequest(http.MethodGet, "/msg123/attachments/att456", nil)
	req = withAuthToken(req)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "report.pdf") {
		t.Errorf("expected Content-Disposition with report.pdf, got %s", cd)
	}
	if string(rec.Body.Bytes()) != "file content" {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestSendMessage_Success(t *testing.T) {
	mock := &mockMessageService{
		sendFunc: func(to []string, subject, body string) (*gmail.Message, error) {
			if len(to) != 1 || to[0] != "alice@example.com" {
				t.Errorf("expected to=[alice@example.com], got %v", to)
			}
			if subject != "Hello" {
				t.Errorf("expected subject=Hello, got %q", subject)
			}
			if body != "World" {
				t.Errorf("expected body=World, got %q", body)
			}
			return &gmail.Message{Id: "sent123"}, nil
		},
	}

	h := setupMessageHandler(mock)
	r := chi.NewRouter()
	r.Post("/send", h.Send)

	body := `{"to":["alice@example.com"],"subject":"Hello","body":"World"}`
	req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var msg gmail.Message
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Id != "sent123" {
		t.Fatalf("expected sent message id sent123, got %s", msg.Id)
	}
}

func TestSendMessage_MissingTo(t *testing.T) {
	mock := &mockMessageService{}
	h := setupMessageHandler(mock)
	r := chi.NewRouter()
	r.Post("/send", h.Send)

	body := `{"to":[],"subject":"Hello","body":"World"}`
	req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSendMessage_InvalidBody(t *testing.T) {
	mock := &mockMessageService{}
	h := setupMessageHandler(mock)
	r := chi.NewRouter()
	r.Post("/send", h.Send)

	req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader("not-json"))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestReplyMessage_Success(t *testing.T) {
	mock := &mockMessageService{
		replyFunc: func(originalID, body string) (*gmail.Message, error) {
			if originalID != "msg123" {
				t.Errorf("expected originalID=msg123, got %q", originalID)
			}
			if body != "Sure thing" {
				t.Errorf("expected body=Sure thing, got %q", body)
			}
			return &gmail.Message{Id: "reply123"}, nil
		},
	}

	h := setupMessageHandler(mock)
	r := chi.NewRouter()
	r.Post("/{id}/reply", h.Reply)

	body := `{"body":"Sure thing"}`
	req := httptest.NewRequest(http.MethodPost, "/msg123/reply", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var msg gmail.Message
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Id != "reply123" {
		t.Fatalf("expected reply id reply123, got %s", msg.Id)
	}
}

func TestForwardMessage_Success(t *testing.T) {
	mock := &mockMessageService{
		forwardFunc: func(originalID string, to []string, note string) (*gmail.Message, error) {
			if originalID != "msg123" {
				t.Errorf("expected originalID=msg123, got %q", originalID)
			}
			if len(to) != 1 || to[0] != "bob@example.com" {
				t.Errorf("expected to=[bob@example.com], got %v", to)
			}
			if note != "Check this out" {
				t.Errorf("expected note=Check this out, got %q", note)
			}
			return &gmail.Message{Id: "fwd123"}, nil
		},
	}

	h := setupMessageHandler(mock)
	r := chi.NewRouter()
	r.Post("/{id}/forward", h.Forward)

	body := `{"to":["bob@example.com"],"note":"Check this out"}`
	req := httptest.NewRequest(http.MethodPost, "/msg123/forward", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var msg gmail.Message
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Id != "fwd123" {
		t.Fatalf("expected forward id fwd123, got %s", msg.Id)
	}
}

func TestForwardMessage_MissingTo(t *testing.T) {
	mock := &mockMessageService{}
	h := setupMessageHandler(mock)
	r := chi.NewRouter()
	r.Post("/{id}/forward", h.Forward)

	body := `{"to":[],"note":"Check this out"}`
	req := httptest.NewRequest(http.MethodPost, "/msg123/forward", strings.NewReader(body))
	req = withAuthToken(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
