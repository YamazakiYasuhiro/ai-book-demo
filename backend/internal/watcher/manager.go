package watcher

import (
	"context"
	"log"
	"sync"
	"time"

	"multi-avatar-chat/internal/assistant"
	"multi-avatar-chat/internal/db"
	"multi-avatar-chat/internal/models"
)

// MessageBroadcaster defines the interface for broadcasting messages
type MessageBroadcaster interface {
	BroadcastMessage(conversationID int64, message any)
}

// WatcherManager manages avatar watcher goroutines
type WatcherManager struct {
	db                *db.DB
	assistant         *assistant.Client
	broadcaster       MessageBroadcaster
	watchers          map[watcherKey]*AvatarWatcher
	mu                sync.RWMutex
	interval          time.Duration
	useRandomInterval bool
	ctx               context.Context
	cancel            context.CancelFunc
}

type watcherKey struct {
	ConversationID int64
	AvatarID       int64
}

// NewManager creates a new WatcherManager
// If interval is 0, uses random intervals (5-20 seconds) for more natural responses
// Otherwise, uses the specified fixed interval (useful for testing)
func NewManager(database *db.DB, assistantClient *assistant.Client, interval time.Duration) *WatcherManager {
	ctx, cancel := context.WithCancel(context.Background())

	// If interval is 0, use random interval mode
	useRandom := interval == 0

	return &WatcherManager{
		db:                database,
		assistant:         assistantClient,
		watchers:          make(map[watcherKey]*AvatarWatcher),
		interval:          interval,
		useRandomInterval: useRandom,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// SetBroadcaster sets the message broadcaster for SSE notifications
func (m *WatcherManager) SetBroadcaster(broadcaster MessageBroadcaster) {
	m.broadcaster = broadcaster
}

// StartWatcher starts a new watcher for the given conversation and avatar
func (m *WatcherManager) StartWatcher(conversationID, avatarID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := watcherKey{ConversationID: conversationID, AvatarID: avatarID}

	// Check if already running
	if _, exists := m.watchers[key]; exists {
		log.Printf("[WatcherManager] Watcher already exists conversation_id=%d avatar_id=%d", conversationID, avatarID)
		return nil
	}

	// Get avatar info from DB
	avatar, err := m.db.GetAvatar(avatarID)
	if err != nil {
		log.Printf("[WatcherManager] Failed to get avatar avatar_id=%d err=%v", avatarID, err)
		return err
	}

	// Get conversation info for context
	conv, err := m.db.GetConversation(conversationID)
	if err != nil {
		log.Printf("[WatcherManager] Failed to get conversation conversation_id=%d err=%v", conversationID, err)
		return err
	}

	// Get all avatars in the conversation for participant list
	conversationAvatars, err := m.db.GetConversationAvatars(conversationID)
	if err != nil {
		log.Printf("[WatcherManager] Failed to get conversation avatars conversation_id=%d err=%v", conversationID, err)
		return err
	}

	// Build participant names list (User + all avatars)
	participantNames := []string{"ユーザ"}
	for _, a := range conversationAvatars {
		participantNames = append(participantNames, a.Name)
	}

	// Create and start watcher with broadcast callback
	var broadcastFn func(conversationID int64, msg *models.Message, senderName string)
	if m.broadcaster != nil {
		broadcastFn = func(convID int64, msg *models.Message, senderName string) {
			// Create a response object similar to MessageResponse in API
			msgData := map[string]any{
				"id":          msg.ID,
				"sender_type": string(msg.SenderType),
				"content":     msg.Content,
				"created_at":  msg.CreatedAt.Format(time.RFC3339),
			}
			if msg.SenderID != nil {
				msgData["sender_id"] = *msg.SenderID
			}
			if senderName != "" {
				msgData["sender_name"] = senderName
			}
			m.broadcaster.BroadcastMessage(convID, msgData)
		}
	}

	// Pass interval to watcher (0 means use random interval)
	watcher := NewAvatarWatcher(m.ctx, conversationID, *avatar, m.db, m.assistant, m.interval, broadcastFn)

	// Set conversation context for improved prompts
	watcher.SetConversationContext(conv.Title, participantNames)

	watcher.Start()

	m.watchers[key] = watcher
	log.Printf("[WatcherManager] Watcher started conversation_id=%d avatar_id=%d avatar_name=%s",
		conversationID, avatarID, avatar.Name)

	return nil
}

// StopWatcher stops the watcher for the given conversation and avatar
func (m *WatcherManager) StopWatcher(conversationID, avatarID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := watcherKey{ConversationID: conversationID, AvatarID: avatarID}

	watcher, exists := m.watchers[key]
	if !exists {
		log.Printf("[WatcherManager] Watcher not found conversation_id=%d avatar_id=%d", conversationID, avatarID)
		return nil
	}

	watcher.Stop()
	delete(m.watchers, key)
	log.Printf("[WatcherManager] Watcher stopped conversation_id=%d avatar_id=%d", conversationID, avatarID)

	return nil
}

// StopRoomWatchers stops all watchers for a conversation
func (m *WatcherManager) StopRoomWatchers(conversationID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stoppedCount := 0
	for key, watcher := range m.watchers {
		if key.ConversationID == conversationID {
			watcher.Stop()
			delete(m.watchers, key)
			log.Printf("[WatcherManager] Watcher stopped conversation_id=%d avatar_id=%d",
				key.ConversationID, key.AvatarID)
			stoppedCount++
		}
	}

	log.Printf("[WatcherManager] StopRoomWatchers completed conversation_id=%d stopped_count=%d",
		conversationID, stoppedCount)
	return nil
}

// InterruptRoomWatchers interrupts all watchers for a conversation
// This cancels any active LLM runs and stops the watchers
func (m *WatcherManager) InterruptRoomWatchers(conversationID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	interruptedCount := 0
	for key, watcher := range m.watchers {
		if key.ConversationID == conversationID {
			watcher.Interrupt()
			delete(m.watchers, key)
			log.Printf("[WatcherManager] Watcher interrupted conversation_id=%d avatar_id=%d",
				key.ConversationID, key.AvatarID)
			interruptedCount++
		}
	}

	log.Printf("[WatcherManager] InterruptRoomWatchers completed conversation_id=%d interrupted_count=%d",
		conversationID, interruptedCount)
	return nil
}

// InitializeAll starts watchers for all existing conversation-avatar pairs
func (m *WatcherManager) InitializeAll(ctx context.Context) error {
	pairs, err := m.db.GetAllConversationAvatars()
	if err != nil {
		log.Printf("[WatcherManager] Failed to get conversation avatars err=%v", err)
		return err
	}

	log.Printf("[WatcherManager] Initializing %d watchers", len(pairs))

	for _, pair := range pairs {
		if err := m.StartWatcher(pair.ConversationID, pair.AvatarID); err != nil {
			log.Printf("[WatcherManager] Failed to start watcher conversation_id=%d avatar_id=%d err=%v",
				pair.ConversationID, pair.AvatarID, err)
			// Continue with other watchers even if one fails
		}
	}

	log.Printf("[WatcherManager] Initialization completed active_watchers=%d", m.WatcherCount())
	return nil
}

// Shutdown stops all watchers gracefully
func (m *WatcherManager) Shutdown() error {
	log.Printf("[WatcherManager] Shutting down...")
	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	for key, watcher := range m.watchers {
		watcher.Stop()
		log.Printf("[WatcherManager] Watcher stopped conversation_id=%d avatar_id=%d",
			key.ConversationID, key.AvatarID)
	}

	watcherCount := len(m.watchers)
	m.watchers = make(map[watcherKey]*AvatarWatcher)

	log.Printf("[WatcherManager] Shutdown complete stopped_count=%d", watcherCount)
	return nil
}

// WatcherCount returns the number of active watchers
func (m *WatcherManager) WatcherCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.watchers)
}

// HasWatcher checks if a watcher exists for the given conversation and avatar
func (m *WatcherManager) HasWatcher(conversationID, avatarID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := watcherKey{ConversationID: conversationID, AvatarID: avatarID}
	_, exists := m.watchers[key]
	return exists
}
