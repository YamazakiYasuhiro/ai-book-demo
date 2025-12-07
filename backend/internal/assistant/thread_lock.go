package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// ThreadLockManager manages exclusive access to OpenAI threads
// OpenAI Assistants API only allows one active run per thread at a time
type ThreadLockManager struct {
	locks map[string]*sync.Mutex
	mu    sync.Mutex
}

// NewThreadLockManager creates a new ThreadLockManager
func NewThreadLockManager() *ThreadLockManager {
	return &ThreadLockManager{
		locks: make(map[string]*sync.Mutex),
	}
}

// getLock returns the mutex for a specific thread, creating one if needed
func (m *ThreadLockManager) getLock(threadID string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.locks[threadID] == nil {
		m.locks[threadID] = &sync.Mutex{}
	}
	return m.locks[threadID]
}

// Lock acquires the lock for a thread
func (m *ThreadLockManager) Lock(threadID string) {
	lock := m.getLock(threadID)
	lock.Lock()
	log.Printf("[ThreadLock] Acquired lock thread_id=%s", threadID)
}

// Unlock releases the lock for a thread
func (m *ThreadLockManager) Unlock(threadID string) {
	lock := m.getLock(threadID)
	lock.Unlock()
	log.Printf("[ThreadLock] Released lock thread_id=%s", threadID)
}

// TryLockWithTimeout attempts to acquire the lock with a timeout
func (m *ThreadLockManager) TryLockWithTimeout(ctx context.Context, threadID string, timeout time.Duration) error {
	lock := m.getLock(threadID)

	done := make(chan struct{})
	go func() {
		lock.Lock()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[ThreadLock] Acquired lock with timeout thread_id=%s", threadID)
		return nil
	case <-time.After(timeout):
		log.Printf("[ThreadLock] Timeout acquiring lock thread_id=%s timeout=%v", threadID, timeout)
		return fmt.Errorf("timeout acquiring thread lock for %s", threadID)
	case <-ctx.Done():
		log.Printf("[ThreadLock] Context cancelled while acquiring lock thread_id=%s", threadID)
		return ctx.Err()
	}
}

// ListRunsResponse represents the response from listing runs
type ListRunsResponse struct {
	Data []Run `json:"data"`
}

// ListRuns retrieves all runs for a thread
func (c *Client) ListRuns(threadID string) ([]Run, error) {
	log.Printf("[Assistant] ListRuns started thread_id=%s", threadID)

	req, err := c.newRequest("GET", baseURL+"/threads/"+threadID+"/runs", nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Assistant] ListRuns failed: send request err=%v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, c.handleError(resp)
	}

	var listResp ListRunsResponse
	if err := decodeJSON(resp.Body, &listResp); err != nil {
		return nil, err
	}

	log.Printf("[Assistant] ListRuns completed thread_id=%s count=%d", threadID, len(listResp.Data))
	return listResp.Data, nil
}

// HasActiveRun checks if there's an active run on the thread
func (c *Client) HasActiveRun(threadID string) (bool, *Run, error) {
	runs, err := c.ListRuns(threadID)
	if err != nil {
		return false, nil, err
	}

	for _, run := range runs {
		if run.Status == "queued" || run.Status == "in_progress" || run.Status == "requires_action" {
			log.Printf("[Assistant] Found active run thread_id=%s run_id=%s status=%s", threadID, run.ID, run.Status)
			return true, &run, nil
		}
	}

	return false, nil, nil
}

// WaitForActiveRunsToComplete waits for all active runs to complete
func (c *Client) WaitForActiveRunsToComplete(threadID string, timeout time.Duration) error {
	log.Printf("[Assistant] WaitForActiveRunsToComplete started thread_id=%s timeout=%v", threadID, timeout)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		hasActive, activeRun, err := c.HasActiveRun(threadID)
		if err != nil {
			return err
		}

		if !hasActive {
			log.Printf("[Assistant] WaitForActiveRunsToComplete: no active runs thread_id=%s", threadID)
			return nil
		}

		log.Printf("[Assistant] WaitForActiveRunsToComplete: waiting for run_id=%s status=%s", activeRun.ID, activeRun.Status)
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for active runs to complete on thread %s", threadID)
}

// newRequest is a helper to create HTTP requests
func (c *Client) newRequest(method, url string, body []byte) (*http.Request, error) {
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return req, nil
}

// decodeJSON is a helper to decode JSON responses
func decodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

