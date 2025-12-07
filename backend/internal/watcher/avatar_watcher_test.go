package watcher

import (
	"context"
	"testing"
	"time"

	"multi-avatar-chat/internal/models"
)

func TestNewAvatarWatcher(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	avatar := models.Avatar{
		ID:     1,
		Name:   "TestBot",
		Prompt: "Helpful assistant",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, 1, avatar, database, nil, 100*time.Millisecond, nil)

	if watcher == nil {
		t.Fatal("expected non-nil watcher")
	}

	if watcher.conversationID != 1 {
		t.Errorf("expected conversationID 1, got %d", watcher.conversationID)
	}

	if watcher.avatar.Name != "TestBot" {
		t.Errorf("expected avatar name 'TestBot', got '%s'", watcher.avatar.Name)
	}
}

func TestAvatarWatcher_StartStop(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// Create conversation
	conv, err := database.CreateConversation("Test Chat", "thread_123")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	avatar := models.Avatar{
		ID:     1,
		Name:   "TestBot",
		Prompt: "Helpful assistant",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, conv.ID, avatar, database, nil, 50*time.Millisecond, nil)

	// Start watcher
	watcher.Start()

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	// Stop watcher (should complete without hanging)
	done := make(chan struct{})
	go func() {
		watcher.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() timed out - possible goroutine leak")
	}
}

func TestAvatarWatcher_ShouldRespond_Mention(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	avatar := models.Avatar{
		ID:     1,
		Name:   "太郎",
		Prompt: "Helpful assistant",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, 1, avatar, database, nil, 100*time.Millisecond, nil)

	// Test with mention
	message := &models.Message{
		ID:         1,
		Content:    "@太郎 質問があります",
		SenderType: models.SenderTypeUser,
	}

	shouldRespond, err := watcher.shouldRespond(message)
	if err != nil {
		t.Fatalf("shouldRespond failed: %v", err)
	}

	if !shouldRespond {
		t.Error("expected shouldRespond to return true for mentioned avatar")
	}
}

func TestAvatarWatcher_ShouldRespond_NoMention(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	avatar := models.Avatar{
		ID:     1,
		Name:   "太郎",
		Prompt: "Helpful assistant",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, 1, avatar, database, nil, 100*time.Millisecond, nil)

	// Test without mention (and no assistant for LLM judgment)
	message := &models.Message{
		ID:         1,
		Content:    "こんにちは",
		SenderType: models.SenderTypeUser,
	}

	shouldRespond, err := watcher.shouldRespond(message)
	if err != nil {
		t.Fatalf("shouldRespond failed: %v", err)
	}

	// Without assistant, should return false for no mention
	if shouldRespond {
		t.Error("expected shouldRespond to return false without mention and without assistant")
	}
}

func TestAvatarWatcher_ShouldRespond_CaseInsensitive(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	avatar := models.Avatar{
		ID:     1,
		Name:   "Alice",
		Prompt: "Helpful assistant",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, 1, avatar, database, nil, 100*time.Millisecond, nil)

	// Test with different case mention
	message := &models.Message{
		ID:         1,
		Content:    "@alice please help",
		SenderType: models.SenderTypeUser,
	}

	shouldRespond, err := watcher.shouldRespond(message)
	if err != nil {
		t.Fatalf("shouldRespond failed: %v", err)
	}

	if !shouldRespond {
		t.Error("expected shouldRespond to return true for case-insensitive mention")
	}
}

func TestAvatarWatcher_CheckAndRespond_SkipsOwnMessages(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// Create conversation
	conv, _ := database.CreateConversation("Test Chat", "thread_123")

	avatar := models.Avatar{
		ID:     1,
		Name:   "TestBot",
		Prompt: "Helpful assistant",
	}

	// Create a message from the avatar itself
	avatarID := avatar.ID
	database.CreateMessage(conv.ID, models.SenderTypeAvatar, &avatarID, "@TestBot test")

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, conv.ID, avatar, database, nil, 100*time.Millisecond, nil)

	// Initialize and check
	watcher.initializeLastMessageID()
	initialLastID := watcher.GetLastMessageID()

	// Create another message from the same avatar (mentioning itself)
	database.CreateMessage(conv.ID, models.SenderTypeAvatar, &avatarID, "@TestBot another test")

	// Run check - should skip own message even if mentioned
	err := watcher.checkAndRespond()
	if err != nil {
		t.Fatalf("checkAndRespond failed: %v", err)
	}

	// lastMessageID should be updated (message was processed)
	if watcher.GetLastMessageID() <= initialLastID {
		t.Error("expected lastMessageID to be updated")
	}
}

func TestAvatarWatcher_BuildJudgmentPrompt(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	avatar := models.Avatar{
		ID:     1,
		Name:   "助手さん",
		Prompt: "親切で丁寧なアシスタント",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, 1, avatar, database, nil, 100*time.Millisecond, nil)

	prompt := watcher.buildJudgmentPrompt("質問があります")

	// Check that prompt contains avatar info
	if !contains(prompt, "助手さん") {
		t.Error("prompt should contain avatar name")
	}

	if !contains(prompt, "親切で丁寧なアシスタント") {
		t.Error("prompt should contain avatar prompt/personality")
	}

	if !contains(prompt, "質問があります") {
		t.Error("prompt should contain message content")
	}

	if !contains(prompt, "yes") && !contains(prompt, "no") {
		t.Error("prompt should mention yes/no answer format")
	}
}

func TestAvatarWatcher_BuildJudgmentPrompt_WithContext(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	avatar := models.Avatar{
		ID:     1,
		Name:   "助手さん",
		Prompt: "親切で丁寧なアシスタント",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, 1, avatar, database, nil, 100*time.Millisecond, nil)

	// Set context information
	watcher.SetConversationContext("AIについての議論", []string{"ユーザ", "助手さん", "博士"})

	prompt := watcher.buildJudgmentPrompt("質問があります")

	// Check that prompt contains context info
	if !contains(prompt, "AIについての議論") {
		t.Error("prompt should contain conversation title (topic)")
	}

	if !contains(prompt, "ユーザ") {
		t.Error("prompt should contain user in participants")
	}

	if !contains(prompt, "博士") {
		t.Error("prompt should contain other avatar in participants")
	}

	// Avatar's own name should also be in prompt
	if !contains(prompt, "助手さん") {
		t.Error("prompt should contain avatar name")
	}
}

func TestAvatarWatcher_SetConversationContext(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	avatar := models.Avatar{
		ID:   1,
		Name: "TestBot",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, 1, avatar, database, nil, 100*time.Millisecond, nil)

	// Set context
	watcher.SetConversationContext("Test Topic", []string{"ユーザ", "TestBot", "OtherBot"})

	if watcher.conversationTitle != "Test Topic" {
		t.Errorf("expected conversationTitle 'Test Topic', got '%s'", watcher.conversationTitle)
	}

	if len(watcher.participantNames) != 3 {
		t.Errorf("expected 3 participants, got %d", len(watcher.participantNames))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAvatarWatcher_InitializeLastMessageID(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	conv, _ := database.CreateConversation("Test Chat", "thread_123")

	// Create some messages
	database.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Message 1")
	msg2, _ := database.CreateMessage(conv.ID, models.SenderTypeUser, nil, "Message 2")

	avatar := models.Avatar{
		ID:     1,
		Name:   "TestBot",
		Prompt: "Helpful assistant",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, conv.ID, avatar, database, nil, 100*time.Millisecond, nil)

	err := watcher.initializeLastMessageID()
	if err != nil {
		t.Fatalf("initializeLastMessageID failed: %v", err)
	}

	if watcher.GetLastMessageID() != msg2.ID {
		t.Errorf("expected lastMessageID to be %d, got %d", msg2.ID, watcher.GetLastMessageID())
	}
}

func TestAvatarWatcher_InitializeLastMessageID_Empty(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	conv, _ := database.CreateConversation("Test Chat", "thread_123")

	avatar := models.Avatar{
		ID:     1,
		Name:   "TestBot",
		Prompt: "Helpful assistant",
	}

	ctx := context.Background()
	watcher := NewAvatarWatcher(ctx, conv.ID, avatar, database, nil, 100*time.Millisecond, nil)

	err := watcher.initializeLastMessageID()
	if err != nil {
		t.Fatalf("initializeLastMessageID failed: %v", err)
	}

	if watcher.GetLastMessageID() != 0 {
		t.Errorf("expected lastMessageID to be 0 for empty conversation, got %d", watcher.GetLastMessageID())
	}
}

func TestGetRandomInterval(t *testing.T) {
	// Test that random interval is within range [5s, 20s]
	minInterval := 5 * time.Second
	maxInterval := 20 * time.Second

	// Run multiple times to test randomness
	for i := range 100 {
		interval := getRandomInterval()

		if interval < minInterval {
			t.Errorf("iteration %d: interval %v is less than minimum %v", i, interval, minInterval)
		}

		if interval > maxInterval {
			t.Errorf("iteration %d: interval %v is greater than maximum %v", i, interval, maxInterval)
		}
	}
}

func TestGetRandomInterval_Variance(t *testing.T) {
	// Test that we get some variance in the intervals
	intervals := make(map[time.Duration]int)

	for range 50 {
		interval := getRandomInterval()
		// Round to nearest second for grouping
		rounded := interval.Round(time.Second)
		intervals[rounded]++
	}

	// Should have at least 3 different intervals over 50 iterations
	if len(intervals) < 3 {
		t.Errorf("expected at least 3 different interval values, got %d", len(intervals))
	}
}

