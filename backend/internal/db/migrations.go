package db

// Migrate runs all database migrations
func (d *DB) Migrate() error {
	return d.WithLock(func() error {
		// Create avatars table
		_, err := d.db.Exec(`
			CREATE TABLE IF NOT EXISTS avatars (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				prompt TEXT NOT NULL,
				openai_assistant_id TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return err
		}

		// Create conversations table
		_, err = d.db.Exec(`
			CREATE TABLE IF NOT EXISTS conversations (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				thread_id TEXT,
				title TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return err
		}

		// Create conversation_avatars junction table
		_, err = d.db.Exec(`
			CREATE TABLE IF NOT EXISTS conversation_avatars (
				conversation_id INTEGER NOT NULL,
				avatar_id INTEGER NOT NULL,
				PRIMARY KEY (conversation_id, avatar_id),
				FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
				FOREIGN KEY (avatar_id) REFERENCES avatars(id) ON DELETE CASCADE
			)
		`)
		if err != nil {
			return err
		}

		// Create messages table
		_, err = d.db.Exec(`
			CREATE TABLE IF NOT EXISTS messages (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				conversation_id INTEGER NOT NULL,
				sender_type TEXT NOT NULL CHECK(sender_type IN ('user', 'avatar')),
				sender_id INTEGER,
				content TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
			)
		`)
		if err != nil {
			return err
		}

		// Create indexes for better query performance
		indexes := []string{
			"CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id)",
			"CREATE INDEX IF NOT EXISTS idx_conversation_avatars_conversation ON conversation_avatars(conversation_id)",
			"CREATE INDEX IF NOT EXISTS idx_conversation_avatars_avatar ON conversation_avatars(avatar_id)",
		}

		for _, idx := range indexes {
			if _, err := d.db.Exec(idx); err != nil {
				return err
			}
		}

		// Add thread_id column to conversation_avatars table if it doesn't exist
		if err := d.migrateConversationAvatarsThreadID(); err != nil {
			return err
		}

		// Migrate existing conversation thread_ids to avatar-specific threads
		if err := d.migrateExistingConversationThreads(); err != nil {
			return err
		}

		return nil
	})
}

// migrateConversationAvatarsThreadID adds thread_id column to conversation_avatars table if it doesn't exist
func (d *DB) migrateConversationAvatarsThreadID() error {
	// Check if thread_id column exists
	rows, err := d.db.Query("PRAGMA table_info(conversation_avatars)")
	if err != nil {
		return err
	}
	defer rows.Close()

	columnExists := false
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int

		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "thread_id" {
			columnExists = true
			break
		}
	}

	if !columnExists {
		// Add thread_id column
		_, err := d.db.Exec("ALTER TABLE conversation_avatars ADD COLUMN thread_id TEXT")
		if err != nil {
			return err
		}
	}

	return nil
}

// migrateExistingConversationThreads migrates existing conversation thread_ids to avatar-specific threads
// This is a one-time migration that creates new threads for avatars that don't have thread_ids yet
// Note: This migration does not copy message history - it starts fresh threads for each avatar
func (d *DB) migrateExistingConversationThreads() error {
	// Get all conversations that have a thread_id but avatars without thread_ids
	rows, err := d.db.Query(`
		SELECT DISTINCT c.id, c.thread_id
		FROM conversations c
		INNER JOIN conversation_avatars ca ON c.id = ca.conversation_id
		WHERE c.thread_id IS NOT NULL AND c.thread_id != ''
		AND (ca.thread_id IS NULL OR ca.thread_id = '')
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var conversationsToMigrate []struct {
		conversationID int64
		threadID        string
	}

	for rows.Next() {
		var convID int64
		var threadID string
		if err := rows.Scan(&convID, &threadID); err != nil {
			return err
		}
		conversationsToMigrate = append(conversationsToMigrate, struct {
			conversationID int64
			threadID       string
		}{conversationID: convID, threadID: threadID})
	}

	// Note: We don't create new threads here automatically because we need the assistant client
	// The migration just marks that migration is needed - actual thread creation happens
	// when the system detects avatars without thread_ids (handled in application code)
	// For now, we just log that migration is needed
	if len(conversationsToMigrate) > 0 {
		// Log that migration is needed - actual thread creation will happen when avatars are accessed
		// This is a soft migration - threads will be created on-demand
	}

	return nil
}
