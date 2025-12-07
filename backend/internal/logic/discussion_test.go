package logic

import (
	"testing"

	"multi-avatar-chat/internal/models"
)

func TestDiscussionMode_InitialState(t *testing.T) {
	config := DefaultDiscussionConfig()
	dm := NewDiscussionMode(config)

	if dm.IsRunning() {
		t.Error("expected discussion to not be running initially")
	}
	if dm.GetResponseCount() != 0 {
		t.Errorf("expected 0 responses initially, got %d", dm.GetResponseCount())
	}
}

func TestDiscussionMode_Start(t *testing.T) {
	config := DefaultDiscussionConfig()
	dm := NewDiscussionMode(config)

	dm.Start()

	if !dm.IsRunning() {
		t.Error("expected discussion to be running after Start")
	}
}

func TestDiscussionMode_Stop(t *testing.T) {
	config := DefaultDiscussionConfig()
	dm := NewDiscussionMode(config)

	dm.Start()
	dm.Stop()

	if dm.IsRunning() {
		t.Error("expected discussion to stop after Stop")
	}
}

func TestDiscussionMode_MaxResponses(t *testing.T) {
	config := DiscussionConfig{
		MaxResponses:   3,
		EnableChaining: true,
	}
	dm := NewDiscussionMode(config)
	dm.Start()

	avatars := createTestAvatars()

	// Simulate responses up to max
	for i := range 3 {
		if !dm.CanContinue() {
			t.Errorf("expected CanContinue to be true at response %d", i)
		}
		dm.RecordResponse(avatars[i%len(avatars)])
	}

	// Should stop after max responses
	if dm.CanContinue() {
		t.Error("expected CanContinue to be false after max responses")
	}
}

func TestDiscussionMode_RecordResponse(t *testing.T) {
	config := DefaultDiscussionConfig()
	dm := NewDiscussionMode(config)
	dm.Start()

	avatars := createTestAvatars()
	dm.RecordResponse(avatars[0])

	if dm.GetResponseCount() != 1 {
		t.Errorf("expected 1 response, got %d", dm.GetResponseCount())
	}

	lastResponder := dm.GetLastResponder()
	if lastResponder == nil || lastResponder.ID != avatars[0].ID {
		t.Error("expected last responder to be the recorded avatar")
	}
}

func TestDiscussionMode_GetNextResponder_ExcludesLastResponder(t *testing.T) {
	config := DiscussionConfig{
		MaxResponses:      10,
		EnableChaining:    true,
		ExcludeLastSender: true,
	}
	dm := NewDiscussionMode(config)
	dm.Start()

	avatars := createTestAvatars()
	dm.RecordResponse(avatars[0])

	// Get next responder - should not be the same as last
	next := dm.GetNextResponder(avatars)
	if next != nil && next.ID == avatars[0].ID {
		t.Error("next responder should not be the same as last responder")
	}
}

func TestDiscussionMode_GetNextResponder_AllAvatarsExhausted(t *testing.T) {
	config := DiscussionConfig{
		MaxResponses:      10,
		EnableChaining:    true,
		ExcludeLastSender: true,
	}
	dm := NewDiscussionMode(config)
	dm.Start()

	// Single avatar case
	singleAvatar := []models.Avatar{{ID: 1, Name: "Solo"}}
	dm.RecordResponse(singleAvatar[0])

	// With ExcludeLastSender, no next responder available
	next := dm.GetNextResponder(singleAvatar)
	if next != nil {
		t.Error("expected nil when all avatars are exhausted")
	}
}

func TestDiscussionMode_ChainingDisabled(t *testing.T) {
	config := DiscussionConfig{
		MaxResponses:   10,
		EnableChaining: false,
	}
	dm := NewDiscussionMode(config)
	dm.Start()

	avatars := createTestAvatars()
	dm.RecordResponse(avatars[0])

	// Should not continue when chaining is disabled
	if dm.CanContinue() {
		t.Error("expected CanContinue to be false when chaining is disabled")
	}
}

func TestDiscussionMode_Reset(t *testing.T) {
	config := DefaultDiscussionConfig()
	dm := NewDiscussionMode(config)
	dm.Start()

	avatars := createTestAvatars()
	dm.RecordResponse(avatars[0])
	dm.RecordResponse(avatars[1])

	dm.Reset()

	if dm.GetResponseCount() != 0 {
		t.Errorf("expected 0 responses after reset, got %d", dm.GetResponseCount())
	}
	if dm.GetLastResponder() != nil {
		t.Error("expected nil last responder after reset")
	}
}

func TestDefaultDiscussionConfig(t *testing.T) {
	config := DefaultDiscussionConfig()

	if config.MaxResponses != 5 {
		t.Errorf("expected default MaxResponses to be 5, got %d", config.MaxResponses)
	}
	if !config.EnableChaining {
		t.Error("expected default EnableChaining to be true")
	}
}

func TestDiscussionMode_ResponseHistory(t *testing.T) {
	config := DefaultDiscussionConfig()
	dm := NewDiscussionMode(config)
	dm.Start()

	avatars := createTestAvatars()
	dm.RecordResponse(avatars[0])
	dm.RecordResponse(avatars[1])
	dm.RecordResponse(avatars[2])

	history := dm.GetResponseHistory()
	if len(history) != 3 {
		t.Errorf("expected 3 items in history, got %d", len(history))
	}

	// Verify order
	if history[0].ID != avatars[0].ID {
		t.Error("history order incorrect")
	}
}

