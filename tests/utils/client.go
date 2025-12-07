package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TestClient wraps HTTP client for integration tests
type TestClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewTestClient creates a new test client
func NewTestClient(baseURL string) *TestClient {
	return &TestClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GET performs a GET request
func (c *TestClient) GET(path string) (*http.Response, error) {
	return c.HTTPClient.Get(c.BaseURL + path)
}

// POST performs a POST request with JSON body
func (c *TestClient) POST(path string, body any) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	return c.HTTPClient.Post(
		c.BaseURL+path,
		"application/json",
		bytes.NewReader(jsonBody),
	)
}

// PUT performs a PUT request with JSON body
func (c *TestClient) PUT(path string, body any) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, c.BaseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.HTTPClient.Do(req)
}

// DELETE performs a DELETE request
func (c *TestClient) DELETE(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, c.BaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	return c.HTTPClient.Do(req)
}

// ReadJSON reads response body as JSON into target
func ReadJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	return json.Unmarshal(body, target)
}

// ReadBody reads response body as string
func ReadBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}
	return string(body), nil
}

