package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"multi-avatar-chat/internal/assistant"
	"multi-avatar-chat/internal/db"
	"multi-avatar-chat/internal/watcher"
)

// ConversationAvatarHandler handles avatar participation in conversations
type ConversationAvatarHandler struct {
	db          *db.DB
	assistant   *assistant.Client
	watcher     *watcher.WatcherManager
	broadcaster *EventBroadcaster
}

// NewConversationAvatarHandler creates a new handler
func NewConversationAvatarHandler(database *db.DB, assistantClient *assistant.Client, watcherManager *watcher.WatcherManager) *ConversationAvatarHandler {
	return &ConversationAvatarHandler{
		db:        database,
		assistant: assistantClient,
		watcher:   watcherManager,
	}
}

// SetBroadcaster sets the event broadcaster for SSE notifications
func (h *ConversationAvatarHandler) SetBroadcaster(broadcaster *EventBroadcaster) {
	h.broadcaster = broadcaster
}

// AddAvatarRequest represents the request body for adding an avatar
type AddAvatarRequest struct {
	AvatarID int64 `json:"avatar_id"`
}

// AddAvatar handles POST /api/conversations/{id}/avatars
func (h *ConversationAvatarHandler) AddAvatar(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] AddAvatar started")

	conversationID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Printf("[API] AddAvatar failed: invalid conversation ID err=%v", err)
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	var req AddAvatarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[API] AddAvatar failed: invalid request body err=%v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("[API] AddAvatar request conversation_id=%d avatar_id=%d", conversationID, req.AvatarID)

	// Verify conversation exists
	_, err = h.db.GetConversation(conversationID)
	if err == sql.ErrNoRows {
		log.Printf("[API] AddAvatar failed: conversation not found conversation_id=%d", conversationID)
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("[API] AddAvatar failed: DB error getting conversation err=%v", err)
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}

	// Verify avatar exists and get avatar info
	avatar, err := h.db.GetAvatar(req.AvatarID)
	if err == sql.ErrNoRows {
		log.Printf("[API] AddAvatar failed: avatar not found avatar_id=%d", req.AvatarID)
		http.Error(w, "Avatar not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("[API] AddAvatar failed: DB error getting avatar err=%v", err)
		http.Error(w, "Failed to get avatar", http.StatusInternalServerError)
		return
	}

	// Create OpenAI Thread for the avatar
	var threadID string
	if h.assistant != nil {
		log.Printf("[API] Creating OpenAI thread for avatar conversation_id=%d avatar_id=%d", conversationID, req.AvatarID)
		thread, err := h.assistant.CreateThread()
		if err != nil {
			log.Printf("[API] Failed to create OpenAI thread for avatar conversation_id=%d avatar_id=%d err=%v", conversationID, req.AvatarID, err)
			// Continue even if thread creation fails, but log the error
			// Add avatar without thread_id
			if err := h.db.AddAvatarToConversationWithThreadID(conversationID, req.AvatarID, ""); err != nil {
				log.Printf("[API] AddAvatar failed: DB error adding avatar err=%v", err)
				http.Error(w, "Failed to add avatar", http.StatusInternalServerError)
				return
			}
		} else {
			threadID = thread.ID
			log.Printf("[API] OpenAI thread created for avatar conversation_id=%d avatar_id=%d thread_id=%s", conversationID, req.AvatarID, threadID)

			// Add avatar to conversation with thread ID
			if err := h.db.AddAvatarToConversationWithThreadID(conversationID, req.AvatarID, threadID); err != nil {
				log.Printf("[API] AddAvatar failed: DB error adding avatar err=%v", err)
				http.Error(w, "Failed to add avatar", http.StatusInternalServerError)
				return
			}
		}
	} else {
		log.Printf("[API] OpenAI assistant client is nil, skipping thread creation for avatar_id=%d", req.AvatarID)
		// Add avatar without thread_id
		if err := h.db.AddAvatarToConversationWithThreadID(conversationID, req.AvatarID, ""); err != nil {
			log.Printf("[API] AddAvatar failed: DB error adding avatar err=%v", err)
			http.Error(w, "Failed to add avatar", http.StatusInternalServerError)
			return
		}
	}

	// Start watcher
	if h.watcher != nil {
		if err := h.watcher.StartWatcher(conversationID, req.AvatarID); err != nil {
			log.Printf("[API] AddAvatar warning: failed to start watcher err=%v", err)
			// Continue - avatar was added, watcher failure is non-fatal
		}
	}

	// Broadcast avatar joined event via SSE
	if h.broadcaster != nil {
		h.broadcaster.BroadcastAvatarJoined(conversationID, avatar.ID, avatar.Name)
		log.Printf("[API] AddAvatar broadcasted avatar_joined event conversation_id=%d avatar_id=%d",
			conversationID, avatar.ID)
	}

	log.Printf("[API] AddAvatar completed conversation_id=%d avatar_id=%d", conversationID, req.AvatarID)
	w.WriteHeader(http.StatusNoContent)
}

// RemoveAvatar handles DELETE /api/conversations/{id}/avatars/{avatar_id}
func (h *ConversationAvatarHandler) RemoveAvatar(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] RemoveAvatar started")

	conversationID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Printf("[API] RemoveAvatar failed: invalid conversation ID err=%v", err)
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	avatarID, err := strconv.ParseInt(r.PathValue("avatar_id"), 10, 64)
	if err != nil {
		log.Printf("[API] RemoveAvatar failed: invalid avatar ID err=%v", err)
		http.Error(w, "Invalid avatar ID", http.StatusBadRequest)
		return
	}

	log.Printf("[API] RemoveAvatar request conversation_id=%d avatar_id=%d", conversationID, avatarID)

	// Stop watcher first
	if h.watcher != nil {
		if err := h.watcher.StopWatcher(conversationID, avatarID); err != nil {
			log.Printf("[API] RemoveAvatar warning: failed to stop watcher err=%v", err)
			// Continue - proceed with removal
		}
	}

	// Remove from database
	if err := h.db.RemoveAvatarFromConversation(conversationID, avatarID); err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[API] RemoveAvatar failed: avatar not in conversation conversation_id=%d avatar_id=%d", conversationID, avatarID)
			http.Error(w, "Avatar not in conversation", http.StatusNotFound)
			return
		}
		log.Printf("[API] RemoveAvatar failed: DB error removing avatar err=%v", err)
		http.Error(w, "Failed to remove avatar", http.StatusInternalServerError)
		return
	}

	// Broadcast avatar left event via SSE
	if h.broadcaster != nil {
		h.broadcaster.BroadcastAvatarLeft(conversationID, avatarID)
		log.Printf("[API] RemoveAvatar broadcasted avatar_left event conversation_id=%d avatar_id=%d",
			conversationID, avatarID)
	}

	log.Printf("[API] RemoveAvatar completed conversation_id=%d avatar_id=%d", conversationID, avatarID)
	w.WriteHeader(http.StatusNoContent)
}

// ListAvatars handles GET /api/conversations/{id}/avatars
func (h *ConversationAvatarHandler) ListAvatars(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] ListAvatars started")

	conversationID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Printf("[API] ListAvatars failed: invalid conversation ID err=%v", err)
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	log.Printf("[API] ListAvatars request conversation_id=%d", conversationID)

	// Verify conversation exists
	_, err = h.db.GetConversation(conversationID)
	if err == sql.ErrNoRows {
		log.Printf("[API] ListAvatars failed: conversation not found conversation_id=%d", conversationID)
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("[API] ListAvatars failed: DB error getting conversation err=%v", err)
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}

	avatars, err := h.db.GetConversationAvatars(conversationID)
	if err != nil {
		log.Printf("[API] ListAvatars failed: DB error getting avatars err=%v", err)
		http.Error(w, "Failed to get avatars", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	response := make([]AvatarResponse, len(avatars))
	for i, avatar := range avatars {
		response[i] = AvatarResponse{
			ID:                avatar.ID,
			Name:              avatar.Name,
			Prompt:            avatar.Prompt,
			OpenAIAssistantID: avatar.OpenAIAssistantID,
			CreatedAt:         avatar.CreatedAt.Format(time.RFC3339),
		}
	}

	log.Printf("[API] ListAvatars completed conversation_id=%d count=%d", conversationID, len(response))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
