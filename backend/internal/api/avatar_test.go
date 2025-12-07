package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"multi-avatar-chat/internal/assistant"
	"multi-avatar-chat/internal/db"
)

func setupTestAvatarHandler(t *testing.T) (*AvatarHandler, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test_avatar_*.db")
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

	handler := NewAvatarHandler(database, nil) // nil assistant for testing

	cleanup := func() {
		database.Close()
		os.Remove(tmpFile.Name())
	}

	return handler, cleanup
}

func TestCreateAvatar_Success(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	body := `{"name": "TestBot", "prompt": "You are helpful"}`
	req := httptest.NewRequest(http.MethodPost, "/api/avatars", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response AvatarResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "TestBot" {
		t.Errorf("expected name 'TestBot', got '%s'", response.Name)
	}
	if response.Prompt != "You are helpful" {
		t.Errorf("expected prompt 'You are helpful', got '%s'", response.Prompt)
	}
	if response.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestCreateAvatar_MissingFields(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	testCases := []struct {
		name string
		body string
	}{
		{"missing name", `{"prompt": "test"}`},
		{"missing prompt", `{"name": "test"}`},
		{"empty body", `{}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/avatars", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Create(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestListAvatars_Empty(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/avatars", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

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

func TestListAvatars_WithData(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	// Create test avatars
	createBody := `{"name": "Avatar1", "prompt": "Prompt 1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/avatars", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	createBody = `{"name": "Avatar2", "prompt": "Prompt 2"}`
	req = httptest.NewRequest(http.MethodPost, "/api/avatars", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.Create(w, req)

	// List avatars
	req = httptest.NewRequest(http.MethodGet, "/api/avatars", nil)
	w = httptest.NewRecorder()
	handler.List(w, req)

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

func TestGetAvatar_Success(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	// Create test avatar
	createBody := `{"name": "GetTest", "prompt": "Test prompt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/avatars", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	var created AvatarResponse
	json.NewDecoder(w.Body).Decode(&created)

	// Get avatar
	req = httptest.NewRequest(http.MethodGet, "/api/avatars/1", nil)
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response AvatarResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "GetTest" {
		t.Errorf("expected name 'GetTest', got '%s'", response.Name)
	}
}

func TestGetAvatar_NotFound(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/avatars/99999", nil)
	req.SetPathValue("id", "99999")
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUpdateAvatar_Success(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	// Create test avatar
	createBody := `{"name": "Original", "prompt": "Original prompt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/avatars", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Update avatar
	updateBody := `{"name": "Updated", "prompt": "Updated prompt"}`
	req = httptest.NewRequest(http.MethodPut, "/api/avatars/1", bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response AvatarResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "Updated" {
		t.Errorf("expected name 'Updated', got '%s'", response.Name)
	}
	if response.Prompt != "Updated prompt" {
		t.Errorf("expected prompt 'Updated prompt', got '%s'", response.Prompt)
	}
}

func TestDeleteAvatar_Success(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	// Create test avatar
	createBody := `{"name": "ToDelete", "prompt": "Delete me"}`
	req := httptest.NewRequest(http.MethodPost, "/api/avatars", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Delete avatar
	req = httptest.NewRequest(http.MethodDelete, "/api/avatars/1", nil)
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Verify deletion
	req = httptest.NewRequest(http.MethodGet, "/api/avatars/1", nil)
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d after deletion, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDeleteAvatar_NotFound(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/api/avatars/99999", nil)
	req.SetPathValue("id", "99999")
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestCreateAvatar_AddsUserPriorityPrompt(t *testing.T) {
	handler, cleanup := setupTestAvatarHandler(t)
	defer cleanup()

	// Create a mock HTTP server that captures the request body
	var capturedInstructions string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Path == "/v1/assistants" || r.URL.Path == "/assistants") && r.Method == http.MethodPost {
			// Decode request body to capture instructions
			var reqBody struct {
				Name         string `json:"name"`
				Instructions string `json:"instructions"`
				Model        string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
				capturedInstructions = reqBody.Instructions
			}

			// Return mock response
			resp := assistant.Assistant{
				ID:           "asst_test",
				Name:         reqBody.Name,
				Instructions: reqBody.Instructions,
				Model:        reqBody.Model,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer mockServer.Close()

	// Create assistant client pointing to mock server
	httpClient := &http.Client{
		Transport: &mockTransport{baseURL: mockServer.URL},
	}
	assistantClient := assistant.NewClient("test-api-key", assistant.WithHTTPClient(httpClient))
	handler.assistant = assistantClient

	body := `{"name": "TestBot", "prompt": "You are helpful"}`
	req := httptest.NewRequest(http.MethodPost, "/api/avatars", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	// Verify that the instructions contain the user priority prompt
	expectedPrefix := "【重要】`Name: ユーザ` となっているメッセージがユーザの意見です。"
	if !strings.Contains(capturedInstructions, expectedPrefix) {
		t.Errorf("expected instructions to contain user priority prompt, got: %s", capturedInstructions)
	}

	// Verify that the original prompt is still included
	if !strings.Contains(capturedInstructions, "You are helpful") {
		t.Errorf("expected instructions to contain original prompt, got: %s", capturedInstructions)
	}
}

// mockTransport redirects requests to a mock server
type mockTransport struct {
	baseURL string
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect OpenAI API calls to mock server
	// Parse the baseURL to extract host and port
	baseURL := t.baseURL
	if strings.HasPrefix(baseURL, "http://") {
		baseURL = baseURL[7:]
	} else if strings.HasPrefix(baseURL, "https://") {
		baseURL = baseURL[8:]
	}
	
	// Extract host and port
	parts := strings.Split(baseURL, "/")
	host := parts[0]
	
	// Create new request with mock server URL
	// Remove /v1 prefix from path if present, as mock server handles both
	path := req.URL.Path
	if strings.HasPrefix(path, "/v1") {
		path = path[3:]
	}
	newURL := "http://" + host + path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}
	
	newReq, err := http.NewRequest(req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	
	// Copy headers
	for k, v := range req.Header {
		newReq.Header[k] = v
	}
	
	return http.DefaultTransport.RoundTrip(newReq)
}

