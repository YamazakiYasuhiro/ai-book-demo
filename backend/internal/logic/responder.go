package logic

import (
	"strings"

	"multi-avatar-chat/internal/models"
)

// ResponderResult contains the analysis of a message for response selection
type ResponderResult struct {
	// HasMentions indicates if the message contains @mentions
	HasMentions bool

	// MentionedNames contains the names mentioned in the message
	MentionedNames []string

	// Responders contains the avatars selected to respond
	Responders []models.Avatar

	// CleanedContent is the message content with mentions removed
	CleanedContent string
}

// SelectResponders selects which avatars should respond to a message
// If mentions are present, only mentioned avatars respond
// Otherwise, all avatars are potential responders
func SelectResponders(content string, avatars []models.Avatar) []models.Avatar {
	if len(avatars) == 0 {
		return []models.Avatar{}
	}

	// Extract avatar names for matching
	avatarNames := make([]string, len(avatars))
	nameToAvatar := make(map[string]models.Avatar)
	for i, avatar := range avatars {
		avatarNames[i] = avatar.Name
		nameToAvatar[strings.ToLower(avatar.Name)] = avatar
	}

	// Parse mentions from content
	mentions := ParseMentions(content)
	if len(mentions) == 0 {
		// No mentions - return all avatars as potential responders
		return avatars
	}

	// Match mentions to avatar names
	matchedNames := MatchAvatarNames(mentions, avatarNames)
	if len(matchedNames) == 0 {
		// Mentions don't match any avatar - return all as fallback
		return avatars
	}

	// Return only the mentioned avatars
	var responders []models.Avatar
	for _, name := range matchedNames {
		if avatar, ok := nameToAvatar[strings.ToLower(name)]; ok {
			responders = append(responders, avatar)
		}
	}

	return responders
}

// AnalyzeResponse provides detailed analysis of a message for response handling
func AnalyzeResponse(content string, avatars []models.Avatar) ResponderResult {
	result := ResponderResult{
		CleanedContent: RemoveMentions(content),
	}

	// Extract avatar names
	avatarNames := make([]string, len(avatars))
	for i, avatar := range avatars {
		avatarNames[i] = avatar.Name
	}

	// Parse and match mentions
	mentions := ParseMentions(content)
	result.MentionedNames = MatchAvatarNames(mentions, avatarNames)
	result.HasMentions = len(result.MentionedNames) > 0

	// Select responders
	result.Responders = SelectResponders(content, avatars)

	return result
}

// SelectSingleResponder selects a single avatar to respond
// Prefers mentioned avatar, otherwise returns the first available
func SelectSingleResponder(content string, avatars []models.Avatar) *models.Avatar {
	responders := SelectResponders(content, avatars)
	if len(responders) == 0 {
		return nil
	}
	return &responders[0]
}

// FilterByIDs filters avatars to only those with the specified IDs
func FilterByIDs(avatars []models.Avatar, ids []int64) []models.Avatar {
	idSet := make(map[int64]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	var filtered []models.Avatar
	for _, avatar := range avatars {
		if idSet[avatar.ID] {
			filtered = append(filtered, avatar)
		}
	}
	return filtered
}

