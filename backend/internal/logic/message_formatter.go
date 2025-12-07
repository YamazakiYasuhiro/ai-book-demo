package logic

import (
	"fmt"
	"strings"
)

// SenderTypeFormat represents the sender type for message formatting
type SenderTypeFormat string

const (
	SenderTypeUserFormat   SenderTypeFormat = "user"
	SenderTypeAvatarFormat SenderTypeFormat = "avatar"
)

// MessageForFormat represents a message structure for formatting
type MessageForFormat struct {
	SenderType SenderTypeFormat
	SenderName string
	Content    string
}

// FormatUserMessage formats a user's message for OpenAI API
// Format:
//
//	Name: ユーザ
//	Message:
//	{content}
func FormatUserMessage(content string) string {
	return fmt.Sprintf("Name: ユーザ\nMessage:\n%s", content)
}

// FormatAvatarMessage formats another avatar's message for OpenAI API
// Format:
//
//	Name: (Avatar) {avatarName}
//	Message:
//	{content}
func FormatAvatarMessage(avatarName, content string) string {
	return fmt.Sprintf("Name: (Avatar) %s\nMessage:\n%s", avatarName, content)
}

// FormatMessageHistory formats a list of messages excluding the current avatar's messages
// Returns formatted string with messages separated by "---"
func FormatMessageHistory(messages []MessageForFormat, currentAvatarName string) string {
	var formatted []string

	for _, msg := range messages {
		// Skip messages from the current avatar
		if msg.SenderType == SenderTypeAvatarFormat && msg.SenderName == currentAvatarName {
			continue
		}

		var formattedMsg string
		if msg.SenderType == SenderTypeUserFormat {
			formattedMsg = FormatUserMessage(msg.Content)
		} else {
			formattedMsg = FormatAvatarMessage(msg.SenderName, msg.Content)
		}
		formatted = append(formatted, formattedMsg)
	}

	return strings.Join(formatted, "\n\n---\n\n")
}
