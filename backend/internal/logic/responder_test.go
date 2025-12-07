package logic

import (
	"testing"

	"multi-avatar-chat/internal/models"
)

func createTestAvatars() []models.Avatar {
	return []models.Avatar{
		{ID: 1, Name: "CodeExpert", Prompt: "You are a programming expert"},
		{ID: 2, Name: "Designer", Prompt: "You are a UI/UX designer"},
		{ID: 3, Name: "Manager", Prompt: "You are a project manager"},
	}
}

func TestSelectResponders_MentionedAvatars(t *testing.T) {
	avatars := createTestAvatars()
	content := "@CodeExpert プログラミングについて質問です"

	responders := SelectResponders(content, avatars)

	if len(responders) != 1 {
		t.Fatalf("expected 1 responder, got %d", len(responders))
	}
	if responders[0].ID != 1 {
		t.Errorf("expected CodeExpert (ID=1), got ID=%d", responders[0].ID)
	}
}

func TestSelectResponders_MultipleMentions(t *testing.T) {
	avatars := createTestAvatars()
	content := "@CodeExpert @Designer デザインパターンについて教えてください"

	responders := SelectResponders(content, avatars)

	if len(responders) != 2 {
		t.Fatalf("expected 2 responders, got %d", len(responders))
	}

	// Check both are present
	found := make(map[int64]bool)
	for _, r := range responders {
		found[r.ID] = true
	}
	if !found[1] || !found[2] {
		t.Error("expected both CodeExpert and Designer to be selected")
	}
}

func TestSelectResponders_NoMention_ReturnsAll(t *testing.T) {
	avatars := createTestAvatars()
	content := "プロジェクトの進め方について相談です"

	responders := SelectResponders(content, avatars)

	// When no mention, all avatars should potentially respond
	// The actual selection can be based on content analysis
	if len(responders) == 0 {
		t.Error("expected at least one responder when no mention")
	}
}

func TestSelectResponders_UnknownMention(t *testing.T) {
	avatars := createTestAvatars()
	content := "@UnknownBot hello"

	responders := SelectResponders(content, avatars)

	// Unknown mention should not select any specific avatar
	// Falls back to normal selection
	if len(responders) == 0 {
		t.Error("expected fallback responders when unknown mention")
	}
}

func TestSelectResponders_EmptyContent(t *testing.T) {
	avatars := createTestAvatars()
	content := ""

	responders := SelectResponders(content, avatars)

	// Empty content should return all avatars as potential responders
	if len(responders) != len(avatars) {
		t.Errorf("expected %d responders for empty content, got %d", len(avatars), len(responders))
	}
}

func TestSelectResponders_NoAvatars(t *testing.T) {
	avatars := []models.Avatar{}
	content := "@Someone hello"

	responders := SelectResponders(content, avatars)

	if len(responders) != 0 {
		t.Errorf("expected 0 responders with no avatars, got %d", len(responders))
	}
}

func TestSelectResponders_CaseInsensitiveMention(t *testing.T) {
	avatars := createTestAvatars()
	content := "@codeexpert help me"

	responders := SelectResponders(content, avatars)

	if len(responders) != 1 {
		t.Fatalf("expected 1 responder, got %d", len(responders))
	}
	if responders[0].Name != "CodeExpert" {
		t.Errorf("expected CodeExpert, got %s", responders[0].Name)
	}
}

func TestResponderResult_HasMentions(t *testing.T) {
	avatars := createTestAvatars()
	content := "@CodeExpert help"

	result := AnalyzeResponse(content, avatars)

	if !result.HasMentions {
		t.Error("expected HasMentions to be true")
	}
	if len(result.MentionedNames) != 1 {
		t.Errorf("expected 1 mentioned name, got %d", len(result.MentionedNames))
	}
}

func TestResponderResult_NoMentions(t *testing.T) {
	avatars := createTestAvatars()
	content := "General question"

	result := AnalyzeResponse(content, avatars)

	if result.HasMentions {
		t.Error("expected HasMentions to be false")
	}
	if len(result.MentionedNames) != 0 {
		t.Errorf("expected 0 mentioned names, got %d", len(result.MentionedNames))
	}
}

func TestResponderResult_CleanedContent(t *testing.T) {
	avatars := createTestAvatars()
	content := "@CodeExpert プログラミングを教えて"

	result := AnalyzeResponse(content, avatars)

	expected := "プログラミングを教えて"
	if result.CleanedContent != expected {
		t.Errorf("expected cleaned content '%s', got '%s'", expected, result.CleanedContent)
	}
}

