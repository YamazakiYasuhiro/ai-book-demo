package logic

import (
	"regexp"
	"strings"
)

// mentionRegex matches @username patterns with Unicode support
// First character must be a letter (any language), followed by letters, numbers, or underscores
var mentionRegex = regexp.MustCompile(`@(\p{L}[\p{L}\p{N}_]*)`)

// ParseMentions extracts mention names from a message content
// Returns a unique list of mentioned names (without @ prefix)
func ParseMentions(content string) []string {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return []string{}
	}

	// Use a map to track unique mentions
	seen := make(map[string]bool)
	var mentions []string

	for _, match := range matches {
		if len(match) > 1 {
			name := match[1]
			if !seen[name] {
				seen[name] = true
				mentions = append(mentions, name)
			}
		}
	}

	return mentions
}

// RemoveMentions removes all @mentions from the content
func RemoveMentions(content string) string {
	result := mentionRegex.ReplaceAllString(content, "")
	// Clean up extra whitespace
	result = strings.TrimSpace(result)
	// Replace multiple spaces with single space
	spaceRegex := regexp.MustCompile(`\s+`)
	result = spaceRegex.ReplaceAllString(result, " ")
	return result
}

// MatchAvatarNames matches mention names against available avatar names (case-insensitive)
// Returns the actual avatar names that were matched
func MatchAvatarNames(mentions []string, avatarNames []string) []string {
	// Create lowercase lookup map
	nameMap := make(map[string]string)
	for _, name := range avatarNames {
		nameMap[strings.ToLower(name)] = name
	}

	var matched []string
	for _, mention := range mentions {
		if actualName, ok := nameMap[strings.ToLower(mention)]; ok {
			matched = append(matched, actualName)
		}
	}

	return matched
}

// ExtractMentionedAvatars combines parsing and matching
// Returns the avatar names that were mentioned in the content
func ExtractMentionedAvatars(content string, avatarNames []string) []string {
	mentions := ParseMentions(content)
	return MatchAvatarNames(mentions, avatarNames)
}

