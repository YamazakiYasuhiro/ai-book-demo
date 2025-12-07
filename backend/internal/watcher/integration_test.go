package watcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"multi-avatar-chat/internal/assistant"
	"multi-avatar-chat/internal/db"
	"multi-avatar-chat/internal/models"
)

// MockOpenAIServer simulates OpenAI API for integration testing
type MockOpenAIServer struct {
	server       *httptest.Server
	mutex        sync.Mutex
	activeRuns   map[string]string // threadID -> runID
	runStatuses  map[string]string // runID -> status
	messages     map[string][]mockMessage
	runCounter   int
	msgCounter   int
	responseText string
}

type mockMessage struct {
	ID      string
	Role    string
	Content string
}

func newMockOpenAIServer() *MockOpenAIServer {
	m := &MockOpenAIServer{
		activeRuns:   make(map[string]string),
		runStatuses:  make(map[string]string),
		messages:     make(map[string][]mockMessage),
		responseText: "This is a mock response from the avatar.",
	}

	mux := http.NewServeMux()

	// Create thread endpoint
	mux.HandleFunc("POST /v1/threads", func(w http.ResponseWriter, r *http.Request) {
		m.handleCreateThread(w, r)
	})

	// List runs endpoint
	mux.HandleFunc("/v1/threads/", func(w http.ResponseWriter, r *http.Request) {
		m.handleThreadsRequest(w, r)
	})

	// Chat completions endpoint (for SimpleCompletion)
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		m.handleChatCompletion(w, r)
	})

	m.server = httptest.NewServer(mux)
	return m
}

func (m *MockOpenAIServer) handleCreateThread(w http.ResponseWriter, r *http.Request) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.msgCounter++
	threadID := "thread_mock_" + string(rune('0'+m.msgCounter))
	m.messages[threadID] = []mockMessage{}
	json.NewEncoder(w).Encode(map[string]any{
		"id":         threadID,
		"created_at": int64(1234567890),
	})
}

func (m *MockOpenAIServer) handleThreadsRequest(w http.ResponseWriter, r *http.Request) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	path := r.URL.Path

	// Extract thread ID from path: /v1/threads/{thread_id}/...
	// Pattern: /v1/threads/thread_123/runs or /v1/threads/thread_123/messages
	var threadID string
	if len(path) > 12 { // "/v1/threads/"
		rest := path[12:]
		for i, c := range rest {
			if c == '/' {
				threadID = rest[:i]
				break
			}
		}
		if threadID == "" {
			threadID = rest
		}
	}

	// Handle /v1/threads/{thread_id}/runs
	if r.Method == "GET" && containsStr(path, "/runs") && !containsStr(path, "/runs/") {
		// List runs
		runs := []map[string]string{}
		if runID, ok := m.activeRuns[threadID]; ok {
			status := m.runStatuses[runID]
			if status == "" {
				status = "completed"
			}
			runs = append(runs, map[string]string{
				"id":           runID,
				"status":       status,
				"thread_id":    threadID,
				"assistant_id": "asst_mock",
			})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": runs})
		return
	}

	// Handle POST /v1/threads/{thread_id}/runs - Create run
	if r.Method == "POST" && containsStr(path, "/runs") && !containsStr(path, "/runs/") {
		// Check if there's already an active run
		if existingRunID, ok := m.activeRuns[threadID]; ok {
			status := m.runStatuses[existingRunID]
			if status == "in_progress" || status == "queued" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]string{
						"message": "Thread " + threadID + " already has an active run " + existingRunID + ".",
						"type":    "invalid_request_error",
					},
				})
				return
			}
		}

		m.runCounter++
		runID := "run_mock_" + string(rune('0'+m.runCounter))
		m.activeRuns[threadID] = runID
		m.runStatuses[runID] = "queued"

		// Simulate run completion after a short delay
		go func(rid string) {
			time.Sleep(100 * time.Millisecond)
			m.mutex.Lock()
			m.runStatuses[rid] = "in_progress"
			m.mutex.Unlock()

			time.Sleep(100 * time.Millisecond)
			m.mutex.Lock()
			m.runStatuses[rid] = "completed"
			// Add assistant message
			m.msgCounter++
			msgID := "msg_mock_" + string(rune('0'+m.msgCounter))
			m.messages[threadID] = append([]mockMessage{{
				ID:      msgID,
				Role:    "assistant",
				Content: m.responseText,
			}}, m.messages[threadID]...)
			m.mutex.Unlock()
		}(runID)

		json.NewEncoder(w).Encode(map[string]string{
			"id":           runID,
			"status":       "queued",
			"thread_id":    threadID,
			"assistant_id": "asst_mock",
		})
		return
	}

	// Handle GET /v1/threads/{thread_id}/runs/{run_id} - Get run status
	if r.Method == "GET" && containsStr(path, "/runs/") {
		// Extract run ID
		parts := splitPath(path)
		var runID string
		for i, p := range parts {
			if p == "runs" && i+1 < len(parts) {
				runID = parts[i+1]
				break
			}
		}

		status := m.runStatuses[runID]
		if status == "" {
			status = "completed"
		}

		json.NewEncoder(w).Encode(map[string]string{
			"id":           runID,
			"status":       status,
			"thread_id":    threadID,
			"assistant_id": "asst_mock",
		})
		return
	}

	// Handle GET /v1/threads/{thread_id}/messages - List messages
	if r.Method == "GET" && containsStr(path, "/messages") {
		msgs := m.messages[threadID]
		data := make([]map[string]any, 0, len(msgs))
		for _, msg := range msgs {
			data = append(data, map[string]any{
				"id":   msg.ID,
				"role": msg.Role,
				"content": []map[string]any{
					{
						"type": "text",
						"text": map[string]string{
							"value": msg.Content,
						},
					},
				},
			})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
		return
	}

	// Handle POST /v1/threads/{thread_id}/messages - Create message
	if r.Method == "POST" && containsStr(path, "/messages") {
		// Check if there's an active run
		if runID, ok := m.activeRuns[threadID]; ok {
			status := m.runStatuses[runID]
			if status == "in_progress" || status == "queued" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]string{
						"message": "Can't add messages to " + threadID + " while a run " + runID + " is active.",
						"type":    "invalid_request_error",
					},
				})
				return
			}
		}

		var req struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		m.msgCounter++
		msgID := "msg_mock_" + string(rune('0'+m.msgCounter))
		m.messages[threadID] = append(m.messages[threadID], mockMessage{
			ID:      msgID,
			Role:    req.Role,
			Content: req.Content,
		})

		json.NewEncoder(w).Encode(map[string]string{
			"id":   msgID,
			"role": req.Role,
		})
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (m *MockOpenAIServer) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	// Always respond "yes" to simulate avatar wanting to respond
	json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]string{
					"content": "yes",
				},
			},
		},
	})
}

func (m *MockOpenAIServer) Close() {
	m.server.Close()
}

func (m *MockOpenAIServer) URL() string {
	return m.server.URL
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// Helper to create a test assistant client with mock server
func createMockAssistantClient(serverURL string) *assistant.Client {
	// Create client with custom HTTP client pointing to mock server
	httpClient := &http.Client{
		Transport: &mockTransport{baseURL: serverURL},
	}
	return assistant.NewClient("mock-api-key", assistant.WithHTTPClient(httpClient))
}

type mockTransport struct {
	baseURL string
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect OpenAI API calls to mock server
	req.URL.Scheme = "http"
	req.URL.Host = t.baseURL[7:] // Remove "http://"
	return http.DefaultTransport.RoundTrip(req)
}

// Integration Tests

func TestIntegration_WatcherRespondsToNewMessage(t *testing.T) {
	// Setup mock OpenAI server
	mockServer := newMockOpenAIServer()
	defer mockServer.Close()

	// Setup database
	tmpFile, _ := os.CreateTemp("", "integration_test_*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	database, _ := db.NewDB(tmpFile.Name())
	defer database.Close()
	database.Migrate()

	// Create assistant client with mock
	assistantClient := createMockAssistantClient(mockServer.URL())

	// Create conversation with thread
	conv, _ := database.CreateConversation("Integration Test Chat", "thread_integration_1")

	// Create avatar
	avatar, _ := database.CreateAvatar("IntegrationBot", "Helpful assistant", "asst_integration")
	// Create thread for avatar
	thread, _ := assistantClient.CreateThread()
	database.AddAvatarToConversationWithThreadID(conv.ID, avatar.ID, thread.ID)

	// Create WatcherManager with short interval
	manager := NewManager(database, assistantClient, 200*time.Millisecond)
	defer manager.Shutdown()

	// Initialize watchers BEFORE sending any messages
	ctx := context.Background()
	manager.InitializeAll(ctx)

	if manager.WatcherCount() != 1 {
		t.Fatalf("expected 1 watcher, got %d", manager.WatcherCount())
	}

	// Wait for watchers to fully initialize (including lastMessageID)
	time.Sleep(300 * time.Millisecond)

	// Simulate user sending a message AFTER watcher is fully initialized
	database.CreateMessage(conv.ID, models.SenderTypeUser, nil, "@IntegrationBot please respond")

	// Wait for watcher to detect and respond (with timeout)
	deadline := time.Now().Add(5 * time.Second)
	var messages []models.Message
	for time.Now().Before(deadline) {
		messages, _ = database.GetMessages(conv.ID)
		// Look for avatar response
		for _, msg := range messages {
			if msg.SenderType == models.SenderTypeAvatar {
				t.Logf("Avatar responded with message ID %d", msg.ID)
				return // Success!
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Errorf("Avatar did not respond within timeout. Messages: %d", len(messages))
}

func TestIntegration_MultipleWatchersNoConflict(t *testing.T) {
	// Setup mock OpenAI server
	mockServer := newMockOpenAIServer()
	defer mockServer.Close()

	// Setup database
	tmpFile, _ := os.CreateTemp("", "integration_multi_*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	database, _ := db.NewDB(tmpFile.Name())
	defer database.Close()
	database.Migrate()

	// Create assistant client with mock
	assistantClient := createMockAssistantClient(mockServer.URL())

	// Create conversation
	conv, _ := database.CreateConversation("Multi Watcher Test", "thread_multi_1")

	// Create two avatars
	avatar1, _ := database.CreateAvatar("Bot1", "First bot", "asst_1")
	avatar2, _ := database.CreateAvatar("Bot2", "Second bot", "asst_2")
	// Create threads for avatars
	thread1, _ := assistantClient.CreateThread()
	thread2, _ := assistantClient.CreateThread()
	database.AddAvatarToConversationWithThreadID(conv.ID, avatar1.ID, thread1.ID)
	database.AddAvatarToConversationWithThreadID(conv.ID, avatar2.ID, thread2.ID)

	// Create WatcherManager
	manager := NewManager(database, assistantClient, 200*time.Millisecond)
	defer manager.Shutdown()

	ctx := context.Background()
	manager.InitializeAll(ctx)

	if manager.WatcherCount() != 2 {
		t.Fatalf("expected 2 watchers, got %d", manager.WatcherCount())
	}

	// Wait for watchers to fully initialize
	time.Sleep(300 * time.Millisecond)

	// Simulate user message mentioning both bots
	database.CreateMessage(conv.ID, models.SenderTypeUser, nil, "@Bot1 @Bot2 hello everyone")

	// Wait for both avatars to respond
	deadline := time.Now().Add(10 * time.Second)
	respondedAvatars := make(map[int64]bool)

	for time.Now().Before(deadline) {
		messages, _ := database.GetMessages(conv.ID)
		for _, msg := range messages {
			if msg.SenderType == models.SenderTypeAvatar && msg.SenderID != nil {
				respondedAvatars[*msg.SenderID] = true
			}
		}

		if len(respondedAvatars) >= 2 {
			t.Logf("Both avatars responded successfully")
			return // Success!
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Errorf("Not all avatars responded. Responded: %v", respondedAvatars)
}

func TestIntegration_DynamicAvatarJoinLeave(t *testing.T) {
	// Setup mock OpenAI server
	mockServer := newMockOpenAIServer()
	defer mockServer.Close()

	// Setup database
	tmpFile, _ := os.CreateTemp("", "integration_dynamic_*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	database, _ := db.NewDB(tmpFile.Name())
	defer database.Close()
	database.Migrate()

	// Create assistant client with mock
	assistantClient := createMockAssistantClient(mockServer.URL())

	// Create conversation
	conv, _ := database.CreateConversation("Dynamic Join Test", "thread_dynamic_1")

	// Create avatar
	avatar, _ := database.CreateAvatar("DynamicBot", "Dynamic assistant", "asst_dynamic")

	// Create WatcherManager (no avatars yet)
	manager := NewManager(database, assistantClient, 200*time.Millisecond)
	defer manager.Shutdown()

	if manager.WatcherCount() != 0 {
		t.Fatalf("expected 0 watchers initially, got %d", manager.WatcherCount())
	}

	// Add avatar to conversation and start watcher
	thread, _ := assistantClient.CreateThread()
	database.AddAvatarToConversationWithThreadID(conv.ID, avatar.ID, thread.ID)
	manager.StartWatcher(conv.ID, avatar.ID)

	if manager.WatcherCount() != 1 {
		t.Fatalf("expected 1 watcher after adding, got %d", manager.WatcherCount())
	}

	// Verify watcher is running
	if !manager.HasWatcher(conv.ID, avatar.ID) {
		t.Error("expected watcher to exist for conversation/avatar")
	}

	// Remove avatar and stop watcher
	manager.StopWatcher(conv.ID, avatar.ID)
	database.RemoveAvatarFromConversation(conv.ID, avatar.ID)

	if manager.WatcherCount() != 0 {
		t.Fatalf("expected 0 watchers after removal, got %d", manager.WatcherCount())
	}
}

func TestIntegration_GracefulShutdown(t *testing.T) {
	// Setup mock OpenAI server
	mockServer := newMockOpenAIServer()
	defer mockServer.Close()

	// Setup database
	tmpFile, _ := os.CreateTemp("", "integration_shutdown_*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	database, _ := db.NewDB(tmpFile.Name())
	defer database.Close()
	database.Migrate()

	// Create assistant client with mock
	assistantClient := createMockAssistantClient(mockServer.URL())

	// Create multiple conversations with avatars
	conv1, _ := database.CreateConversation("Shutdown Test 1", "thread_shutdown_1")
	conv2, _ := database.CreateConversation("Shutdown Test 2", "thread_shutdown_2")
	avatar1, _ := database.CreateAvatar("ShutdownBot1", "Bot 1", "asst_s1")
	avatar2, _ := database.CreateAvatar("ShutdownBot2", "Bot 2", "asst_s2")
	// Create threads for avatars
	thread1, _ := assistantClient.CreateThread()
	thread2, _ := assistantClient.CreateThread()
	thread3, _ := assistantClient.CreateThread()
	database.AddAvatarToConversationWithThreadID(conv1.ID, avatar1.ID, thread1.ID)
	database.AddAvatarToConversationWithThreadID(conv1.ID, avatar2.ID, thread2.ID)
	database.AddAvatarToConversationWithThreadID(conv2.ID, avatar1.ID, thread3.ID)

	// Create and initialize WatcherManager
	manager := NewManager(database, assistantClient, 200*time.Millisecond)

	ctx := context.Background()
	manager.InitializeAll(ctx)

	if manager.WatcherCount() != 3 {
		t.Fatalf("expected 3 watchers, got %d", manager.WatcherCount())
	}

	// Test graceful shutdown
	done := make(chan struct{})
	go func() {
		manager.Shutdown()
		close(done)
	}()

	// Shutdown should complete within reasonable time
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown timed out")
	}

	if manager.WatcherCount() != 0 {
		t.Errorf("expected 0 watchers after shutdown, got %d", manager.WatcherCount())
	}
}

func TestIntegration_MentionTriggersResponse(t *testing.T) {
	// Setup mock OpenAI server
	mockServer := newMockOpenAIServer()
	defer mockServer.Close()

	// Setup database
	tmpFile, _ := os.CreateTemp("", "integration_mention_*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	database, _ := db.NewDB(tmpFile.Name())
	defer database.Close()
	database.Migrate()

	// Create assistant client with mock
	assistantClient := createMockAssistantClient(mockServer.URL())

	// Create conversation
	conv, _ := database.CreateConversation("Mention Test", "thread_mention_1")

	// Create avatar with Japanese name
	avatar, _ := database.CreateAvatar("太郎", "Japanese assistant", "asst_taro")
	// Create thread for avatar
	thread, _ := assistantClient.CreateThread()
	database.AddAvatarToConversationWithThreadID(conv.ID, avatar.ID, thread.ID)

	// Create WatcherManager
	manager := NewManager(database, assistantClient, 200*time.Millisecond)
	defer manager.Shutdown()

	ctx := context.Background()
	manager.InitializeAll(ctx)

	// Wait for watchers to fully initialize
	time.Sleep(300 * time.Millisecond)

	// Send message with Japanese mention
	database.CreateMessage(conv.ID, models.SenderTypeUser, nil, "@太郎 質問があります")

	// Wait for response
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		messages, _ := database.GetMessages(conv.ID)
		for _, msg := range messages {
			if msg.SenderType == models.SenderTypeAvatar {
				t.Logf("Japanese-named avatar responded successfully")
				return // Success!
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Error("Avatar with Japanese name did not respond to mention")
}

