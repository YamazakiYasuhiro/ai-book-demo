package db

import (
	"database/sql"
	"testing"

	"multi-avatar-chat/internal/models"
)

func TestCreateConversation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("Test Chat", "thread_123")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	if conv.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if conv.Title != "Test Chat" {
		t.Errorf("expected title 'Test Chat', got '%s'", conv.Title)
	}
	if conv.ThreadID != "thread_123" {
		t.Errorf("expected thread_id 'thread_123', got '%s'", conv.ThreadID)
	}
}

func TestGetConversation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	created, err := db.CreateConversation("Get Test", "thread_456")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	conv, err := db.GetConversation(created.ID)
	if err != nil {
		t.Fatalf("failed to get conversation: %v", err)
	}

	if conv.Title != "Get Test" {
		t.Errorf("expected title 'Get Test', got '%s'", conv.Title)
	}
}

func TestGetConversation_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.GetConversation(99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestGetAllConversations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.CreateConversation("Conv1", "thread_1")
	if err != nil {
		t.Fatalf("failed to create conversation 1: %v", err)
	}
	_, err = db.CreateConversation("Conv2", "thread_2")
	if err != nil {
		t.Fatalf("failed to create conversation 2: %v", err)
	}

	conversations, err := db.GetAllConversations()
	if err != nil {
		t.Fatalf("failed to get all conversations: %v", err)
	}

	if len(conversations) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(conversations))
	}
}

func TestDeleteConversation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("ToDelete", "thread_del")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	err = db.DeleteConversation(conv.ID)
	if err != nil {
		t.Fatalf("failed to delete conversation: %v", err)
	}

	_, err = db.GetConversation(conv.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after deletion, got %v", err)
	}
}

func TestConversationAvatars(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create conversation and avatars
	conv, err := db.CreateConversation("Chat with Avatars", "thread_chat")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	avatar1, err := db.CreateAvatar("Avatar1", "Prompt1", "asst_1")
	if err != nil {
		t.Fatalf("failed to create avatar 1: %v", err)
	}

	avatar2, err := db.CreateAvatar("Avatar2", "Prompt2", "asst_2")
	if err != nil {
		t.Fatalf("failed to create avatar 2: %v", err)
	}

	// Add avatars to conversation
	if err := db.AddAvatarToConversation(conv.ID, avatar1.ID); err != nil {
		t.Fatalf("failed to add avatar 1: %v", err)
	}
	if err := db.AddAvatarToConversation(conv.ID, avatar2.ID); err != nil {
		t.Fatalf("failed to add avatar 2: %v", err)
	}

	// Get avatars
	avatars, err := db.GetConversationAvatars(conv.ID)
	if err != nil {
		t.Fatalf("failed to get conversation avatars: %v", err)
	}

	if len(avatars) != 2 {
		t.Errorf("expected 2 avatars, got %d", len(avatars))
	}
}

func TestCreateMessage(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("Message Test", "thread_msg")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	msg, err := db.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Hello, world!")
	if err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	if msg.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got '%s'", msg.Content)
	}
	if msg.SenderType != models.SenderTypeUser {
		t.Errorf("expected sender_type 'user', got '%s'", msg.SenderType)
	}
}

func TestCreateMessage_WithSenderID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("Avatar Message Test", "thread_avmsg")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	avatar, err := db.CreateAvatar("MsgBot", "Helpful", "asst_msg")
	if err != nil {
		t.Fatalf("failed to create avatar: %v", err)
	}

	senderID := avatar.ID
	msg, err := db.CreateMessage(conv.ID, models.SenderTypeAvatar, &senderID, "Bot response")
	if err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	if msg.SenderID == nil || *msg.SenderID != avatar.ID {
		t.Errorf("expected sender_id %d, got %v", avatar.ID, msg.SenderID)
	}
}

func TestGetMessages(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("Get Messages Test", "thread_getmsg")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	_, err = db.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Message 1")
	if err != nil {
		t.Fatalf("failed to create message 1: %v", err)
	}
	_, err = db.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Message 2")
	if err != nil {
		t.Fatalf("failed to create message 2: %v", err)
	}

	messages, err := db.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("failed to get messages: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestDeleteConversation_CascadesMessages(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("Cascade Test", "thread_cascade")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	_, err = db.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Test message")
	if err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	err = db.DeleteConversation(conv.ID)
	if err != nil {
		t.Fatalf("failed to delete conversation: %v", err)
	}

	messages, err := db.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("failed to get messages: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected 0 messages after cascade delete, got %d", len(messages))
	}
}

func TestRemoveAvatarFromConversation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("Remove Avatar Test", "thread_remove")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	avatar, err := db.CreateAvatar("RemoveBot", "Prompt", "asst_remove")
	if err != nil {
		t.Fatalf("failed to create avatar: %v", err)
	}

	// Add avatar
	if err := db.AddAvatarToConversation(conv.ID, avatar.ID); err != nil {
		t.Fatalf("failed to add avatar: %v", err)
	}

	// Verify avatar is added
	avatars, err := db.GetConversationAvatars(conv.ID)
	if err != nil {
		t.Fatalf("failed to get avatars: %v", err)
	}
	if len(avatars) != 1 {
		t.Errorf("expected 1 avatar, got %d", len(avatars))
	}

	// Remove avatar
	err = db.RemoveAvatarFromConversation(conv.ID, avatar.ID)
	if err != nil {
		t.Fatalf("failed to remove avatar: %v", err)
	}

	// Verify avatar is removed
	avatars, err = db.GetConversationAvatars(conv.ID)
	if err != nil {
		t.Fatalf("failed to get avatars after removal: %v", err)
	}
	if len(avatars) != 0 {
		t.Errorf("expected 0 avatars after removal, got %d", len(avatars))
	}
}

func TestRemoveAvatarFromConversation_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("Remove Not Found Test", "thread_notfound")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	err = db.RemoveAvatarFromConversation(conv.ID, 99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestGetMessagesAfter(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("Messages After Test", "thread_after")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	// Create 3 messages
	msg1, err := db.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Message 1")
	if err != nil {
		t.Fatalf("failed to create message 1: %v", err)
	}
	msg2, err := db.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Message 2")
	if err != nil {
		t.Fatalf("failed to create message 2: %v", err)
	}
	_, err = db.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Message 3")
	if err != nil {
		t.Fatalf("failed to create message 3: %v", err)
	}

	// Get messages after msg1
	messages, err := db.GetMessagesAfter(conv.ID, msg1.ID)
	if err != nil {
		t.Fatalf("failed to get messages after: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	// Get messages after msg2
	messages, err = db.GetMessagesAfter(conv.ID, msg2.ID)
	if err != nil {
		t.Fatalf("failed to get messages after msg2: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
}

func TestGetMessagesAfter_NoMessages(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := db.CreateConversation("No Messages After Test", "thread_nomsg")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	msg, err := db.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Only message")
	if err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	messages, err := db.GetMessagesAfter(conv.ID, msg.ID)
	if err != nil {
		t.Fatalf("failed to get messages: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestGetAllConversationAvatars(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create conversations
	conv1, err := db.CreateConversation("Conv1", "thread_1")
	if err != nil {
		t.Fatalf("failed to create conversation 1: %v", err)
	}
	conv2, err := db.CreateConversation("Conv2", "thread_2")
	if err != nil {
		t.Fatalf("failed to create conversation 2: %v", err)
	}

	// Create avatars
	avatar1, err := db.CreateAvatar("Avatar1", "Prompt1", "asst_1")
	if err != nil {
		t.Fatalf("failed to create avatar 1: %v", err)
	}
	avatar2, err := db.CreateAvatar("Avatar2", "Prompt2", "asst_2")
	if err != nil {
		t.Fatalf("failed to create avatar 2: %v", err)
	}

	// Add avatars to conversations
	db.AddAvatarToConversation(conv1.ID, avatar1.ID)
	db.AddAvatarToConversation(conv1.ID, avatar2.ID)
	db.AddAvatarToConversation(conv2.ID, avatar1.ID)

	// Get all pairs
	pairs, err := db.GetAllConversationAvatars()
	if err != nil {
		t.Fatalf("failed to get all conversation avatars: %v", err)
	}

	if len(pairs) != 3 {
		t.Errorf("expected 3 pairs, got %d", len(pairs))
	}
}

func TestGetAllConversationAvatars_Empty(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	pairs, err := db.GetAllConversationAvatars()
	if err != nil {
		t.Fatalf("failed to get all conversation avatars: %v", err)
	}

	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs, got %d", len(pairs))
	}
}

