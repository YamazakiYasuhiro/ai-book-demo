package logic

import (
	"testing"
)

func TestFormatUserMessage(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:    "simple message",
			content: "こんにちは",
			expected: "Name: ユーザ\nMessage:\nこんにちは",
		},
		{
			name:    "multiline message",
			content: "こんにちは\n今日はいい天気ですね",
			expected: "Name: ユーザ\nMessage:\nこんにちは\n今日はいい天気ですね",
		},
		{
			name:    "empty message",
			content: "",
			expected: "Name: ユーザ\nMessage:\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatUserMessage(tt.content)
			if result != tt.expected {
				t.Errorf("FormatUserMessage(%q) = %q, want %q", tt.content, result, tt.expected)
			}
		})
	}
}

func TestFormatAvatarMessage(t *testing.T) {
	tests := []struct {
		name       string
		avatarName string
		content    string
		expected   string
	}{
		{
			name:       "simple message",
			avatarName: "アバターA",
			content:    "こんにちは",
			expected:   "Name: (Avatar) アバターA\nMessage:\nこんにちは",
		},
		{
			name:       "multiline message",
			avatarName: "Assistant",
			content:    "Hello\nHow can I help?",
			expected:   "Name: (Avatar) Assistant\nMessage:\nHello\nHow can I help?",
		},
		{
			name:       "empty message",
			avatarName: "Bot",
			content:    "",
			expected:   "Name: (Avatar) Bot\nMessage:\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAvatarMessage(tt.avatarName, tt.content)
			if result != tt.expected {
				t.Errorf("FormatAvatarMessage(%q, %q) = %q, want %q", tt.avatarName, tt.content, result, tt.expected)
			}
		})
	}
}

func TestFormatMessageHistory(t *testing.T) {
	tests := []struct {
		name           string
		messages       []MessageForFormat
		currentAvatar  string
		expectedResult string
	}{
		{
			name: "mixed messages excluding current avatar",
			messages: []MessageForFormat{
				{SenderType: SenderTypeUserFormat, SenderName: "", Content: "こんにちは"},
				{SenderType: SenderTypeAvatarFormat, SenderName: "アバターA", Content: "はじめまして"},
				{SenderType: SenderTypeAvatarFormat, SenderName: "アバターB", Content: "よろしく"},
			},
			currentAvatar: "アバターA",
			expectedResult: "Name: ユーザ\nMessage:\nこんにちは\n\n---\n\nName: (Avatar) アバターB\nMessage:\nよろしく",
		},
		{
			name: "only user messages",
			messages: []MessageForFormat{
				{SenderType: SenderTypeUserFormat, SenderName: "", Content: "質問です"},
			},
			currentAvatar:  "Bot",
			expectedResult: "Name: ユーザ\nMessage:\n質問です",
		},
		{
			name:           "empty messages",
			messages:       []MessageForFormat{},
			currentAvatar:  "Bot",
			expectedResult: "",
		},
		{
			name: "all messages from current avatar",
			messages: []MessageForFormat{
				{SenderType: SenderTypeAvatarFormat, SenderName: "Bot", Content: "Hello"},
			},
			currentAvatar:  "Bot",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMessageHistory(tt.messages, tt.currentAvatar)
			if result != tt.expectedResult {
				t.Errorf("FormatMessageHistory() = %q, want %q", result, tt.expectedResult)
			}
		})
	}
}

