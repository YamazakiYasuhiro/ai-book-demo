package assistant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Thread represents an OpenAI Thread
type Thread struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
}

// CreateThread creates a new thread
func (c *Client) CreateThread() (*Thread, error) {
	log.Printf("[Assistant] CreateThread started")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/threads", bytes.NewReader([]byte("{}")))
	if err != nil {
		log.Printf("[Assistant] CreateThread failed: create request err=%v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Assistant] CreateThread failed: send request err=%v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Assistant] CreateThread failed: API error status=%d", resp.StatusCode)
		return nil, c.handleError(resp)
	}

	var thread Thread
	if err := json.NewDecoder(resp.Body).Decode(&thread); err != nil {
		log.Printf("[Assistant] CreateThread failed: decode response err=%v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[Assistant] CreateThread completed thread_id=%s", thread.ID)
	return &thread, nil
}

// DeleteThread deletes a thread
func (c *Client) DeleteThread(id string) error {
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/threads/"+id, nil)
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

// Message represents an OpenAI Thread Message
type Message struct {
	ID        string           `json:"id"`
	Role      string           `json:"role"`
	Content   []MessageContent `json:"content"`
	CreatedAt int64            `json:"created_at"`
}

// MessageContent represents the content of a message
type MessageContent struct {
	Type string      `json:"type"`
	Text *TextObject `json:"text,omitempty"`
}

// TextObject represents text content
type TextObject struct {
	Value string `json:"value"`
}

// CreateMessageRequest represents a request to create a message
type CreateMessageRequest struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CreateMessage adds a message to a thread
func (c *Client) CreateMessage(threadID, content string) (*Message, error) {
	// Truncate content for logging
	contentPreview := content
	if len(contentPreview) > 50 {
		contentPreview = contentPreview[:50] + "..."
	}
	log.Printf("[Assistant] CreateMessage started thread_id=%s content_preview=%q", threadID, contentPreview)

	reqBody := CreateMessageRequest{
		Role:    "user",
		Content: content,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[Assistant] CreateMessage failed: marshal request err=%v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/threads/"+threadID+"/messages", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Assistant] CreateMessage failed: create request err=%v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Assistant] CreateMessage failed: send request err=%v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Assistant] CreateMessage failed: API error status=%d thread_id=%s", resp.StatusCode, threadID)
		return nil, c.handleError(resp)
	}

	var message Message
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		log.Printf("[Assistant] CreateMessage failed: decode response err=%v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[Assistant] CreateMessage completed thread_id=%s message_id=%s content_length=%d full_content=%q", threadID, message.ID, len(content), content)
	return &message, nil
}

// ListMessagesResponse represents the response from listing messages
type ListMessagesResponse struct {
	Data []Message `json:"data"`
}

// ListMessages retrieves messages from a thread
func (c *Client) ListMessages(threadID string) ([]Message, error) {
	log.Printf("[Assistant] ListMessages started thread_id=%s", threadID)

	req, err := http.NewRequest(http.MethodGet, baseURL+"/threads/"+threadID+"/messages", nil)
	if err != nil {
		log.Printf("[Assistant] ListMessages failed: create request err=%v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Assistant] ListMessages failed: send request err=%v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Assistant] ListMessages failed: API error status=%d thread_id=%s", resp.StatusCode, threadID)
		return nil, c.handleError(resp)
	}

	var listResp ListMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		log.Printf("[Assistant] ListMessages failed: decode response err=%v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[Assistant] ListMessages completed thread_id=%s message_count=%d", threadID, len(listResp.Data))
	return listResp.Data, nil
}

// Run represents an OpenAI Run
type Run struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	AssistantID string `json:"assistant_id"`
	ThreadID    string `json:"thread_id"`
}

// CreateRunRequest represents a request to create a run
type CreateRunRequest struct {
	AssistantID            string `json:"assistant_id"`
	AdditionalInstructions string `json:"additional_instructions,omitempty"`
}

// CreateRun creates a run to generate a response from an assistant
func (c *Client) CreateRun(threadID, assistantID string) (*Run, error) {
	log.Printf("[Assistant] CreateRun started thread_id=%s assistant_id=%s", threadID, assistantID)

	reqBody := CreateRunRequest{
		AssistantID: assistantID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[Assistant] CreateRun failed: marshal request err=%v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/threads/"+threadID+"/runs", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Assistant] CreateRun failed: create request err=%v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Assistant] CreateRun failed: send request err=%v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Assistant] CreateRun failed: API error status=%d thread_id=%s assistant_id=%s", resp.StatusCode, threadID, assistantID)
		return nil, c.handleError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		log.Printf("[Assistant] CreateRun failed: decode response err=%v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[Assistant] CreateRun completed run_id=%s status=%s", run.ID, run.Status)
	return &run, nil
}

// CreateRunWithContext creates a run with additional context/instructions
// The additionalInstructions parameter provides context like conversation history
func (c *Client) CreateRunWithContext(threadID, assistantID, additionalInstructions string) (*Run, error) {
	log.Printf("[Assistant] CreateRunWithContext started thread_id=%s assistant_id=%s context_length=%d additional_context=%q",
		threadID, assistantID, len(additionalInstructions), additionalInstructions)

	reqBody := CreateRunRequest{
		AssistantID:            assistantID,
		AdditionalInstructions: additionalInstructions,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[Assistant] CreateRunWithContext failed: marshal request err=%v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/threads/"+threadID+"/runs", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Assistant] CreateRunWithContext failed: create request err=%v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Assistant] CreateRunWithContext failed: send request err=%v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Assistant] CreateRunWithContext failed: API error status=%d thread_id=%s assistant_id=%s",
			resp.StatusCode, threadID, assistantID)
		return nil, c.handleError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		log.Printf("[Assistant] CreateRunWithContext failed: decode response err=%v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[Assistant] CreateRunWithContext completed run_id=%s status=%s", run.ID, run.Status)
	return &run, nil
}

// GetRun retrieves the status of a run
func (c *Client) GetRun(threadID, runID string) (*Run, error) {
	log.Printf("[Assistant] GetRun started thread_id=%s run_id=%s", threadID, runID)

	req, err := http.NewRequest(http.MethodGet, baseURL+"/threads/"+threadID+"/runs/"+runID, nil)
	if err != nil {
		log.Printf("[Assistant] GetRun failed: create request err=%v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Assistant] GetRun failed: send request err=%v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Assistant] GetRun failed: API error status=%d", resp.StatusCode)
		return nil, c.handleError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		log.Printf("[Assistant] GetRun failed: decode response err=%v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[Assistant] GetRun completed run_id=%s status=%s", run.ID, run.Status)
	return &run, nil
}

// WaitForRun polls until the run is complete
func (c *Client) WaitForRun(threadID, runID string, timeout time.Duration) (*Run, error) {
	log.Printf("[Assistant] WaitForRun started thread_id=%s run_id=%s timeout=%v", threadID, runID, timeout)
	deadline := time.Now().Add(timeout)
	pollCount := 0

	for time.Now().Before(deadline) {
		pollCount++
		run, err := c.GetRun(threadID, runID)
		if err != nil {
			log.Printf("[Assistant] WaitForRun failed: GetRun error err=%v", err)
			return nil, err
		}

		log.Printf("[Assistant] WaitForRun polling run_id=%s status=%s poll_count=%d", run.ID, run.Status, pollCount)

		switch run.Status {
		case "completed":
			log.Printf("[Assistant] WaitForRun completed run_id=%s status=completed poll_count=%d", run.ID, pollCount)
			return run, nil
		case "failed", "cancelled", "expired":
			log.Printf("[Assistant] WaitForRun failed: run ended status=%s run_id=%s", run.Status, run.ID)
			return run, fmt.Errorf("run ended with status: %s", run.Status)
		}

		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("[Assistant] WaitForRun timeout run_id=%s poll_count=%d", runID, pollCount)
	return nil, fmt.Errorf("timeout waiting for run to complete")
}

// CancelRun cancels a running run
func (c *Client) CancelRun(threadID, runID string) error {
	log.Printf("[Assistant] CancelRun started thread_id=%s run_id=%s", threadID, runID)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/threads/"+threadID+"/runs/"+runID+"/cancel", nil)
	if err != nil {
		log.Printf("[Assistant] CancelRun failed: create request err=%v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Assistant] CancelRun failed: send request err=%v", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Assistant] CancelRun failed: API error status=%d thread_id=%s run_id=%s", resp.StatusCode, threadID, runID)
		return c.handleError(resp)
	}

	log.Printf("[Assistant] CancelRun completed thread_id=%s run_id=%s", threadID, runID)
	return nil
}

// GetLatestAssistantMessage retrieves the most recent assistant message from a thread
func (c *Client) GetLatestAssistantMessage(threadID string) (string, error) {
	log.Printf("[Assistant] GetLatestAssistantMessage started thread_id=%s", threadID)

	messages, err := c.ListMessages(threadID)
	if err != nil {
		log.Printf("[Assistant] GetLatestAssistantMessage failed: list messages err=%v", err)
		return "", err
	}

	log.Printf("[Assistant] GetLatestAssistantMessage found %d messages", len(messages))

	// Messages are returned in reverse chronological order
	// Find the first assistant message
	for _, msg := range messages {
		if msg.Role == "assistant" && len(msg.Content) > 0 {
			for _, content := range msg.Content {
				if content.Type == "text" && content.Text != nil {
					log.Printf("[Assistant] GetLatestAssistantMessage found message_id=%s", msg.ID)
					return content.Text.Value, nil
				}
			}
		}
	}

	log.Printf("[Assistant] GetLatestAssistantMessage: no assistant message found")
	return "", fmt.Errorf("no assistant message found in thread")
}
