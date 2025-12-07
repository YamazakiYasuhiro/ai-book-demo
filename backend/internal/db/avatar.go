package db

import (
	"database/sql"
	"time"

	"multi-avatar-chat/internal/models"
)

// CreateAvatar inserts a new avatar into the database
func (d *DB) CreateAvatar(name, prompt, openaiAssistantID string) (*models.Avatar, error) {
	return WithLockResult(d, func() (*models.Avatar, error) {
		result, err := d.db.Exec(
			`INSERT INTO avatars (name, prompt, openai_assistant_id) VALUES (?, ?, ?)`,
			name, prompt, openaiAssistantID,
		)
		if err != nil {
			return nil, err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, err
		}

		return &models.Avatar{
			ID:                id,
			Name:              name,
			Prompt:            prompt,
			OpenAIAssistantID: openaiAssistantID,
			CreatedAt:         time.Now(),
		}, nil
	})
}

// GetAvatar retrieves an avatar by ID
func (d *DB) GetAvatar(id int64) (*models.Avatar, error) {
	return WithLockResult(d, func() (*models.Avatar, error) {
		row := d.db.QueryRow(
			`SELECT id, name, prompt, openai_assistant_id, created_at FROM avatars WHERE id = ?`,
			id,
		)

		var avatar models.Avatar
		var assistantID sql.NullString
		err := row.Scan(&avatar.ID, &avatar.Name, &avatar.Prompt, &assistantID, &avatar.CreatedAt)
		if err != nil {
			return nil, err
		}

		if assistantID.Valid {
			avatar.OpenAIAssistantID = assistantID.String
		}

		return &avatar, nil
	})
}

// GetAllAvatars retrieves all avatars
func (d *DB) GetAllAvatars() ([]models.Avatar, error) {
	return WithLockResult(d, func() ([]models.Avatar, error) {
		rows, err := d.db.Query(
			`SELECT id, name, prompt, openai_assistant_id, created_at FROM avatars ORDER BY created_at DESC`,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var avatars []models.Avatar
		for rows.Next() {
			var avatar models.Avatar
			var assistantID sql.NullString
			if err := rows.Scan(&avatar.ID, &avatar.Name, &avatar.Prompt, &assistantID, &avatar.CreatedAt); err != nil {
				return nil, err
			}
			if assistantID.Valid {
				avatar.OpenAIAssistantID = assistantID.String
			}
			avatars = append(avatars, avatar)
		}

		return avatars, rows.Err()
	})
}

// UpdateAvatar updates an existing avatar
func (d *DB) UpdateAvatar(id int64, name, prompt, openaiAssistantID string) (*models.Avatar, error) {
	return WithLockResult(d, func() (*models.Avatar, error) {
		_, err := d.db.Exec(
			`UPDATE avatars SET name = ?, prompt = ?, openai_assistant_id = ? WHERE id = ?`,
			name, prompt, openaiAssistantID, id,
		)
		if err != nil {
			return nil, err
		}

		// Fetch updated avatar
		row := d.db.QueryRow(
			`SELECT id, name, prompt, openai_assistant_id, created_at FROM avatars WHERE id = ?`,
			id,
		)

		var avatar models.Avatar
		var assistantIDNull sql.NullString
		err = row.Scan(&avatar.ID, &avatar.Name, &avatar.Prompt, &assistantIDNull, &avatar.CreatedAt)
		if err != nil {
			return nil, err
		}

		if assistantIDNull.Valid {
			avatar.OpenAIAssistantID = assistantIDNull.String
		}

		return &avatar, nil
	})
}

// DeleteAvatar deletes an avatar by ID
func (d *DB) DeleteAvatar(id int64) error {
	return d.WithLock(func() error {
		result, err := d.db.Exec(`DELETE FROM avatars WHERE id = ?`, id)
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

