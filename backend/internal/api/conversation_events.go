package api

import (
	"log"
	"net/http"
	"strconv"
)

// ConversationEventsHandler は会話イベントのSSE接続を処理する
type ConversationEventsHandler struct {
	broadcaster *EventBroadcaster
}

// NewConversationEventsHandler は新しいハンドラーを作成する
func NewConversationEventsHandler(broadcaster *EventBroadcaster) *ConversationEventsHandler {
	return &ConversationEventsHandler{
		broadcaster: broadcaster,
	}
}

// HandleEvents は GET /api/conversations/{id}/events を処理する
func (h *ConversationEventsHandler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	conversationID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Printf("[SSE] Invalid conversation ID err=%v", err)
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	log.Printf("[SSE] New connection request conversation_id=%d", conversationID)

	// SSEヘッダーを設定
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no") // nginxバッファリングを無効化

	// flusherを取得
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("[SSE] Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// イベントを購読
	eventCh := h.broadcaster.Subscribe(conversationID)
	defer h.broadcaster.Unsubscribe(conversationID, eventCh)

	// 接続完了イベントを送信
	_, err = w.Write([]byte("event: connected\ndata: {}\n\n"))
	if err != nil {
		log.Printf("[SSE] Failed to send connected event err=%v", err)
		return
	}
	flusher.Flush()

	log.Printf("[SSE] Client connected conversation_id=%d", conversationID)

	// イベントとクライアント切断を監視
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[SSE] Client disconnected conversation_id=%d", conversationID)
			return
		case event, ok := <-eventCh:
			if !ok {
				log.Printf("[SSE] Event channel closed conversation_id=%d", conversationID)
				return
			}
			data, err := FormatSSE(event)
			if err != nil {
				log.Printf("[SSE] Failed to format event err=%v", err)
				continue
			}
			_, err = w.Write(data)
			if err != nil {
				log.Printf("[SSE] Failed to write event err=%v", err)
				return
			}
			flusher.Flush()
		}
	}
}
