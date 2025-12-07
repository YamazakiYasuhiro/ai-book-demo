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

func setupTestConversationAvatarHandler(t *testing.T) (*ConversationAvatarHandler, *db.DB, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test_conv_avatar_*.db")
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

	handler := NewConversationAvatarHandler(database, nil, nil) // nil assistant and watcher for testing

	cleanup := func() {
		database.Close()
		os.Remove(tmpFile.Name())
	}

	return handler, database, cleanup
}

func TestAddAvatar(t *testing.T) {
	handler, database, cleanup := setupTestConversationAvatarHandler(t)
	defer cleanup()

	// Create conversation and avatar
	conv, _ := database.CreateConversation("Test Chat", "thread_123")
	avatar, _ := database.CreateAvatar("TestBot", "Prompt", "asst_123")

	reqBody := AddAvatarRequest{AvatarID: avatar.ID}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/conversations/1/avatars", bytes.NewReader(body))
	req.SetPathValue("id", "1")

	w := httptest.NewRecorder()
	handler.AddAvatar(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Verify avatar was added
	avatars, _ := database.GetConversationAvatars(conv.ID)
	if len(avatars) != 1 {
		t.Errorf("expected 1 avatar, got %d", len(avatars))
	}
}

func TestAddAvatar_ConversationNotFound(t *testing.T) {
	handler, _, cleanup := setupTestConversationAvatarHandler(t)
	defer cleanup()

	reqBody := AddAvatarRequest{AvatarID: 1}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/conversations/99999/avatars", bytes.NewReader(body))
	req.SetPathValue("id", "99999")

	w := httptest.NewRecorder()
	handler.AddAvatar(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestAddAvatar_AvatarNotFound(t *testing.T) {
	handler, database, cleanup := setupTestConversationAvatarHandler(t)
	defer cleanup()

	database.CreateConversation("Test Chat", "thread_123")

	reqBody := AddAvatarRequest{AvatarID: 99999}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/conversations/1/avatars", bytes.NewReader(body))
	req.SetPathValue("id", "1")

	w := httptest.NewRecorder()
	handler.AddAvatar(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestRemoveAvatar(t *testing.T) {
	handler, database, cleanup := setupTestConversationAvatarHandler(t)
	defer cleanup()

	conv, _ := database.CreateConversation("Test Chat", "thread_123")
	avatar, _ := database.CreateAvatar("TestBot", "Prompt", "asst_123")
	database.AddAvatarToConversation(conv.ID, avatar.ID)

	req := httptest.NewRequest(http.MethodDelete, "/api/conversations/1/avatars/1", nil)
	req.SetPathValue("id", "1")
	req.SetPathValue("avatar_id", "1")

	w := httptest.NewRecorder()
	handler.RemoveAvatar(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Verify avatar was removed
	avatars, _ := database.GetConversationAvatars(conv.ID)
	if len(avatars) != 0 {
		t.Errorf("expected 0 avatars after removal, got %d", len(avatars))
	}
}

func TestRemoveAvatar_NotInConversation(t *testing.T) {
	handler, database, cleanup := setupTestConversationAvatarHandler(t)
	defer cleanup()

	database.CreateConversation("Test Chat", "thread_123")
	database.CreateAvatar("TestBot", "Prompt", "asst_123")
	// Note: avatar is NOT added to conversation

	req := httptest.NewRequest(http.MethodDelete, "/api/conversations/1/avatars/1", nil)
	req.SetPathValue("id", "1")
	req.SetPathValue("avatar_id", "1")

	w := httptest.NewRecorder()
	handler.RemoveAvatar(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestListConversationAvatars(t *testing.T) {
	handler, database, cleanup := setupTestConversationAvatarHandler(t)
	defer cleanup()

	conv, _ := database.CreateConversation("Test Chat", "thread_123")
	avatar1, _ := database.CreateAvatar("Bot1", "Prompt1", "asst_1")
	avatar2, _ := database.CreateAvatar("Bot2", "Prompt2", "asst_2")
	database.AddAvatarToConversation(conv.ID, avatar1.ID)
	database.AddAvatarToConversation(conv.ID, avatar2.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/conversations/1/avatars", nil)
	req.SetPathValue("id", "1")

	w := httptest.NewRecorder()
	handler.ListAvatars(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []AvatarResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("expected 2 avatars, got %d", len(response))
	}
}

func TestListConversationAvatars_ConversationNotFound(t *testing.T) {
	handler, _, cleanup := setupTestConversationAvatarHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/conversations/99999/avatars", nil)
	req.SetPathValue("id", "99999")

	w := httptest.NewRecorder()
	handler.ListAvatars(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestListConversationAvatars_Empty(t *testing.T) {
	handler, database, cleanup := setupTestConversationAvatarHandler(t)
	defer cleanup()

	database.CreateConversation("Test Chat", "thread_123")

	req := httptest.NewRequest(http.MethodGet, "/api/conversations/1/avatars", nil)
	req.SetPathValue("id", "1")

	w := httptest.NewRecorder()
	handler.ListAvatars(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []AvatarResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 0 {
		t.Errorf("expected 0 avatars, got %d", len(response))
	}
}
