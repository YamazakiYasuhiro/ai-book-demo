package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"multi-avatar-chat/internal/assistant"
	"multi-avatar-chat/internal/db"
)

// AvatarHandler handles avatar-related HTTP requests
type AvatarHandler struct {
	db        *db.DB
	assistant *assistant.Client
}

// NewAvatarHandler creates a new avatar handler
func NewAvatarHandler(database *db.DB, assistantClient *assistant.Client) *AvatarHandler {
	return &AvatarHandler{
		db:        database,
		assistant: assistantClient,
	}
}

// CreateAvatarRequest represents the request body for creating an avatar
type CreateAvatarRequest struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// AvatarResponse represents an avatar in API responses
type AvatarResponse struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Prompt            string `json:"prompt"`
	OpenAIAssistantID string `json:"openai_assistant_id,omitempty"`
	CreatedAt         string `json:"created_at"`
}

// Create handles POST /api/avatars
func (h *AvatarHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateAvatarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Prompt == "" {
		http.Error(w, "Name and prompt are required", http.StatusBadRequest)
		return
	}

	// Add user priority instruction to prompt
	userPriorityPrompt := "【重要】`Name: ユーザ` となっているメッセージがユーザの意見です。あなたはこれを最重視して発言をする必要があります。ユーザの意見を尊重し、それに基づいて応答してください。\n\n" + req.Prompt

	// Create OpenAI Assistant
	var assistantID string
	if h.assistant != nil {
		openAIAssistant, err := h.assistant.CreateAssistant(req.Name, userPriorityPrompt)
		if err != nil {
			http.Error(w, "Failed to create OpenAI assistant: "+err.Error(), http.StatusInternalServerError)
			return
		}
		assistantID = openAIAssistant.ID
	}

	// Save to database
	avatar, err := h.db.CreateAvatar(req.Name, req.Prompt, assistantID)
	if err != nil {
		http.Error(w, "Failed to create avatar", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AvatarResponse{
		ID:                avatar.ID,
		Name:              avatar.Name,
		Prompt:            avatar.Prompt,
		OpenAIAssistantID: avatar.OpenAIAssistantID,
		CreatedAt:         avatar.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// List handles GET /api/avatars
func (h *AvatarHandler) List(w http.ResponseWriter, r *http.Request) {
	avatars, err := h.db.GetAllAvatars()
	if err != nil {
		http.Error(w, "Failed to get avatars", http.StatusInternalServerError)
		return
	}

	response := make([]AvatarResponse, len(avatars))
	for i, avatar := range avatars {
		response[i] = AvatarResponse{
			ID:                avatar.ID,
			Name:              avatar.Name,
			Prompt:            avatar.Prompt,
			OpenAIAssistantID: avatar.OpenAIAssistantID,
			CreatedAt:         avatar.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Get handles GET /api/avatars/{id}
func (h *AvatarHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid avatar ID", http.StatusBadRequest)
		return
	}

	avatar, err := h.db.GetAvatar(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Avatar not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to get avatar", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AvatarResponse{
		ID:                avatar.ID,
		Name:              avatar.Name,
		Prompt:            avatar.Prompt,
		OpenAIAssistantID: avatar.OpenAIAssistantID,
		CreatedAt:         avatar.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// UpdateAvatarRequest represents the request body for updating an avatar
type UpdateAvatarRequest struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// Update handles PUT /api/avatars/{id}
func (h *AvatarHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid avatar ID", http.StatusBadRequest)
		return
	}

	var req UpdateAvatarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get existing avatar
	existing, err := h.db.GetAvatar(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Avatar not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to get avatar", http.StatusInternalServerError)
		return
	}

	// Update OpenAI Assistant if prompt changed
	assistantID := existing.OpenAIAssistantID
	if h.assistant != nil && existing.OpenAIAssistantID != "" && (req.Prompt != existing.Prompt || req.Name != existing.Name) {
		_, err := h.assistant.UpdateAssistant(existing.OpenAIAssistantID, req.Name, req.Prompt)
		if err != nil {
			http.Error(w, "Failed to update OpenAI assistant: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Update in database
	avatar, err := h.db.UpdateAvatar(id, req.Name, req.Prompt, assistantID)
	if err != nil {
		http.Error(w, "Failed to update avatar", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AvatarResponse{
		ID:                avatar.ID,
		Name:              avatar.Name,
		Prompt:            avatar.Prompt,
		OpenAIAssistantID: avatar.OpenAIAssistantID,
		CreatedAt:         avatar.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// Delete handles DELETE /api/avatars/{id}
func (h *AvatarHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid avatar ID", http.StatusBadRequest)
		return
	}

	// Get existing avatar to get OpenAI assistant ID
	existing, err := h.db.GetAvatar(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Avatar not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to get avatar", http.StatusInternalServerError)
		return
	}

	// Delete OpenAI Assistant
	if h.assistant != nil && existing.OpenAIAssistantID != "" {
		if err := h.assistant.DeleteAssistant(existing.OpenAIAssistantID); err != nil {
			// Log error but continue with local deletion
			// In production, you might want different behavior
		}
	}

	// Delete from database
	if err := h.db.DeleteAvatar(id); err != nil {
		http.Error(w, "Failed to delete avatar", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
