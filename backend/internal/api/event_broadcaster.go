package api

import (
	"encoding/json"
	"log"
	"sync"
)

// Event はServer-Sent Eventを表す
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// EventBroadcaster はSSEクライアントを管理し、イベントをブロードキャストする
type EventBroadcaster struct {
	mu      sync.RWMutex
	clients map[int64]map[chan Event]struct{} // conversationID -> clients
}

// NewEventBroadcaster は新しいイベントブロードキャスターを作成する
func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{
		clients: make(map[int64]map[chan Event]struct{}),
	}
}

// Subscribe は会話のイベントを受信するクライアントを追加する
func (b *EventBroadcaster) Subscribe(conversationID int64) chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, 10) // バッファ付きチャネル

	if b.clients[conversationID] == nil {
		b.clients[conversationID] = make(map[chan Event]struct{})
	}
	b.clients[conversationID][ch] = struct{}{}

	log.Printf("[SSE] Client subscribed conversation_id=%d total_clients=%d",
		conversationID, len(b.clients[conversationID]))

	return ch
}

// Unsubscribe はクライアントのイベント受信を解除する
func (b *EventBroadcaster) Unsubscribe(conversationID int64, ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if clients, ok := b.clients[conversationID]; ok {
		delete(clients, ch)
		close(ch)
		if len(clients) == 0 {
			delete(b.clients, conversationID)
		}
	}

	log.Printf("[SSE] Client unsubscribed conversation_id=%d", conversationID)
}

// Broadcast は会話を監視しているすべてのクライアントにイベントを送信する
func (b *EventBroadcaster) Broadcast(conversationID int64, event Event) {
	b.mu.RLock()
	clients := b.clients[conversationID]
	b.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	log.Printf("[SSE] Broadcasting event type=%s conversation_id=%d clients=%d",
		event.Type, conversationID, len(clients))

	for ch := range clients {
		select {
		case ch <- event:
		default:
			// クライアントチャネルが満杯の場合、スキップ
			log.Printf("[SSE] Client channel full, skipping event")
		}
	}
}

// BroadcastMessage は新しいメッセージイベントをブロードキャストする
func (b *EventBroadcaster) BroadcastMessage(conversationID int64, message any) {
	b.Broadcast(conversationID, Event{
		Type: "message",
		Data: message,
	})
}

// BroadcastAvatarJoined はアバター参加イベントをブロードキャストする
func (b *EventBroadcaster) BroadcastAvatarJoined(conversationID int64, avatarID int64, avatarName string) {
	b.Broadcast(conversationID, Event{
		Type: "avatar_joined",
		Data: map[string]any{
			"avatar_id":   avatarID,
			"avatar_name": avatarName,
		},
	})
}

// BroadcastAvatarLeft はアバター退室イベントをブロードキャストする
func (b *EventBroadcaster) BroadcastAvatarLeft(conversationID int64, avatarID int64) {
	b.Broadcast(conversationID, Event{
		Type: "avatar_left",
		Data: map[string]any{
			"avatar_id": avatarID,
		},
	})
}

// ClientCount は会話に購読しているクライアント数を返す
func (b *EventBroadcaster) ClientCount(conversationID int64) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients[conversationID])
}

// TotalClientCount は全会話の合計クライアント数を返す
func (b *EventBroadcaster) TotalClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	total := 0
	for _, clients := range b.clients {
		total += len(clients)
	}
	return total
}

// FormatSSE はイベントをSSE形式にフォーマットする
func FormatSSE(event Event) ([]byte, error) {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return nil, err
	}
	return []byte("event: " + event.Type + "\ndata: " + string(data) + "\n\n"), nil
}
