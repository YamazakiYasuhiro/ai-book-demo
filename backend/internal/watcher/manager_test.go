package watcher

import (
	"context"
	"os"
	"testing"
	"time"

	"multi-avatar-chat/internal/db"
)

func setupTestDB(t *testing.T) (*db.DB, func()) {
	t.Helper()

	// Create a temporary file for the test database
	tmpFile, err := os.CreateTemp("", "test_watcher_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	database, err := db.NewDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to open database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		database.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to migrate database: %v", err)
	}

	cleanup := func() {
		database.Close()
		os.Remove(tmpFile.Name())
	}

	return database, cleanup
}

func TestNewManager(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(database, nil, 10*time.Second)

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}

	if manager.WatcherCount() != 0 {
		t.Errorf("expected 0 watchers, got %d", manager.WatcherCount())
	}
}

func TestManager_StartWatcher(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a conversation and avatar
	conv, err := database.CreateConversation("Test Chat", "thread_123")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	avatar, err := database.CreateAvatar("TestBot", "Helpful assistant", "asst_123")
	if err != nil {
		t.Fatalf("failed to create avatar: %v", err)
	}

	manager := NewManager(database, nil, 100*time.Millisecond)
	defer manager.Shutdown()

	// Start watcher
	err = manager.StartWatcher(conv.ID, avatar.ID)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	if manager.WatcherCount() != 1 {
		t.Errorf("expected 1 watcher, got %d", manager.WatcherCount())
	}

	if !manager.HasWatcher(conv.ID, avatar.ID) {
		t.Error("expected HasWatcher to return true")
	}
}

func TestManager_StartWatcher_Duplicate(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	conv, _ := database.CreateConversation("Test Chat", "thread_123")
	avatar, _ := database.CreateAvatar("TestBot", "Helpful assistant", "asst_123")

	manager := NewManager(database, nil, 100*time.Millisecond)
	defer manager.Shutdown()

	// Start watcher twice
	manager.StartWatcher(conv.ID, avatar.ID)
	manager.StartWatcher(conv.ID, avatar.ID) // Should not create duplicate

	if manager.WatcherCount() != 1 {
		t.Errorf("expected 1 watcher (no duplicate), got %d", manager.WatcherCount())
	}
}

func TestManager_StopWatcher(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	conv, _ := database.CreateConversation("Test Chat", "thread_123")
	avatar, _ := database.CreateAvatar("TestBot", "Helpful assistant", "asst_123")

	manager := NewManager(database, nil, 100*time.Millisecond)
	defer manager.Shutdown()

	manager.StartWatcher(conv.ID, avatar.ID)

	// Stop watcher
	err := manager.StopWatcher(conv.ID, avatar.ID)
	if err != nil {
		t.Fatalf("failed to stop watcher: %v", err)
	}

	if manager.WatcherCount() != 0 {
		t.Errorf("expected 0 watchers, got %d", manager.WatcherCount())
	}

	if manager.HasWatcher(conv.ID, avatar.ID) {
		t.Error("expected HasWatcher to return false after stop")
	}
}

func TestManager_StopWatcher_NotFound(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(database, nil, 100*time.Millisecond)
	defer manager.Shutdown()

	// Stop non-existent watcher - should not error
	err := manager.StopWatcher(99999, 99999)
	if err != nil {
		t.Fatalf("expected no error for non-existent watcher, got %v", err)
	}
}

func TestManager_StopRoomWatchers(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	conv, _ := database.CreateConversation("Test Chat", "thread_123")
	avatar1, _ := database.CreateAvatar("Bot1", "Prompt1", "asst_1")
	avatar2, _ := database.CreateAvatar("Bot2", "Prompt2", "asst_2")

	manager := NewManager(database, nil, 100*time.Millisecond)
	defer manager.Shutdown()

	manager.StartWatcher(conv.ID, avatar1.ID)
	manager.StartWatcher(conv.ID, avatar2.ID)

	if manager.WatcherCount() != 2 {
		t.Fatalf("expected 2 watchers, got %d", manager.WatcherCount())
	}

	// Stop all watchers for the room
	err := manager.StopRoomWatchers(conv.ID)
	if err != nil {
		t.Fatalf("failed to stop room watchers: %v", err)
	}

	if manager.WatcherCount() != 0 {
		t.Errorf("expected 0 watchers after StopRoomWatchers, got %d", manager.WatcherCount())
	}
}

func TestManager_InitializeAll(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// Create conversations and avatars
	conv1, _ := database.CreateConversation("Conv1", "thread_1")
	conv2, _ := database.CreateConversation("Conv2", "thread_2")
	avatar1, _ := database.CreateAvatar("Bot1", "Prompt1", "asst_1")
	avatar2, _ := database.CreateAvatar("Bot2", "Prompt2", "asst_2")

	// Add avatars to conversations
	database.AddAvatarToConversation(conv1.ID, avatar1.ID)
	database.AddAvatarToConversation(conv1.ID, avatar2.ID)
	database.AddAvatarToConversation(conv2.ID, avatar1.ID)

	manager := NewManager(database, nil, 100*time.Millisecond)
	defer manager.Shutdown()

	ctx := context.Background()
	err := manager.InitializeAll(ctx)
	if err != nil {
		t.Fatalf("failed to initialize all: %v", err)
	}

	if manager.WatcherCount() != 3 {
		t.Errorf("expected 3 watchers, got %d", manager.WatcherCount())
	}

	// Verify specific watchers exist
	if !manager.HasWatcher(conv1.ID, avatar1.ID) {
		t.Error("expected watcher for conv1/avatar1")
	}
	if !manager.HasWatcher(conv1.ID, avatar2.ID) {
		t.Error("expected watcher for conv1/avatar2")
	}
	if !manager.HasWatcher(conv2.ID, avatar1.ID) {
		t.Error("expected watcher for conv2/avatar1")
	}
}

func TestManager_Shutdown(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	conv, _ := database.CreateConversation("Test Chat", "thread_123")
	avatar1, _ := database.CreateAvatar("Bot1", "Prompt1", "asst_1")
	avatar2, _ := database.CreateAvatar("Bot2", "Prompt2", "asst_2")

	manager := NewManager(database, nil, 100*time.Millisecond)

	manager.StartWatcher(conv.ID, avatar1.ID)
	manager.StartWatcher(conv.ID, avatar2.ID)

	if manager.WatcherCount() != 2 {
		t.Fatalf("expected 2 watchers, got %d", manager.WatcherCount())
	}

	err := manager.Shutdown()
	if err != nil {
		t.Fatalf("failed to shutdown: %v", err)
	}

	if manager.WatcherCount() != 0 {
		t.Errorf("expected 0 watchers after shutdown, got %d", manager.WatcherCount())
	}
}

func TestManager_MultipleRooms(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	conv1, _ := database.CreateConversation("Conv1", "thread_1")
	conv2, _ := database.CreateConversation("Conv2", "thread_2")
	avatar, _ := database.CreateAvatar("Bot", "Prompt", "asst_1")

	manager := NewManager(database, nil, 100*time.Millisecond)
	defer manager.Shutdown()

	// Same avatar in multiple rooms
	manager.StartWatcher(conv1.ID, avatar.ID)
	manager.StartWatcher(conv2.ID, avatar.ID)

	if manager.WatcherCount() != 2 {
		t.Errorf("expected 2 watchers (same avatar, different rooms), got %d", manager.WatcherCount())
	}

	// Stop one room's watchers
	manager.StopRoomWatchers(conv1.ID)

	if manager.WatcherCount() != 1 {
		t.Errorf("expected 1 watcher after stopping conv1, got %d", manager.WatcherCount())
	}

	if manager.HasWatcher(conv1.ID, avatar.ID) {
		t.Error("expected no watcher for conv1")
	}
	if !manager.HasWatcher(conv2.ID, avatar.ID) {
		t.Error("expected watcher for conv2 to still exist")
	}
}

