package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConversationEventsHandler_HandleEvents_InvalidID(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	handler := NewConversationEventsHandler(broadcaster)

	// Create request with invalid ID
	req := httptest.NewRequest("GET", "/api/conversations/invalid/events", nil)
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	handler.HandleEvents(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestConversationEventsHandler_SSEHeaders(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	handler := NewConversationEventsHandler(broadcaster)

	// Create a context that can be cancelled
	req := httptest.NewRequest("GET", "/api/conversations/1/events", nil)
	req.SetPathValue("id", "1")

	// Use a channel to signal when headers are written
	done := make(chan bool)

	// Create a custom response writer that checks headers
	rr := &testResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		onWriteHeader: func(code int) {
			// Headers should be set before this
		},
		onWrite: func(data []byte) {
			// Check headers on first write
			select {
			case done <- true:
			default:
			}
		},
	}

	// Run handler in goroutine and cancel quickly
	go func() {
		handler.HandleEvents(rr, req)
	}()

	// Wait for headers to be set
	<-done

	// Check headers
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", ct)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Expected Cache-Control 'no-cache', got '%s'", cc)
	}
	if conn := rr.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("Expected Connection 'keep-alive', got '%s'", conn)
	}
}

// testResponseWriter wraps ResponseRecorder for testing
type testResponseWriter struct {
	*httptest.ResponseRecorder
	onWriteHeader func(int)
	onWrite       func([]byte)
}

func (w *testResponseWriter) WriteHeader(code int) {
	if w.onWriteHeader != nil {
		w.onWriteHeader(code)
	}
	w.ResponseRecorder.WriteHeader(code)
}

func (w *testResponseWriter) Write(data []byte) (int, error) {
	if w.onWrite != nil {
		w.onWrite(data)
	}
	return w.ResponseRecorder.Write(data)
}

func (w *testResponseWriter) Flush() {
	// ResponseRecorder implements Flusher
	if f, ok := w.ResponseRecorder.Result().Body.(http.Flusher); ok {
		f.Flush()
	}
}

