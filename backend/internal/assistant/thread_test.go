package assistant

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateThread_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/threads" {
			t.Errorf("expected path '/threads', got %s", r.URL.Path)
		}

		resp := Thread{
			ID:        "thread_123",
			CreatedAt: 1234567890,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	testClient := &testableThreadClient{
		Client:  client,
		baseURL: server.URL,
	}

	thread, err := testClient.CreateThread()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if thread.ID != "thread_123" {
		t.Errorf("expected ID 'thread_123', got '%s'", thread.ID)
	}
}

func TestCreateMessage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/threads/thread_123/messages" {
			t.Errorf("expected path '/threads/thread_123/messages', got %s", r.URL.Path)
		}

		resp := Message{
			ID:   "msg_123",
			Role: "user",
			Content: []MessageContent{
				{
					Type: "text",
					Text: &TextObject{Value: "Hello"},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	testClient := &testableThreadClient{
		Client:  client,
		baseURL: server.URL,
	}

	msg, err := testClient.CreateMessage("thread_123", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.ID != "msg_123" {
		t.Errorf("expected ID 'msg_123', got '%s'", msg.ID)
	}
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got '%s'", msg.Role)
	}
}

func TestListMessages_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}

		resp := ListMessagesResponse{
			Data: []Message{
				{ID: "msg_1", Role: "user"},
				{ID: "msg_2", Role: "assistant"},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	testClient := &testableThreadClient{
		Client:  client,
		baseURL: server.URL,
	}

	messages, err := testClient.ListMessages("thread_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestCreateRun_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		resp := Run{
			ID:          "run_123",
			Status:      "queued",
			AssistantID: "asst_123",
			ThreadID:    "thread_123",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	testClient := &testableThreadClient{
		Client:  client,
		baseURL: server.URL,
	}

	run, err := testClient.CreateRun("thread_123", "asst_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if run.ID != "run_123" {
		t.Errorf("expected ID 'run_123', got '%s'", run.ID)
	}
	if run.Status != "queued" {
		t.Errorf("expected status 'queued', got '%s'", run.Status)
	}
}

// testableThreadClient wraps Client for testing thread operations
type testableThreadClient struct {
	*Client
	baseURL string
}

func (tc *testableThreadClient) CreateThread() (*Thread, error) {
	req, err := http.NewRequest(http.MethodPost, tc.baseURL+"/threads", bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	tc.setHeaders(req)

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, tc.handleError(resp)
	}

	var thread Thread
	if err := json.NewDecoder(resp.Body).Decode(&thread); err != nil {
		return nil, err
	}
	return &thread, nil
}

func (tc *testableThreadClient) CreateMessage(threadID, content string) (*Message, error) {
	reqBody := CreateMessageRequest{
		Role:    "user",
		Content: content,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, tc.baseURL+"/threads/"+threadID+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	tc.setHeaders(req)

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, tc.handleError(resp)
	}

	var message Message
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return nil, err
	}
	return &message, nil
}

func (tc *testableThreadClient) ListMessages(threadID string) ([]Message, error) {
	req, err := http.NewRequest(http.MethodGet, tc.baseURL+"/threads/"+threadID+"/messages", nil)
	if err != nil {
		return nil, err
	}
	tc.setHeaders(req)

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, tc.handleError(resp)
	}

	var listResp ListMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}
	return listResp.Data, nil
}

func (tc *testableThreadClient) CreateRun(threadID, assistantID string) (*Run, error) {
	reqBody := CreateRunRequest{AssistantID: assistantID}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, tc.baseURL+"/threads/"+threadID+"/runs", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	tc.setHeaders(req)

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, tc.handleError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, err
	}
	return &run, nil
}

func TestCreateRunWithContext_Success(t *testing.T) {
	var receivedInstructions string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Parse request body to check additional_instructions
		var reqBody CreateRunRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		receivedInstructions = reqBody.AdditionalInstructions

		resp := Run{
			ID:          "run_123",
			Status:      "queued",
			AssistantID: "asst_123",
			ThreadID:    "thread_123",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	testClient := &testableThreadClient{
		Client:  client,
		baseURL: server.URL,
	}

	contextInfo := "Previous messages:\nUser: Hello\nAssistant: Hi there!"
	run, err := testClient.CreateRunWithContext("thread_123", "asst_123", contextInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if run.ID != "run_123" {
		t.Errorf("expected ID 'run_123', got '%s'", run.ID)
	}

	if receivedInstructions != contextInfo {
		t.Errorf("expected additional_instructions '%s', got '%s'", contextInfo, receivedInstructions)
	}
}

func (tc *testableThreadClient) CreateRunWithContext(threadID, assistantID, additionalInstructions string) (*Run, error) {
	reqBody := CreateRunRequest{
		AssistantID:            assistantID,
		AdditionalInstructions: additionalInstructions,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, tc.baseURL+"/threads/"+threadID+"/runs", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	tc.setHeaders(req)

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, tc.handleError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, err
	}
	return &run, nil
}
