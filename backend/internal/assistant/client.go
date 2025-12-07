package assistant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	baseURL        = "https://api.openai.com/v1"
	defaultModel   = "gpt-4o"
	defaultTimeout = 30 * time.Second
)

// Client provides access to OpenAI Assistants API
type Client struct {
	apiKey     string
	httpClient *http.Client
	model      string
}

// ClientOption configures the client
type ClientOption func(*Client)

// WithModel sets a custom model
func WithModel(model string) ClientOption {
	return func(c *Client) {
		c.model = model
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new OpenAI Assistants API client
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		model: defaultModel,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Assistant represents an OpenAI Assistant
type Assistant struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Instructions string `json:"instructions"`
	Model        string `json:"model"`
}

// CreateAssistantRequest represents a request to create an assistant
type CreateAssistantRequest struct {
	Name         string `json:"name"`
	Instructions string `json:"instructions"`
	Model        string `json:"model"`
}

// CreateAssistant creates a new assistant
func (c *Client) CreateAssistant(name, instructions string) (*Assistant, error) {
	log.Printf("[Assistant] CreateAssistant started name=%q model=%s", name, c.model)

	reqBody := CreateAssistantRequest{
		Name:         name,
		Instructions: instructions,
		Model:        c.model,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[Assistant] CreateAssistant failed: marshal request err=%v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/assistants", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Assistant] CreateAssistant failed: create request err=%v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Assistant] CreateAssistant failed: send request err=%v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Assistant] CreateAssistant failed: API error status=%d name=%q", resp.StatusCode, name)
		return nil, c.handleError(resp)
	}

	var assistant Assistant
	if err := json.NewDecoder(resp.Body).Decode(&assistant); err != nil {
		log.Printf("[Assistant] CreateAssistant failed: decode response err=%v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[Assistant] CreateAssistant completed assistant_id=%s name=%q", assistant.ID, assistant.Name)
	return &assistant, nil
}

// GetAssistant retrieves an assistant by ID
func (c *Client) GetAssistant(id string) (*Assistant, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/assistants/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var assistant Assistant
	if err := json.NewDecoder(resp.Body).Decode(&assistant); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &assistant, nil
}

// UpdateAssistantRequest represents a request to update an assistant
type UpdateAssistantRequest struct {
	Name         string `json:"name,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

// UpdateAssistant updates an existing assistant
func (c *Client) UpdateAssistant(id, name, instructions string) (*Assistant, error) {
	reqBody := UpdateAssistantRequest{
		Name:         name,
		Instructions: instructions,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/assistants/"+id, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var assistant Assistant
	if err := json.NewDecoder(resp.Body).Decode(&assistant); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &assistant, nil
}

// DeleteAssistant deletes an assistant
func (c *Client) DeleteAssistant(id string) error {
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/assistants/"+id, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// setHeaders sets the required headers for API requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "assistants=v2")
}

// APIError represents an error from the OpenAI API
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("OpenAI API error (status %d): %s", e.StatusCode, e.Message)
}

// handleError processes error responses from the API
func (c *Client) handleError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Truncate for logging if too long
	logBody := bodyStr
	if len(logBody) > 500 {
		logBody = logBody[:500] + "..."
	}
	log.Printf("[Assistant] API Error status=%d body=%s", resp.StatusCode, logBody)

	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    bodyStr,
	}
}

// SimpleCompletion sends a simple chat completion request for quick judgments
// Uses gpt-4o-mini for efficiency
func (c *Client) SimpleCompletion(prompt string) (string, error) {
	log.Printf("[Assistant] SimpleCompletion started prompt_length=%d", len(prompt))

	reqBody := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 10,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[Assistant] SimpleCompletion API error status=%d body=%s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("OpenAI API error: %s", string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	content := result.Choices[0].Message.Content
	log.Printf("[Assistant] SimpleCompletion completed response=%q", content)

	return content, nil
}
