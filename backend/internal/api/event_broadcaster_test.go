package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEventBroadcaster(t *testing.T) {
	b := NewEventBroadcaster()
	if b == nil {
		t.Fatal("NewEventBroadcaster returned nil")
	}
	if b.clients == nil {
		t.Fatal("clients map is nil")
	}
}

func TestEventBroadcaster_Subscribe(t *testing.T) {
	b := NewEventBroadcaster()
	conversationID := int64(1)

	ch := b.Subscribe(conversationID)
	if ch == nil {
		t.Fatal("Subscribe returned nil channel")
	}

	if b.ClientCount(conversationID) != 1 {
		t.Errorf("Expected 1 client, got %d", b.ClientCount(conversationID))
	}

	if b.TotalClientCount() != 1 {
		t.Errorf("Expected 1 total client, got %d", b.TotalClientCount())
	}
}

func TestEventBroadcaster_MultipleSubscribers(t *testing.T) {
	b := NewEventBroadcaster()
	conversationID := int64(1)

	ch1 := b.Subscribe(conversationID)
	ch2 := b.Subscribe(conversationID)
	ch3 := b.Subscribe(int64(2))

	if b.ClientCount(conversationID) != 2 {
		t.Errorf("Expected 2 clients for conversation 1, got %d", b.ClientCount(conversationID))
	}

	if b.ClientCount(2) != 1 {
		t.Errorf("Expected 1 client for conversation 2, got %d", b.ClientCount(2))
	}

	if b.TotalClientCount() != 3 {
		t.Errorf("Expected 3 total clients, got %d", b.TotalClientCount())
	}

	// Clean up
	b.Unsubscribe(conversationID, ch1)
	b.Unsubscribe(conversationID, ch2)
	b.Unsubscribe(2, ch3)
}

func TestEventBroadcaster_Unsubscribe(t *testing.T) {
	b := NewEventBroadcaster()
	conversationID := int64(1)

	ch := b.Subscribe(conversationID)
	b.Unsubscribe(conversationID, ch)

	if b.ClientCount(conversationID) != 0 {
		t.Errorf("Expected 0 clients after unsubscribe, got %d", b.ClientCount(conversationID))
	}
}

func TestEventBroadcaster_Broadcast(t *testing.T) {
	b := NewEventBroadcaster()
	conversationID := int64(1)

	ch := b.Subscribe(conversationID)

	// Broadcast an event
	go func() {
		b.Broadcast(conversationID, Event{
			Type: "test",
			Data: map[string]string{"key": "value"},
		})
	}()

	// Receive the event
	select {
	case event := <-ch:
		if event.Type != "test" {
			t.Errorf("Expected event type 'test', got '%s'", event.Type)
		}
		data, ok := event.Data.(map[string]string)
		if !ok {
			t.Fatal("Event data is not map[string]string")
		}
		if data["key"] != "value" {
			t.Errorf("Expected data['key'] = 'value', got '%s'", data["key"])
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}

	b.Unsubscribe(conversationID, ch)
}

func TestEventBroadcaster_BroadcastToWrongConversation(t *testing.T) {
	b := NewEventBroadcaster()
	conversationID1 := int64(1)
	conversationID2 := int64(2)

	ch := b.Subscribe(conversationID1)

	// Broadcast to different conversation
	b.Broadcast(conversationID2, Event{
		Type: "test",
		Data: "should not receive",
	})

	// Should not receive anything
	select {
	case <-ch:
		t.Fatal("Should not receive event for different conversation")
	case <-time.After(100 * time.Millisecond):
		// Expected - no event received
	}

	b.Unsubscribe(conversationID1, ch)
}

func TestEventBroadcaster_BroadcastMessage(t *testing.T) {
	b := NewEventBroadcaster()
	conversationID := int64(1)

	ch := b.Subscribe(conversationID)

	// Broadcast a message
	go func() {
		b.BroadcastMessage(conversationID, map[string]any{
			"id":      1,
			"content": "Hello",
		})
	}()

	// Receive the event
	select {
	case event := <-ch:
		if event.Type != "message" {
			t.Errorf("Expected event type 'message', got '%s'", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message event")
	}

	b.Unsubscribe(conversationID, ch)
}

func TestEventBroadcaster_BroadcastAvatarJoined(t *testing.T) {
	b := NewEventBroadcaster()
	conversationID := int64(1)

	ch := b.Subscribe(conversationID)

	// Broadcast avatar joined
	go func() {
		b.BroadcastAvatarJoined(conversationID, 10, "TestAvatar")
	}()

	// Receive the event
	select {
	case event := <-ch:
		if event.Type != "avatar_joined" {
			t.Errorf("Expected event type 'avatar_joined', got '%s'", event.Type)
		}
		data, ok := event.Data.(map[string]any)
		if !ok {
			t.Fatal("Event data is not map[string]any")
		}
		if data["avatar_name"] != "TestAvatar" {
			t.Errorf("Expected avatar_name 'TestAvatar', got '%v'", data["avatar_name"])
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for avatar_joined event")
	}

	b.Unsubscribe(conversationID, ch)
}

func TestEventBroadcaster_BroadcastAvatarLeft(t *testing.T) {
	b := NewEventBroadcaster()
	conversationID := int64(1)

	ch := b.Subscribe(conversationID)

	// Broadcast avatar left
	go func() {
		b.BroadcastAvatarLeft(conversationID, 10)
	}()

	// Receive the event
	select {
	case event := <-ch:
		if event.Type != "avatar_left" {
			t.Errorf("Expected event type 'avatar_left', got '%s'", event.Type)
		}
		data, ok := event.Data.(map[string]any)
		if !ok {
			t.Fatal("Event data is not map[string]any")
		}
		if data["avatar_id"] != int64(10) {
			t.Errorf("Expected avatar_id 10, got '%v'", data["avatar_id"])
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for avatar_left event")
	}

	b.Unsubscribe(conversationID, ch)
}

func TestFormatSSE(t *testing.T) {
	event := Event{
		Type: "message",
		Data: map[string]string{"content": "Hello"},
	}

	data, err := FormatSSE(event)
	if err != nil {
		t.Fatalf("FormatSSE returned error: %v", err)
	}

	expected := "event: message\ndata: "
	if len(data) < len(expected) {
		t.Fatalf("FormatSSE output too short")
	}

	if string(data[:len(expected)]) != expected {
		t.Errorf("Expected prefix '%s', got '%s'", expected, string(data[:len(expected)]))
	}

	// Verify the JSON data part
	jsonStart := len("event: message\ndata: ")
	jsonEnd := len(data) - 2 // Remove trailing \n\n
	var parsed map[string]string
	if err := json.Unmarshal(data[jsonStart:jsonEnd], &parsed); err != nil {
		t.Fatalf("Failed to parse JSON data: %v", err)
	}
	if parsed["content"] != "Hello" {
		t.Errorf("Expected content 'Hello', got '%s'", parsed["content"])
	}
}

