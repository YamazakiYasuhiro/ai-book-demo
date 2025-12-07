package db

import (
	"database/sql"
	"log"
	"time"

	"multi-avatar-chat/internal/models"
)

// CreateConversation creates a new conversation
func (d *DB) CreateConversation(title, threadID string) (*models.Conversation, error) {
	return WithLockResult(d, func() (*models.Conversation, error) {
		result, err := d.db.Exec(
			`INSERT INTO conversations (title, thread_id) VALUES (?, ?)`,
			title, threadID,
		)
		if err != nil {
			return nil, err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, err
		}

		return &models.Conversation{
			ID:        id,
			Title:     title,
			ThreadID:  threadID,
			CreatedAt: time.Now(),
		}, nil
	})
}

// GetConversation retrieves a conversation by ID
func (d *DB) GetConversation(id int64) (*models.Conversation, error) {
	return WithLockResult(d, func() (*models.Conversation, error) {
		row := d.db.QueryRow(
			`SELECT id, title, thread_id, created_at FROM conversations WHERE id = ?`,
			id,
		)

		var conv models.Conversation
		var threadID sql.NullString
		err := row.Scan(&conv.ID, &conv.Title, &threadID, &conv.CreatedAt)
		if err != nil {
			return nil, err
		}

		if threadID.Valid {
			conv.ThreadID = threadID.String
		}

		return &conv, nil
	})
}

// GetAllConversations retrieves all conversations
func (d *DB) GetAllConversations() ([]models.Conversation, error) {
	return WithLockResult(d, func() ([]models.Conversation, error) {
		rows, err := d.db.Query(
			`SELECT id, title, thread_id, created_at FROM conversations ORDER BY created_at DESC`,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var conversations []models.Conversation
		for rows.Next() {
			var conv models.Conversation
			var threadID sql.NullString
			if err := rows.Scan(&conv.ID, &conv.Title, &threadID, &conv.CreatedAt); err != nil {
				return nil, err
			}
			if threadID.Valid {
				conv.ThreadID = threadID.String
			}
			conversations = append(conversations, conv)
		}

		return conversations, rows.Err()
	})
}

// DeleteConversation deletes a conversation and its messages
func (d *DB) DeleteConversation(id int64) error {
	return d.WithLock(func() error {
		result, err := d.db.Exec(`DELETE FROM conversations WHERE id = ?`, id)
		if err != nil {
			return err
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if rows == 0 {
			return sql.ErrNoRows
		}

		return nil
	})
}

// AddAvatarToConversation adds an avatar as a participant in a conversation
func (d *DB) AddAvatarToConversation(conversationID, avatarID int64) error {
	return d.AddAvatarToConversationWithThreadID(conversationID, avatarID, "")
}

// AddAvatarToConversationWithThreadID adds an avatar as a participant in a conversation with a thread ID
func (d *DB) AddAvatarToConversationWithThreadID(conversationID, avatarID int64, threadID string) error {
	return d.WithLock(func() error {
		_, err := d.db.Exec(
			`INSERT OR IGNORE INTO conversation_avatars (conversation_id, avatar_id, thread_id) VALUES (?, ?, ?)`,
			conversationID, avatarID, threadID,
		)
		return err
	})
}

// GetConversationAvatars retrieves all avatars in a conversation
func (d *DB) GetConversationAvatars(conversationID int64) ([]models.Avatar, error) {
	return WithLockResult(d, func() ([]models.Avatar, error) {
		log.Printf("[DB] GetConversationAvatars started conversation_id=%d", conversationID)

		rows, err := d.db.Query(`
			SELECT a.id, a.name, a.prompt, a.openai_assistant_id, a.created_at
			FROM avatars a
			INNER JOIN conversation_avatars ca ON a.id = ca.avatar_id
			WHERE ca.conversation_id = ?
		`, conversationID)
		if err != nil {
			log.Printf("[DB] GetConversationAvatars failed: query error err=%v", err)
			return nil, err
		}
		defer rows.Close()

		var avatars []models.Avatar
		for rows.Next() {
			var avatar models.Avatar
			var assistantID sql.NullString
			if err := rows.Scan(&avatar.ID, &avatar.Name, &avatar.Prompt, &assistantID, &avatar.CreatedAt); err != nil {
				log.Printf("[DB] GetConversationAvatars failed: scan error err=%v", err)
				return nil, err
			}
			if assistantID.Valid {
				avatar.OpenAIAssistantID = assistantID.String
			}
			avatars = append(avatars, avatar)
		}

		// Log avatar names
		avatarNames := make([]string, len(avatars))
		for i, a := range avatars {
			avatarNames[i] = a.Name
		}
		log.Printf("[DB] GetConversationAvatars completed conversation_id=%d count=%d names=%v", conversationID, len(avatars), avatarNames)

		return avatars, rows.Err()
	})
}

// ConversationAvatarsWithThreads represents avatars with their thread IDs
type ConversationAvatarsWithThreads struct {
	Avatars   []models.Avatar
	ThreadIDs []string
}

// GetConversationAvatarsWithThreads retrieves all avatars in a conversation with their thread IDs
func (d *DB) GetConversationAvatarsWithThreads(conversationID int64) ([]models.Avatar, []string, error) {
	result, err := WithLockResult(d, func() (ConversationAvatarsWithThreads, error) {
		log.Printf("[DB] GetConversationAvatarsWithThreads started conversation_id=%d", conversationID)

		rows, err := d.db.Query(`
			SELECT a.id, a.name, a.prompt, a.openai_assistant_id, a.created_at, ca.thread_id
			FROM avatars a
			INNER JOIN conversation_avatars ca ON a.id = ca.avatar_id
			WHERE ca.conversation_id = ?
		`, conversationID)
		if err != nil {
			log.Printf("[DB] GetConversationAvatarsWithThreads failed: query error err=%v", err)
			return ConversationAvatarsWithThreads{}, err
		}
		defer rows.Close()

		var avatars []models.Avatar
		var threadIDs []string
		for rows.Next() {
			var avatar models.Avatar
			var assistantID sql.NullString
			var threadID sql.NullString
			if err := rows.Scan(&avatar.ID, &avatar.Name, &avatar.Prompt, &assistantID, &avatar.CreatedAt, &threadID); err != nil {
				log.Printf("[DB] GetConversationAvatarsWithThreads failed: scan error err=%v", err)
				return ConversationAvatarsWithThreads{}, err
			}
			if assistantID.Valid {
				avatar.OpenAIAssistantID = assistantID.String
			}
			avatars = append(avatars, avatar)
			if threadID.Valid {
				threadIDs = append(threadIDs, threadID.String)
			} else {
				threadIDs = append(threadIDs, "")
			}
		}

		log.Printf("[DB] GetConversationAvatarsWithThreads completed conversation_id=%d count=%d", conversationID, len(avatars))

		return ConversationAvatarsWithThreads{
			Avatars:   avatars,
			ThreadIDs: threadIDs,
		}, rows.Err()
	})
	if err != nil {
		return nil, nil, err
	}
	return result.Avatars, result.ThreadIDs, nil
}

// CreateMessage creates a new message in a conversation
func (d *DB) CreateMessage(conversationID int64, senderType models.SenderType, senderID *int64, content string) (*models.Message, error) {
	return WithLockResult(d, func() (*models.Message, error) {
		var senderIDLog any = "nil"
		if senderID != nil {
			senderIDLog = *senderID
		}
		log.Printf("[DB] CreateMessage started conversation_id=%d sender_type=%s sender_id=%v", conversationID, senderType, senderIDLog)

		result, err := d.db.Exec(
			`INSERT INTO messages (conversation_id, sender_type, sender_id, content) VALUES (?, ?, ?, ?)`,
			conversationID, string(senderType), senderID, content,
		)
		if err != nil {
			log.Printf("[DB] CreateMessage failed: exec error err=%v", err)
			return nil, err
		}

		id, err := result.LastInsertId()
		if err != nil {
			log.Printf("[DB] CreateMessage failed: get last insert id err=%v", err)
			return nil, err
		}

		log.Printf("[DB] CreateMessage completed conversation_id=%d message_id=%d sender_type=%s", conversationID, id, senderType)

		return &models.Message{
			ID:             id,
			ConversationID: conversationID,
			SenderType:     senderType,
			SenderID:       senderID,
			Content:        content,
			CreatedAt:      time.Now(),
		}, nil
	})
}

// GetMessages retrieves all messages in a conversation
func (d *DB) GetMessages(conversationID int64) ([]models.Message, error) {
	return WithLockResult(d, func() ([]models.Message, error) {
		rows, err := d.db.Query(
			`SELECT id, conversation_id, sender_type, sender_id, content, created_at 
			FROM messages WHERE conversation_id = ? ORDER BY created_at ASC`,
			conversationID,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var messages []models.Message
		for rows.Next() {
			var msg models.Message
			var senderID sql.NullInt64
			var senderType string
			if err := rows.Scan(&msg.ID, &msg.ConversationID, &senderType, &senderID, &msg.Content, &msg.CreatedAt); err != nil {
				return nil, err
			}
			msg.SenderType = models.SenderType(senderType)
			if senderID.Valid {
				id := senderID.Int64
				msg.SenderID = &id
			}
			messages = append(messages, msg)
		}

		return messages, rows.Err()
	})
}

// RemoveAvatarFromConversation removes an avatar from a conversation
func (d *DB) RemoveAvatarFromConversation(conversationID, avatarID int64) error {
	return d.WithLock(func() error {
		log.Printf("[DB] RemoveAvatarFromConversation started conversation_id=%d avatar_id=%d", conversationID, avatarID)

		result, err := d.db.Exec(
			`DELETE FROM conversation_avatars WHERE conversation_id = ? AND avatar_id = ?`,
			conversationID, avatarID,
		)
		if err != nil {
			log.Printf("[DB] RemoveAvatarFromConversation failed: exec error err=%v", err)
			return err
		}

		rows, err := result.RowsAffected()
		if err != nil {
			log.Printf("[DB] RemoveAvatarFromConversation failed: rows affected error err=%v", err)
			return err
		}

		if rows == 0 {
			log.Printf("[DB] RemoveAvatarFromConversation: no rows affected (not found)")
			return sql.ErrNoRows
		}

		log.Printf("[DB] RemoveAvatarFromConversation completed conversation_id=%d avatar_id=%d", conversationID, avatarID)
		return nil
	})
}

// GetMessagesAfter retrieves messages with ID greater than the given ID
func (d *DB) GetMessagesAfter(conversationID int64, afterID int64) ([]models.Message, error) {
	return WithLockResult(d, func() ([]models.Message, error) {
		rows, err := d.db.Query(
			`SELECT id, conversation_id, sender_type, sender_id, content, created_at 
			FROM messages 
			WHERE conversation_id = ? AND id > ?
			ORDER BY id ASC`,
			conversationID, afterID,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var messages []models.Message
		for rows.Next() {
			var msg models.Message
			var senderID sql.NullInt64
			var senderType string
			if err := rows.Scan(&msg.ID, &msg.ConversationID, &senderType, &senderID, &msg.Content, &msg.CreatedAt); err != nil {
				return nil, err
			}
			msg.SenderType = models.SenderType(senderType)
			if senderID.Valid {
				id := senderID.Int64
				msg.SenderID = &id
			}
			messages = append(messages, msg)
		}

		return messages, rows.Err()
	})
}

// GetAllConversationAvatars retrieves all conversation-avatar pairs
func (d *DB) GetAllConversationAvatars() ([]models.ConversationAvatar, error) {
	return WithLockResult(d, func() ([]models.ConversationAvatar, error) {
		log.Printf("[DB] GetAllConversationAvatars started")

		rows, err := d.db.Query(
			`SELECT conversation_id, avatar_id, thread_id FROM conversation_avatars`,
		)
		if err != nil {
			log.Printf("[DB] GetAllConversationAvatars failed: query error err=%v", err)
			return nil, err
		}
		defer rows.Close()

		var pairs []models.ConversationAvatar
		for rows.Next() {
			var pair models.ConversationAvatar
			var threadID sql.NullString
			if err := rows.Scan(&pair.ConversationID, &pair.AvatarID, &threadID); err != nil {
				log.Printf("[DB] GetAllConversationAvatars failed: scan error err=%v", err)
				return nil, err
			}
			if threadID.Valid {
				pair.ThreadID = threadID.String
			}
			pairs = append(pairs, pair)
		}

		log.Printf("[DB] GetAllConversationAvatars completed count=%d", len(pairs))
		return pairs, rows.Err()
	})
}

// GetAvatarThreadID retrieves the thread ID for a specific avatar in a conversation
func (d *DB) GetAvatarThreadID(conversationID, avatarID int64) (string, error) {
	return WithLockResult(d, func() (string, error) {
		var threadID sql.NullString
		err := d.db.QueryRow(
			`SELECT thread_id FROM conversation_avatars WHERE conversation_id = ? AND avatar_id = ?`,
			conversationID, avatarID,
		).Scan(&threadID)
		if err != nil {
			return "", err
		}
		if threadID.Valid {
			return threadID.String, nil
		}
		return "", nil
	})
}

// UpdateAvatarThreadID updates the thread ID for an avatar in a conversation
func (d *DB) UpdateAvatarThreadID(conversationID, avatarID int64, threadID string) error {
	return d.WithLock(func() error {
		_, err := d.db.Exec(
			`UPDATE conversation_avatars SET thread_id = ? WHERE conversation_id = ? AND avatar_id = ?`,
			threadID, conversationID, avatarID,
		)
		return err
	})
}
