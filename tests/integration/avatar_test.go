package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ai-book-demo/tests/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Avatar represents an avatar for testing
type Avatar struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Prompt            string `json:"prompt"`
	OpenAIAssistantID string `json:"openai_assistant_id"`
	CreatedAt         string `json:"created_at"`
}

func TestAvatarEndpoints(t *testing.T) {
	suite := utils.NewTestSuite()

	// Wait for server
	if err := suite.WaitForServer(10 * time.Second); err != nil {
		t.Fatalf("Server not ready: %v", err)
	}

	var createdAvatarID int64

	t.Run("List avatars returns empty or existing list", func(t *testing.T) {
		resp, err := suite.Client.GET("/api/avatars")
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusOK(t, resp)
		utils.AssertJSONContentType(t, resp)

		// API returns array directly
		var result []Avatar
		err = utils.ReadJSON(resp, &result)
		require.NoError(t, err)

		// Should return a list (possibly empty)
		assert.NotNil(t, result)
	})

	t.Run("Create avatar with valid data", func(t *testing.T) {
		payload := map[string]string{
			"name":   fmt.Sprintf("TestAvatar_%d", time.Now().UnixNano()),
			"prompt": "A helpful test assistant",
		}

		resp, err := suite.Client.POST("/api/avatars", payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusCreated(t, resp)
		utils.AssertJSONContentType(t, resp)

		var avatar Avatar
		err = utils.ReadJSON(resp, &avatar)
		require.NoError(t, err)

		assert.NotZero(t, avatar.ID)
		assert.Equal(t, payload["name"], avatar.Name)
		assert.Equal(t, payload["prompt"], avatar.Prompt)

		createdAvatarID = avatar.ID
	})

	t.Run("Get avatar by ID", func(t *testing.T) {
		if createdAvatarID == 0 {
			t.Skip("No avatar created in previous test")
		}

		resp, err := suite.Client.GET(fmt.Sprintf("/api/avatars/%d", createdAvatarID))
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusOK(t, resp)
		utils.AssertJSONContentType(t, resp)

		var avatar Avatar
		err = utils.ReadJSON(resp, &avatar)
		require.NoError(t, err)

		assert.Equal(t, createdAvatarID, avatar.ID)
	})

	t.Run("Get non-existent avatar returns 404", func(t *testing.T) {
		resp, err := suite.Client.GET("/api/avatars/999999")
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusNotFound(t, resp)
	})

	t.Run("Create avatar with missing name returns 400", func(t *testing.T) {
		payload := map[string]string{
			"prompt": "A test prompt",
		}

		resp, err := suite.Client.POST("/api/avatars", payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusBadRequest(t, resp)
	})

	t.Run("Update avatar", func(t *testing.T) {
		if createdAvatarID == 0 {
			t.Skip("No avatar created in previous test")
		}

		payload := map[string]string{
			"name":   fmt.Sprintf("UpdatedAvatar_%d", time.Now().UnixNano()),
			"prompt": "An updated prompt",
		}

		resp, err := suite.Client.PUT(fmt.Sprintf("/api/avatars/%d", createdAvatarID), payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusOK(t, resp)
	})

	t.Run("Delete avatar", func(t *testing.T) {
		if createdAvatarID == 0 {
			t.Skip("No avatar created in previous test")
		}

		resp, err := suite.Client.DELETE(fmt.Sprintf("/api/avatars/%d", createdAvatarID))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should be 200 OK or 204 No Content
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 200 or 204, got %d", resp.StatusCode)
		}
	})
}
