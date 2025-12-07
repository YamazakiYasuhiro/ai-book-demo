package db

import (
	"database/sql"
	"os"
	"testing"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	tmpFile := createTempDB(t)
	database, err := NewDB(tmpFile)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	cleanup := func() {
		database.Close()
		os.Remove(tmpFile)
	}

	return database, cleanup
}

func TestCreateAvatar(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	avatar, err := db.CreateAvatar("TestBot", "You are helpful", "asst_123")
	if err != nil {
		t.Fatalf("failed to create avatar: %v", err)
	}

	if avatar.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if avatar.Name != "TestBot" {
		t.Errorf("expected name 'TestBot', got '%s'", avatar.Name)
	}
	if avatar.Prompt != "You are helpful" {
		t.Errorf("expected prompt 'You are helpful', got '%s'", avatar.Prompt)
	}
	if avatar.OpenAIAssistantID != "asst_123" {
		t.Errorf("expected assistant_id 'asst_123', got '%s'", avatar.OpenAIAssistantID)
	}
}

func TestGetAvatar(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	created, err := db.CreateAvatar("GetTest", "Test prompt", "asst_456")
	if err != nil {
		t.Fatalf("failed to create avatar: %v", err)
	}

	avatar, err := db.GetAvatar(created.ID)
	if err != nil {
		t.Fatalf("failed to get avatar: %v", err)
	}

	if avatar.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, avatar.ID)
	}
	if avatar.Name != "GetTest" {
		t.Errorf("expected name 'GetTest', got '%s'", avatar.Name)
	}
}

func TestGetAvatar_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.GetAvatar(99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestGetAllAvatars(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create multiple avatars
	_, err := db.CreateAvatar("Avatar1", "Prompt 1", "asst_1")
	if err != nil {
		t.Fatalf("failed to create avatar 1: %v", err)
	}
	_, err = db.CreateAvatar("Avatar2", "Prompt 2", "asst_2")
	if err != nil {
		t.Fatalf("failed to create avatar 2: %v", err)
	}

	avatars, err := db.GetAllAvatars()
	if err != nil {
		t.Fatalf("failed to get all avatars: %v", err)
	}

	if len(avatars) != 2 {
		t.Errorf("expected 2 avatars, got %d", len(avatars))
	}
}

func TestUpdateAvatar(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	created, err := db.CreateAvatar("Original", "Original prompt", "asst_orig")
	if err != nil {
		t.Fatalf("failed to create avatar: %v", err)
	}

	updated, err := db.UpdateAvatar(created.ID, "Updated", "Updated prompt", "asst_updated")
	if err != nil {
		t.Fatalf("failed to update avatar: %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got '%s'", updated.Name)
	}
	if updated.Prompt != "Updated prompt" {
		t.Errorf("expected prompt 'Updated prompt', got '%s'", updated.Prompt)
	}
	if updated.OpenAIAssistantID != "asst_updated" {
		t.Errorf("expected assistant_id 'asst_updated', got '%s'", updated.OpenAIAssistantID)
	}
}

func TestDeleteAvatar(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	created, err := db.CreateAvatar("ToDelete", "Delete me", "asst_del")
	if err != nil {
		t.Fatalf("failed to create avatar: %v", err)
	}

	err = db.DeleteAvatar(created.ID)
	if err != nil {
		t.Fatalf("failed to delete avatar: %v", err)
	}

	// Verify deletion
	_, err = db.GetAvatar(created.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after deletion, got %v", err)
	}
}

func TestDeleteAvatar_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.DeleteAvatar(99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

