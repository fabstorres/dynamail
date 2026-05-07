package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	dynamail "github.com/fabstorres/dynamail/apps/api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// MessageService abstracts Gmail message operations for testability.
type MessageService interface {
	Get(id string) (*gmail.Message, error)
	GetAttachment(messageId, attachmentId string) (data []byte, filename string, mimeType string, err error)
	Send(to []string, subject, body string) (*gmail.Message, error)
	Reply(originalID, body string) (*gmail.Message, error)
	Forward(originalID string, to []string, note string) (*gmail.Message, error)
}

type gmailMessageService struct {
	srv *gmail.Service
}

func newGmailMessageService(ctx context.Context, token string) (MessageService, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	client := oauth2.NewClient(ctx, ts)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &gmailMessageService{srv: srv}, nil
}

func (g *gmailMessageService) Get(id string) (*gmail.Message, error) {
	return g.srv.Users.Messages.Get("me", id).Format("full").Do()
}

func (g *gmailMessageService) GetAttachment(messageId, attachmentId string) ([]byte, string, string, error) {
	msg, err := g.srv.Users.Messages.Get("me", messageId).Format("full").Do()
	if err != nil {
		return nil, "", "", err
	}

	filename, mimeType := findAttachmentMeta(msg.Payload, attachmentId)
	if filename == "" {
		filename = "attachment.bin"
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	att, err := g.srv.Users.Messages.Attachments.Get("me", messageId, attachmentId).Do()
	if err != nil {
		return nil, "", "", err
	}

	data, err := decodeGmailData(att.Data)
	if err != nil {
		return nil, "", "", err
	}

	return data, filename, mimeType, nil
}

func (g *gmailMessageService) Send(to []string, subject, body string) (*gmail.Message, error) {
	raw := buildRawMessage(to, subject, body, nil)
	msg := &gmail.Message{Raw: raw}
	return g.srv.Users.Messages.Send("me", msg).Do()
}

func (g *gmailMessageService) Reply(originalID, body string) (*gmail.Message, error) {
	orig, err := g.Get(originalID)
	if err != nil {
		return nil, err
	}

	if orig.Payload == nil {
		return nil, errors.New("original message has no payload")
	}

	var to, subject, messageID, references string
	for _, h := range orig.Payload.Headers {
		switch h.Name {
		case "From":
			to = h.Value
		case "Reply-To":
			if h.Value != "" {
				to = h.Value
			}
		case "Subject":
			subject = h.Value
		case "Message-ID":
			messageID = h.Value
		case "References":
			references = h.Value
		}
	}

	if to == "" {
		return nil, errors.New("original message missing From header")
	}

	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	if references != "" {
		references = references + " " + messageID
	} else {
		references = messageID
	}

	hdrs := map[string]string{}
	if messageID != "" {
		hdrs["In-Reply-To"] = messageID
	}
	if references != "" {
		hdrs["References"] = references
	}

	raw := buildRawMessage([]string{to}, subject, body, hdrs)
	msg := &gmail.Message{
		Raw:      raw,
		ThreadId: orig.ThreadId,
	}
	return g.srv.Users.Messages.Send("me", msg).Do()
}

func (g *gmailMessageService) Forward(originalID string, to []string, note string) (*gmail.Message, error) {
	orig, err := g.Get(originalID)
	if err != nil {
		return nil, err
	}

	if orig.Payload == nil {
		return nil, errors.New("original message has no payload")
	}

	var subject, origFrom, origDate, origTo string
	for _, h := range orig.Payload.Headers {
		switch h.Name {
		case "Subject":
			subject = h.Value
		case "From":
			origFrom = h.Value
		case "Date":
			origDate = h.Value
		case "To":
			origTo = h.Value
		}
	}

	origBody := extractTextBody(orig.Payload)

	if !strings.HasPrefix(strings.ToLower(subject), "fwd:") {
		subject = "Fwd: " + subject
	}

	var b bytes.Buffer
	if note != "" {
		b.WriteString(note)
		b.WriteString("\r\n\r\n")
	}
	b.WriteString("---------- Forwarded message ----------\r\n")
	b.WriteString("From: " + origFrom + "\r\n")
	b.WriteString("Date: " + origDate + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("To: " + origTo + "\r\n")
	b.WriteString("\r\n")
	b.WriteString(origBody)

	raw := buildRawMessage(to, subject, b.String(), nil)
	msg := &gmail.Message{Raw: raw}
	return g.srv.Users.Messages.Send("me", msg).Do()
}

func findAttachmentMeta(part *gmail.MessagePart, attachmentId string) (filename, mimeType string) {
	if part == nil {
		return "", ""
	}
	if part.Body != nil && part.Body.AttachmentId == attachmentId {
		return part.Filename, part.MimeType
	}
	for _, p := range part.Parts {
		if f, m := findAttachmentMeta(p, attachmentId); f != "" || m != "" {
			return f, m
		}
	}
	return "", ""
}

func extractTextBody(part *gmail.MessagePart) string {
	if part == nil {
		return ""
	}
	if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
		if data, err := decodeGmailData(part.Body.Data); err == nil {
			return string(data)
		}
	}
	for _, p := range part.Parts {
		if text := extractTextBody(p); text != "" {
			return text
		}
	}
	return ""
}

func buildRawMessage(to []string, subject, body string, extraHeaders map[string]string) string {
	var b bytes.Buffer
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123) + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	for k, v := range extraHeaders {
		b.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	b.WriteString("\r\n")
	b.WriteString(body)
	return base64.URLEncoding.EncodeToString(b.Bytes())
}

func decodeGmailData(s string) ([]byte, error) {
	if len(s)%4 == 0 {
		if b, err := base64.URLEncoding.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return base64.RawURLEncoding.DecodeString(s)
}

type MessageHandler struct {
	newService func(ctx context.Context, token string) (MessageService, error)
}

func NewMessageHandler() *MessageHandler {
	return &MessageHandler{
		newService: newGmailMessageService,
	}
}

// WithServiceFactory allows injecting a mock service in tests.
func (mh *MessageHandler) WithServiceFactory(fn func(ctx context.Context, token string) (MessageService, error)) *MessageHandler {
	mh.newService = fn
	return mh
}

func (mh *MessageHandler) newGmailService(r *http.Request) (MessageService, error) {
	ctx := r.Context()
	oauthToken, ok := ctx.Value(dynamail.CtxOAuthToken).(string)
	if !ok || oauthToken == "" {
		return nil, errors.New("missing oauth token")
	}
	return mh.newService(ctx, oauthToken)
}

func (mh *MessageHandler) Get(w http.ResponseWriter, r *http.Request) {
	svc, err := mh.newGmailService(r)
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
		http.Error(w, "missing message id", http.StatusBadRequest)
		return
	}

	msg, err := svc.Get(id)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to get message:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func (mh *MessageHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	svc, err := mh.newGmailService(r)
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

	messageId := chi.URLParam(r, "id")
	if messageId == "" {
		http.Error(w, "missing message id", http.StatusBadRequest)
		return
	}

	attachmentId := chi.URLParam(r, "attId")
	if attachmentId == "" {
		http.Error(w, "missing attachment id", http.StatusBadRequest)
		return
	}

	data, filename, mimeType, err := svc.GetAttachment(messageId, attachmentId)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to get attachment:", err)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(data)
}

func (mh *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	svc, err := mh.newGmailService(r)
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
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Body    string   `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(reqBody.To) == 0 {
		http.Error(w, "missing recipients", http.StatusBadRequest)
		return
	}

	sent, err := svc.Send(reqBody.To, reqBody.Subject, reqBody.Body)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to send message:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sent)
}

func (mh *MessageHandler) Reply(w http.ResponseWriter, r *http.Request) {
	svc, err := mh.newGmailService(r)
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
		http.Error(w, "missing message id", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	sent, err := svc.Reply(id, reqBody.Body)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to reply:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sent)
}

func (mh *MessageHandler) Forward(w http.ResponseWriter, r *http.Request) {
	svc, err := mh.newGmailService(r)
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
		http.Error(w, "missing message id", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		To   []string `json:"to"`
		Note string   `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(reqBody.To) == 0 {
		http.Error(w, "missing recipients", http.StatusBadRequest)
		return
	}

	sent, err := svc.Forward(id, reqBody.To, reqBody.Note)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Println("Failed to forward:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sent)
}
