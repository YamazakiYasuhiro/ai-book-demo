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

// Conversation represents a conversation for testing
type Conversation struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	ThreadID  string `json:"thread_id"`
	CreatedAt string `json:"created_at"`
}

func TestConversationEndpoints(t *testing.T) {
	suite := utils.NewTestSuite()

	// Wait for server
	if err := suite.WaitForServer(10 * time.Second); err != nil {
		t.Fatalf("Server not ready: %v", err)
	}

	// First, create an avatar to use in conversation tests
	var testAvatarID int64

	t.Run("Setup: Create test avatar", func(t *testing.T) {
		payload := map[string]string{
			"name":   fmt.Sprintf("ConvTestAvatar_%d", time.Now().UnixNano()),
			"prompt": "A test assistant for conversation tests",
		}

		resp, err := suite.Client.POST("/api/avatars", payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			var avatar Avatar
			err = utils.ReadJSON(resp, &avatar)
			require.NoError(t, err)
			testAvatarID = avatar.ID
		}
	})

	var createdConversationID int64

	t.Run("List conversations returns list", func(t *testing.T) {
		resp, err := suite.Client.GET("/api/conversations")
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusOK(t, resp)
		utils.AssertJSONContentType(t, resp)

		// API returns array directly
		var result []Conversation
		err = utils.ReadJSON(resp, &result)
		require.NoError(t, err)

		assert.NotNil(t, result)
	})

	t.Run("Create conversation with valid data", func(t *testing.T) {
		if testAvatarID == 0 {
			t.Skip("No test avatar available")
		}

		payload := map[string]any{
			"title":      fmt.Sprintf("TestConversation_%d", time.Now().UnixNano()),
			"avatar_ids": []int64{testAvatarID},
		}

		resp, err := suite.Client.POST("/api/conversations", payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusCreated(t, resp)
		utils.AssertJSONContentType(t, resp)

		var conversation Conversation
		err = utils.ReadJSON(resp, &conversation)
		require.NoError(t, err)

		assert.NotZero(t, conversation.ID)
		assert.Contains(t, conversation.Title, "TestConversation")

		createdConversationID = conversation.ID
	})

	t.Run("Get conversation by ID", func(t *testing.T) {
		if createdConversationID == 0 {
			t.Skip("No conversation created in previous test")
		}

		resp, err := suite.Client.GET(fmt.Sprintf("/api/conversations/%d", createdConversationID))
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusOK(t, resp)
		utils.AssertJSONContentType(t, resp)

		var conversation Conversation
		err = utils.ReadJSON(resp, &conversation)
		require.NoError(t, err)

		assert.Equal(t, createdConversationID, conversation.ID)
	})

	t.Run("Get non-existent conversation returns 404", func(t *testing.T) {
		resp, err := suite.Client.GET("/api/conversations/999999")
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusNotFound(t, resp)
	})

	t.Run("Create conversation with missing title returns 400", func(t *testing.T) {
		payload := map[string]any{
			"avatar_ids": []int64{1},
		}

		resp, err := suite.Client.POST("/api/conversations", payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusBadRequest(t, resp)
	})

	t.Run("Delete conversation", func(t *testing.T) {
		if createdConversationID == 0 {
			t.Skip("No conversation created in previous test")
		}

		resp, err := suite.Client.DELETE(fmt.Sprintf("/api/conversations/%d", createdConversationID))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should be 200 OK or 204 No Content
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 200 or 204, got %d", resp.StatusCode)
		}
	})

	// Cleanup
	t.Run("Cleanup: Delete test avatar", func(t *testing.T) {
		if testAvatarID == 0 {
			return
		}

		resp, err := suite.Client.DELETE(fmt.Sprintf("/api/avatars/%d", testAvatarID))
		if err == nil {
			resp.Body.Close()
		}
	})
}
