package integration

import (
	"testing"
	"time"

	"github.com/ai-book-demo/tests/utils"
)

func TestHealthEndpoint(t *testing.T) {
	suite := utils.NewTestSuite()

	// Wait for server
	if err := suite.WaitForServer(10 * time.Second); err != nil {
		t.Fatalf("Server not ready: %v", err)
	}

	t.Run("Health check returns OK", func(t *testing.T) {
		resp, err := suite.Client.GET("/health")
		if err != nil {
			t.Fatalf("Failed to call health endpoint: %v", err)
		}
		defer resp.Body.Close()

		utils.AssertStatusOK(t, resp)
		utils.AssertJSONContentType(t, resp)

		var result map[string]string
		if err := utils.ReadJSON(resp, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result["status"] != "ok" {
			t.Errorf("Expected status 'ok', got '%s'", result["status"])
		}
	})
}

