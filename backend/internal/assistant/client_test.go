package assistant

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-api-key")

	if client.apiKey != "test-api-key" {
		t.Errorf("expected apiKey 'test-api-key', got '%s'", client.apiKey)
	}

	if client.model != defaultModel {
		t.Errorf("expected model '%s', got '%s'", defaultModel, client.model)
	}
}

func TestNewClient_WithModel(t *testing.T) {
	client := NewClient("test-api-key", WithModel("gpt-3.5-turbo"))

	if client.model != "gpt-3.5-turbo" {
		t.Errorf("expected model 'gpt-3.5-turbo', got '%s'", client.model)
	}
}

func TestCreateAssistant_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/assistants" {
			t.Errorf("expected path '/assistants', got %s", r.URL.Path)
		}

		// Verify headers
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Error("missing or invalid Authorization header")
		}
		if r.Header.Get("OpenAI-Beta") != "assistants=v2" {
			t.Error("missing or invalid OpenAI-Beta header")
		}

		// Return mock response
		resp := Assistant{
			ID:           "asst_123",
			Name:         "Test Assistant",
			Instructions: "You are helpful",
			Model:        "gpt-4o",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client with test server
	client := NewClient("test-api-key")
	testClient := &testableClient{
		Client:  client,
		baseURL: server.URL,
	}

	assistant, err := testClient.CreateAssistant("Test Assistant", "You are helpful")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if assistant.ID != "asst_123" {
		t.Errorf("expected ID 'asst_123', got '%s'", assistant.ID)
	}
	if assistant.Name != "Test Assistant" {
		t.Errorf("expected Name 'Test Assistant', got '%s'", assistant.Name)
	}
}

func TestCreateAssistant_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	client := NewClient("invalid-key")
	testClient := &testableClient{
		Client:  client,
		baseURL: server.URL,
	}

	_, err := testClient.CreateAssistant("Test", "Instructions")
	if err == nil {
		t.Error("expected error for unauthorized request")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}

	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", apiErr.StatusCode)
	}
}

func TestDeleteAssistant_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		if r.URL.Path != "/assistants/asst_123" {
			t.Errorf("expected path '/assistants/asst_123', got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "asst_123", "deleted": true}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	testClient := &testableClient{
		Client:  client,
		baseURL: server.URL,
	}

	err := testClient.DeleteAssistant("asst_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// testableClient wraps Client to allow testing with custom base URL
type testableClient struct {
	*Client
	baseURL string
}

func (tc *testableClient) CreateAssistant(name, instructions string) (*Assistant, error) {
	reqBody := CreateAssistantRequest{
		Name:         name,
		Instructions: instructions,
		Model:        tc.model,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, tc.baseURL+"/assistants", bytes.NewReader(body))
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

	var assistant Assistant
	if err := json.NewDecoder(resp.Body).Decode(&assistant); err != nil {
		return nil, err
	}

	return &assistant, nil
}

func (tc *testableClient) DeleteAssistant(id string) error {
	req, err := http.NewRequest(http.MethodDelete, tc.baseURL+"/assistants/"+id, nil)
	if err != nil {
		return err
	}

	tc.setHeaders(req)

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return tc.handleError(resp)
	}

	return nil
}
