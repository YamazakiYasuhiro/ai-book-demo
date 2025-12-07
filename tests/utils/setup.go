package utils

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	// DefaultBaseURL is the default server URL for tests
	DefaultBaseURL = "http://localhost:8080"
	// DefaultTimeout is the default timeout for waiting server
	DefaultTimeout = 30 * time.Second
)

// TestSuite provides setup/teardown for integration tests
type TestSuite struct {
	Client *TestClient
}

// NewTestSuite creates a new test suite
func NewTestSuite() *TestSuite {
	baseURL := os.Getenv("TEST_BASE_URL")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	return &TestSuite{
		Client: NewTestClient(baseURL),
	}
}

// WaitForServer waits until the server is healthy
func (s *TestSuite) WaitForServer(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(s.Client.BaseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("server did not become healthy within %v", timeout)
}

// MustWaitForServer waits for server and panics if it fails
func (s *TestSuite) MustWaitForServer() {
	if err := s.WaitForServer(DefaultTimeout); err != nil {
		panic(fmt.Sprintf("Failed to connect to server: %v", err))
	}
}

