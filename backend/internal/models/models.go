package models

import "time"

// Avatar represents a chat avatar with AI personality
type Avatar struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Prompt            string    `json:"prompt"`
	OpenAIAssistantID string    `json:"openai_assistant_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// Conversation represents a chat session
type Conversation struct {
	ID        int64     `json:"id"`
	ThreadID  string    `json:"thread_id,omitempty"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// SenderType defines who sent the message
type SenderType string

const (
	SenderTypeUser   SenderType = "user"
	SenderTypeAvatar SenderType = "avatar"
)

// Message represents a single message in a conversation
type Message struct {
	ID             int64      `json:"id"`
	ConversationID int64      `json:"conversation_id"`
	SenderType     SenderType `json:"sender_type"`
	SenderID       *int64     `json:"sender_id,omitempty"`
	Content        string     `json:"content"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ConversationAvatar represents avatar participation in a conversation
type ConversationAvatar struct {
	ConversationID int64  `json:"conversation_id"`
	AvatarID       int64  `json:"avatar_id"`
	ThreadID       string `json:"thread_id,omitempty"`
}
