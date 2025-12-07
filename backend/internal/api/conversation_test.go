package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"multi-avatar-chat/internal/db"
)

func setupTestConversationHandler(t *testing.T) (*ConversationHandler, *AvatarHandler, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test_conv_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	database, err := db.NewDB(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	convHandler := NewConversationHandler(database, nil)
	avatarHandler := NewAvatarHandler(database, nil)

	cleanup := func() {
		database.Close()
		os.Remove(tmpFile.Name())
	}

	return convHandler, avatarHandler, cleanup
}

func TestCreateConversation_Success(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	body := `{"title": "Test Chat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response ConversationResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Title != "Test Chat" {
		t.Errorf("expected title 'Test Chat', got '%s'", response.Title)
	}
	if response.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestCreateConversation_MissingTitle(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestListConversations_Empty(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/conversations", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []ConversationResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 0 {
		t.Errorf("expected 0 conversations, got %d", len(response))
	}
}

func TestListConversations_WithData(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	// Create test conversations
	createBody := `{"title": "Chat 1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	createBody = `{"title": "Chat 2"}`
	req = httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.Create(w, req)

	// List conversations
	req = httptest.NewRequest(http.MethodGet, "/api/conversations", nil)
	w = httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []ConversationResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(response))
	}
}

func TestGetConversation_Success(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	// Create test conversation
	createBody := `{"title": "Get Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Get conversation
	req = httptest.NewRequest(http.MethodGet, "/api/conversations/1", nil)
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ConversationResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Title != "Get Test" {
		t.Errorf("expected title 'Get Test', got '%s'", response.Title)
	}
}

func TestGetConversation_NotFound(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/conversations/99999", nil)
	req.SetPathValue("id", "99999")
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDeleteConversation_Success(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	// Create test conversation
	createBody := `{"title": "ToDelete"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Delete conversation
	req = httptest.NewRequest(http.MethodDelete, "/api/conversations/1", nil)
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Verify deletion
	req = httptest.NewRequest(http.MethodGet, "/api/conversations/1", nil)
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d after deletion, got %d", http.StatusNotFound, w.Code)
	}
}

func TestSendMessage_Success(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	// Create test conversation
	createBody := `{"title": "Message Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Send message
	msgBody := `{"content": "Hello, world!"}`
	req = httptest.NewRequest(http.MethodPost, "/api/conversations/1/messages", bytes.NewBufferString(msgBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.SendMessage(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response SendMessageResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.UserMessage.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got '%s'", response.UserMessage.Content)
	}
	if response.UserMessage.SenderType != "user" {
		t.Errorf("expected sender_type 'user', got '%s'", response.UserMessage.SenderType)
	}
}

func TestSendMessage_ConversationNotFound(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	msgBody := `{"content": "Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations/99999/messages", bytes.NewBufferString(msgBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "99999")
	w := httptest.NewRecorder()

	handler.SendMessage(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGetMessages_Empty(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	// Create test conversation
	createBody := `{"title": "Empty Messages"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Get messages
	req = httptest.NewRequest(http.MethodGet, "/api/conversations/1/messages", nil)
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.GetMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []MessageResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 0 {
		t.Errorf("expected 0 messages, got %d", len(response))
	}
}

func TestGetMessages_WithData(t *testing.T) {
	handler, _, cleanup := setupTestConversationHandler(t)
	defer cleanup()

	// Create test conversation
	createBody := `{"title": "Messages Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Send messages
	msgBody := `{"content": "Message 1"}`
	req = httptest.NewRequest(http.MethodPost, "/api/conversations/1/messages", bytes.NewBufferString(msgBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.SendMessage(w, req)

	msgBody = `{"content": "Message 2"}`
	req = httptest.NewRequest(http.MethodPost, "/api/conversations/1/messages", bytes.NewBufferString(msgBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.SendMessage(w, req)

	// Get messages
	req = httptest.NewRequest(http.MethodGet, "/api/conversations/1/messages", nil)
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.GetMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []MessageResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("expected 2 messages, got %d", len(response))
	}
}

