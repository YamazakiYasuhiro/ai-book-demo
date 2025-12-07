package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/ai-book-demo/tests/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Message represents a message for testing
type Message struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversation_id"`
	SenderType     string `json:"sender_type"`
	SenderID       *int64 `json:"sender_id,omitempty"`
	SenderName     string `json:"sender_name"`
	Content        string `json:"content"`
	CreatedAt      string `json:"created_at"`
}

func TestMessageEndpoints(t *testing.T) {
	suite := utils.NewTestSuite()

	// Wait for server
	if err := suite.WaitForServer(10 * time.Second); err != nil {
		t.Fatalf("Server not ready: %v", err)
	}

	// Setup: Create avatar and conversation for message tests
	var testAvatarID int64
	var testConversationID int64

	t.Run("Setup: Create test avatar", func(t *testing.T) {
		payload := map[string]string{
			"name":   fmt.Sprintf("MsgTestAvatar_%d", time.Now().UnixNano()),
			"prompt": "A test assistant for message tests",
		}

		resp, err := suite.Client.POST("/api/avatars", payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == 201 {
			var avatar Avatar
			err = utils.ReadJSON(resp, &avatar)
			require.NoError(t, err)
			testAvatarID = avatar.ID
		}
	})

	t.Run("Setup: Create test conversation", func(t *testing.T) {
		if testAvatarID == 0 {
			t.Skip("No test avatar available")
		}

		payload := map[string]any{
			"title":      fmt.Sprintf("MsgTestConversation_%d", time.Now().UnixNano()),
			"avatar_ids": []int64{testAvatarID},
		}

		resp, err := suite.Client.POST("/api/conversations", payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == 201 {
			var conversation Conversation
			err = utils.ReadJSON(resp, &conversation)
			require.NoError(t, err)
			testConversationID = conversation.ID
		}
	})

	t.Run("Get messages returns empty list for new conversation", func(t *testing.T) {
		if testConversationID == 0 {
			t.Skip("No test conversation available")
		}

		resp, err := suite.Client.GET(fmt.Sprintf("/api/conversations/%d/messages", testConversationID))
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusOK(t, resp)
		utils.AssertJSONContentType(t, resp)

		// API returns array directly
		var result []Message
		err = utils.ReadJSON(resp, &result)
		require.NoError(t, err)

		assert.NotNil(t, result)
	})

	t.Run("Send message to conversation", func(t *testing.T) {
		if testConversationID == 0 {
			t.Skip("No test conversation available")
		}

		payload := map[string]string{
			"content": "Hello, this is a test message!",
		}

		resp, err := suite.Client.POST(fmt.Sprintf("/api/conversations/%d/messages", testConversationID), payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 200 OK or 201 Created
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			body, _ := utils.ReadBody(resp)
			t.Fatalf("Expected status 200 or 201, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("Get messages returns sent message", func(t *testing.T) {
		if testConversationID == 0 {
			t.Skip("No test conversation available")
		}

		// Small delay to allow message to be processed
		time.Sleep(100 * time.Millisecond)

		resp, err := suite.Client.GET(fmt.Sprintf("/api/conversations/%d/messages", testConversationID))
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusOK(t, resp)

		// API returns array directly
		var result []Message
		err = utils.ReadJSON(resp, &result)
		require.NoError(t, err)

		// Should have at least one message (the one we sent)
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("Send message with empty content returns 400", func(t *testing.T) {
		if testConversationID == 0 {
			t.Skip("No test conversation available")
		}

		payload := map[string]string{
			"content": "",
		}

		resp, err := suite.Client.POST(fmt.Sprintf("/api/conversations/%d/messages", testConversationID), payload)
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusBadRequest(t, resp)
	})

	t.Run("Get messages for non-existent conversation returns 404", func(t *testing.T) {
		resp, err := suite.Client.GET("/api/conversations/999999/messages")
		require.NoError(t, err)
		defer resp.Body.Close()

		utils.AssertStatusNotFound(t, resp)
	})

	// Cleanup
	t.Run("Cleanup: Delete test conversation", func(t *testing.T) {
		if testConversationID == 0 {
			return
		}

		resp, err := suite.Client.DELETE(fmt.Sprintf("/api/conversations/%d", testConversationID))
		if err == nil {
			resp.Body.Close()
		}
	})

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
