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
	"multi-avatar-chat/internal/logic"
	"multi-avatar-chat/internal/models"
	"multi-avatar-chat/internal/watcher"
)

// ConversationHandler handles conversation-related HTTP requests
type ConversationHandler struct {
	db        *db.DB
	assistant *assistant.Client
	watcher   *watcher.WatcherManager
}

// NewConversationHandler creates a new conversation handler
func NewConversationHandler(database *db.DB, assistantClient *assistant.Client) *ConversationHandler {
	return &ConversationHandler{
		db:        database,
		assistant: assistantClient,
	}
}

// SetWatcherManager sets the watcher manager for the handler
func (h *ConversationHandler) SetWatcherManager(wm *watcher.WatcherManager) {
	h.watcher = wm
}

// CreateConversationRequest represents the request body for creating a conversation
type CreateConversationRequest struct {
	Title     string  `json:"title"`
	AvatarIDs []int64 `json:"avatar_ids,omitempty"`
}

// ConversationResponse represents a conversation in API responses
type ConversationResponse struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	ThreadID  string `json:"thread_id,omitempty"`
	CreatedAt string `json:"created_at"`
}

// Create handles POST /api/conversations
func (h *ConversationHandler) Create(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] Create conversation started")

	var req CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[API] Create conversation failed: invalid request body err=%v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("[API] Create conversation request title=%q avatar_ids=%v", req.Title, req.AvatarIDs)

	if req.Title == "" {
		log.Printf("[API] Create conversation failed: title is required")
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	// Save to database (no thread_id for conversation itself)
	conv, err := h.db.CreateConversation(req.Title, "")
	if err != nil {
		log.Printf("[API] Failed to create conversation in DB err=%v", err)
		http.Error(w, "Failed to create conversation", http.StatusInternalServerError)
		return
	}
	log.Printf("[API] Conversation created in DB conversation_id=%d", conv.ID)

	// Add avatars to conversation and create threads for each avatar
	for _, avatarID := range req.AvatarIDs {
		var threadID string
		if h.assistant != nil {
			log.Printf("[API] Creating OpenAI thread for avatar conversation_id=%d avatar_id=%d", conv.ID, avatarID)
			thread, err := h.assistant.CreateThread()
			if err != nil {
				log.Printf("[API] Failed to create OpenAI thread for avatar conversation_id=%d avatar_id=%d err=%v", conv.ID, avatarID, err)
				// Continue even if thread creation fails, but log the error
				// Add avatar without thread_id
				if err := h.db.AddAvatarToConversationWithThreadID(conv.ID, avatarID, ""); err != nil {
					log.Printf("[API] Failed to add avatar to conversation conversation_id=%d avatar_id=%d err=%v", conv.ID, avatarID, err)
				}
				continue
			}
			threadID = thread.ID
			log.Printf("[API] OpenAI thread created for avatar conversation_id=%d avatar_id=%d thread_id=%s", conv.ID, avatarID, threadID)
		} else {
			log.Printf("[API] OpenAI assistant client is nil, skipping thread creation for avatar_id=%d", avatarID)
		}

		// Add avatar to conversation with thread ID
		if err := h.db.AddAvatarToConversationWithThreadID(conv.ID, avatarID, threadID); err != nil {
			log.Printf("[API] Failed to add avatar to conversation conversation_id=%d avatar_id=%d err=%v", conv.ID, avatarID, err)
			// Continue even if one fails
		} else {
			log.Printf("[API] Avatar added to conversation conversation_id=%d avatar_id=%d thread_id=%s", conv.ID, avatarID, threadID)
			// Start watcher for the avatar
			if h.watcher != nil {
				if err := h.watcher.StartWatcher(conv.ID, avatarID); err != nil {
					log.Printf("[API] Warning: Failed to start watcher conversation_id=%d avatar_id=%d err=%v", conv.ID, avatarID, err)
				}
			}
		}
	}

	log.Printf("[API] Create conversation completed conversation_id=%d title=%q", conv.ID, conv.Title)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ConversationResponse{
		ID:        conv.ID,
		Title:     conv.Title,
		ThreadID:  conv.ThreadID,
		CreatedAt: conv.CreatedAt.Format(time.RFC3339),
	})
}

// List handles GET /api/conversations
func (h *ConversationHandler) List(w http.ResponseWriter, r *http.Request) {
	conversations, err := h.db.GetAllConversations()
	if err != nil {
		http.Error(w, "Failed to get conversations", http.StatusInternalServerError)
		return
	}

	response := make([]ConversationResponse, len(conversations))
	for i, conv := range conversations {
		response[i] = ConversationResponse{
			ID:        conv.ID,
			Title:     conv.Title,
			ThreadID:  conv.ThreadID,
			CreatedAt: conv.CreatedAt.Format(time.RFC3339),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Get handles GET /api/conversations/{id}
func (h *ConversationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	conv, err := h.db.GetConversation(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConversationResponse{
		ID:        conv.ID,
		Title:     conv.Title,
		ThreadID:  conv.ThreadID,
		CreatedAt: conv.CreatedAt.Format(time.RFC3339),
	})
}

// Delete handles DELETE /api/conversations/{id}
func (h *ConversationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] Delete conversation started")

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Printf("[API] Delete conversation failed: invalid conversation ID err=%v", err)
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	log.Printf("[API] Delete conversation request conversation_id=%d", id)

	// Get existing conversation to get thread ID
	existing, err := h.db.GetConversation(id)
	if err == sql.ErrNoRows {
		log.Printf("[API] Delete conversation failed: conversation not found conversation_id=%d", id)
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("[API] Delete conversation failed: DB error getting conversation err=%v", err)
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}

	// Stop all watchers for this conversation first
	if h.watcher != nil {
		if err := h.watcher.StopRoomWatchers(id); err != nil {
			log.Printf("[API] Warning: Failed to stop room watchers conversation_id=%d err=%v", id, err)
		}
	}

	// Delete OpenAI Thread
	if h.assistant != nil && existing.ThreadID != "" {
		// Ignore errors for thread deletion
		_ = h.assistant.DeleteThread(existing.ThreadID)
		log.Printf("[API] OpenAI thread deleted thread_id=%s", existing.ThreadID)
	}

	// Delete from database
	if err := h.db.DeleteConversation(id); err != nil {
		log.Printf("[API] Delete conversation failed: DB error deleting conversation err=%v", err)
		http.Error(w, "Failed to delete conversation", http.StatusInternalServerError)
		return
	}

	log.Printf("[API] Delete conversation completed conversation_id=%d", id)
	w.WriteHeader(http.StatusNoContent)
}

// MessageResponse represents a message in API responses
type MessageResponse struct {
	ID         int64  `json:"id"`
	SenderType string `json:"sender_type"`
	SenderID   *int64 `json:"sender_id,omitempty"`
	SenderName string `json:"sender_name,omitempty"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
}

// SendMessageRequest represents the request body for sending a message
type SendMessageRequest struct {
	Content string `json:"content"`
}

// SendMessageResponse represents the response for sending a message
type SendMessageResponse struct {
	UserMessage     MessageResponse   `json:"user_message"`
	AvatarResponses []MessageResponse `json:"avatar_responses,omitempty"`
}

// SendMessage handles POST /api/conversations/{id}/messages
func (h *ConversationHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("[API] SendMessage started")

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Printf("[API] SendMessage failed: invalid conversation ID err=%v", err)
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[API] SendMessage failed: invalid request body err=%v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Truncate content for logging
	contentPreview := req.Content
	if len(contentPreview) > 100 {
		contentPreview = contentPreview[:100] + "..."
	}
	log.Printf("[API] SendMessage request conversation_id=%d content=%q", id, contentPreview)

	if req.Content == "" {
		log.Printf("[API] SendMessage failed: content is required")
		http.Error(w, "Content is required", http.StatusBadRequest)
		return
	}

	// Verify conversation exists
	conv, err := h.db.GetConversation(id)
	if err == sql.ErrNoRows {
		log.Printf("[API] SendMessage failed: conversation not found conversation_id=%d", id)
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("[API] SendMessage failed: DB error getting conversation err=%v", err)
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}
	log.Printf("[API] Conversation found conversation_id=%d thread_id=%s", conv.ID, conv.ThreadID)

	// Get conversation avatars for debugging
	avatars, err := h.db.GetConversationAvatars(id)
	if err != nil {
		log.Printf("[API] Warning: failed to get conversation avatars err=%v", err)
	} else {
		avatarNames := make([]string, len(avatars))
		for i, a := range avatars {
			avatarNames[i] = a.Name
		}
		log.Printf("[API] Conversation avatars conversation_id=%d count=%d names=%v", id, len(avatars), avatarNames)
	}

	// Save user message to database
	msg, err := h.db.CreateMessage(id, models.SenderTypeUser, nil, req.Content)
	if err != nil {
		log.Printf("[API] SendMessage failed: DB error saving message err=%v", err)
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}
	log.Printf("[API] User message saved to DB message_id=%d conversation_id=%d", msg.ID, id)

	// Send user message to all avatar threads
	if h.assistant != nil {
		avatars, threadIDs, err := h.db.GetConversationAvatarsWithThreads(id)
		if err != nil {
			log.Printf("[API] Warning: failed to get conversation avatars with threads err=%v", err)
		} else {
			// Format user message for OpenAI Thread
			formattedContent := logic.FormatUserMessage(req.Content)

			// Send to each avatar's thread
			for i, avatar := range avatars {
				if i >= len(threadIDs) || threadIDs[i] == "" {
					log.Printf("[API] Skipping avatar without thread_id conversation_id=%d avatar_id=%d avatar_name=%s", id, avatar.ID, avatar.Name)
					continue
				}

				threadID := threadIDs[i]
				log.Printf("[API] Sending user message to avatar thread conversation_id=%d avatar_id=%d avatar_name=%s thread_id=%s", id, avatar.ID, avatar.Name, threadID)
				log.Printf("[API] LLM Input thread_id=%s avatar_name=%s message_content=%q", threadID, avatar.Name, formattedContent)

				// Wait for any active runs to complete before adding message
				if err := h.assistant.WaitForActiveRunsToComplete(threadID, 30*time.Second); err != nil {
					log.Printf("[API] Warning: timeout waiting for active runs thread_id=%s avatar_name=%s err=%v", threadID, avatar.Name, err)
				}

				_, err := h.assistant.CreateMessage(threadID, formattedContent)
				if err != nil {
					log.Printf("[API] Warning: failed to send message to avatar thread thread_id=%s avatar_name=%s err=%v", threadID, avatar.Name, err)
					// Continue - message is saved locally
				} else {
					log.Printf("[API] Message sent to avatar thread successfully thread_id=%s avatar_name=%s", threadID, avatar.Name)
				}
			}
		}
	} else {
		log.Printf("[API] Skipping OpenAI thread: assistant is nil")
	}

	// Generate avatar responses only if WatcherManager is not active
	// When WatcherManager is active, avatars will respond asynchronously via polling
	var avatarResponses []MessageResponse
	if h.watcher == nil {
		avatarResponses = h.generateAvatarResponses(conv, avatars, req.Content)
	} else {
		log.Printf("[API] Skipping synchronous avatar response: WatcherManager is active")
	}

	log.Printf("[API] SendMessage completed conversation_id=%d message_id=%d avatar_responses=%d duration=%v",
		id, msg.ID, len(avatarResponses), time.Since(start))

	// Build response
	userMessage := MessageResponse{
		ID:         msg.ID,
		SenderType: string(msg.SenderType),
		SenderID:   msg.SenderID,
		Content:    msg.Content,
		CreatedAt:  msg.CreatedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SendMessageResponse{
		UserMessage:     userMessage,
		AvatarResponses: avatarResponses,
	})
}

// generateAvatarResponses generates responses from avatars
// Returns a slice of messages created by avatars
func (h *ConversationHandler) generateAvatarResponses(
	conv *models.Conversation,
	avatars []models.Avatar,
	userContent string,
) []MessageResponse {
	if h.assistant == nil || conv.ThreadID == "" {
		log.Printf("[API] Skipping avatar response: assistant not configured")
		return nil
	}

	if len(avatars) == 0 {
		log.Printf("[API] Skipping avatar response: no avatars in conversation")
		return nil
	}

	// Select which avatars should respond
	responders := logic.SelectResponders(userContent, avatars)
	log.Printf("[API] Selected responders count=%d", len(responders))

	// For now, only first responder generates a response (to avoid multiple simultaneous runs)
	if len(responders) == 0 {
		return nil
	}

	responder := responders[0]
	log.Printf("[API] Generating response from avatar name=%q assistant_id=%s",
		responder.Name, responder.OpenAIAssistantID)

	// Check if avatar has OpenAI Assistant ID
	if responder.OpenAIAssistantID == "" {
		log.Printf("[API] Avatar has no OpenAI assistant ID, skipping avatar_id=%d", responder.ID)
		return nil
	}

	// Create a run for the avatar to respond
	run, err := h.assistant.CreateRun(conv.ThreadID, responder.OpenAIAssistantID)
	if err != nil {
		log.Printf("[API] Failed to create run err=%v", err)
		return nil
	}
	log.Printf("[API] Run created run_id=%s", run.ID)

	// Wait for run to complete (30 second timeout)
	completedRun, err := h.assistant.WaitForRun(conv.ThreadID, run.ID, 30*time.Second)
	if err != nil {
		log.Printf("[API] Run failed or timed out err=%v", err)
		return nil
	}
	log.Printf("[API] Run completed run_id=%s status=%s", completedRun.ID, completedRun.Status)

	// Get the latest assistant message
	responseContent, err := h.assistant.GetLatestAssistantMessage(conv.ThreadID)
	if err != nil {
		log.Printf("[API] Failed to get assistant message err=%v", err)
		return nil
	}
	log.Printf("[API] Got assistant response content_length=%d", len(responseContent))

	// Save avatar message to database
	avatarID := responder.ID
	avatarMsg, err := h.db.CreateMessage(conv.ID, models.SenderTypeAvatar, &avatarID, responseContent)
	if err != nil {
		log.Printf("[API] Failed to save avatar message err=%v", err)
		return nil
	}
	log.Printf("[API] Avatar message saved message_id=%d avatar_id=%d", avatarMsg.ID, avatarID)

	return []MessageResponse{{
		ID:         avatarMsg.ID,
		SenderType: string(avatarMsg.SenderType),
		SenderID:   avatarMsg.SenderID,
		SenderName: responder.Name,
		Content:    avatarMsg.Content,
		CreatedAt:  avatarMsg.CreatedAt.Format(time.RFC3339),
	}}
}

// GetMessages handles GET /api/conversations/{id}/messages
func (h *ConversationHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] GetMessages started")

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Printf("[API] GetMessages failed: invalid conversation ID err=%v", err)
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	log.Printf("[API] GetMessages request conversation_id=%d", id)

	// Verify conversation exists
	_, err = h.db.GetConversation(id)
	if err == sql.ErrNoRows {
		log.Printf("[API] GetMessages failed: conversation not found conversation_id=%d", id)
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("[API] GetMessages failed: DB error getting conversation err=%v", err)
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}

	messages, err := h.db.GetMessages(id)
	if err != nil {
		log.Printf("[API] GetMessages failed: DB error getting messages err=%v", err)
		http.Error(w, "Failed to get messages", http.StatusInternalServerError)
		return
	}
	log.Printf("[API] Messages retrieved conversation_id=%d count=%d", id, len(messages))

	// Get avatars for sender names
	avatars, _ := h.db.GetConversationAvatars(id)
	avatarMap := make(map[int64]string)
	for _, a := range avatars {
		avatarMap[a.ID] = a.Name
	}

	response := make([]MessageResponse, len(messages))
	for i, msg := range messages {
		resp := MessageResponse{
			ID:         msg.ID,
			SenderType: string(msg.SenderType),
			SenderID:   msg.SenderID,
			Content:    msg.Content,
			CreatedAt:  msg.CreatedAt.Format(time.RFC3339),
		}
		if msg.SenderID != nil {
			if name, ok := avatarMap[*msg.SenderID]; ok {
				resp.SenderName = name
			}
		}
		response[i] = resp
	}

	log.Printf("[API] GetMessages completed conversation_id=%d message_count=%d", id, len(response))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Interrupt handles POST /api/conversations/{id}/interrupt
func (h *ConversationHandler) Interrupt(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] Interrupt conversation started")

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Printf("[API] Interrupt conversation failed: invalid conversation ID err=%v", err)
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	log.Printf("[API] Interrupt conversation request conversation_id=%d", id)

	// Verify conversation exists
	_, err = h.db.GetConversation(id)
	if err == sql.ErrNoRows {
		log.Printf("[API] Interrupt conversation failed: conversation not found conversation_id=%d", id)
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("[API] Interrupt conversation failed: DB error getting conversation err=%v", err)
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}

	// Interrupt all watchers for this conversation
	if h.watcher != nil {
		if err := h.watcher.InterruptRoomWatchers(id); err != nil {
			log.Printf("[API] Warning: Failed to interrupt room watchers conversation_id=%d err=%v", id, err)
			http.Error(w, "Failed to interrupt watchers", http.StatusInternalServerError)
			return
		}
	} else {
		log.Printf("[API] Warning: WatcherManager is nil, cannot interrupt conversation_id=%d", id)
	}

	log.Printf("[API] Interrupt conversation completed conversation_id=%d", id)
	w.WriteHeader(http.StatusNoContent)
}
