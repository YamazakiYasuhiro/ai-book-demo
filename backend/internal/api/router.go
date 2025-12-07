package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"multi-avatar-chat/internal/assistant"
	"multi-avatar-chat/internal/db"
	"multi-avatar-chat/internal/watcher"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher interface for SSE support
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Router holds the HTTP multiplexer and dependencies
type Router struct {
	mux                       *http.ServeMux
	avatarHandler             *AvatarHandler
	conversationHandler       *ConversationHandler
	conversationAvatarHandler *ConversationAvatarHandler
	eventsHandler             *ConversationEventsHandler
	broadcaster               *EventBroadcaster
	watcherManager            *watcher.WatcherManager
	staticDir                 string
}

// NewRouter creates a new router with all routes configured
func NewRouter(database *db.DB, assistantClient *assistant.Client, staticDir string, watcherManager *watcher.WatcherManager) *Router {
	// Create event broadcaster for SSE
	broadcaster := NewEventBroadcaster()

	// Set broadcaster on watcher manager if available
	if watcherManager != nil {
		watcherManager.SetBroadcaster(broadcaster)
	}

	convHandler := NewConversationHandler(database, assistantClient)
	convHandler.SetWatcherManager(watcherManager)

	// Create conversation avatar handler with broadcaster
	convAvatarHandler := NewConversationAvatarHandler(database, assistantClient, watcherManager)
	convAvatarHandler.SetBroadcaster(broadcaster)

	r := &Router{
		mux:                       http.NewServeMux(),
		avatarHandler:             NewAvatarHandler(database, assistantClient),
		conversationHandler:       convHandler,
		conversationAvatarHandler: convAvatarHandler,
		eventsHandler:             NewConversationEventsHandler(broadcaster),
		broadcaster:               broadcaster,
		watcherManager:            watcherManager,
		staticDir:                 staticDir,
	}
	r.setupRoutes()
	return r
}

// setupRoutes configures all HTTP routes
func (r *Router) setupRoutes() {
	// Health check
	r.mux.HandleFunc("GET /health", HealthHandler)

	// Avatar routes
	r.mux.HandleFunc("GET /api/avatars", r.avatarHandler.List)
	r.mux.HandleFunc("POST /api/avatars", r.avatarHandler.Create)
	r.mux.HandleFunc("GET /api/avatars/{id}", r.avatarHandler.Get)
	r.mux.HandleFunc("PUT /api/avatars/{id}", r.avatarHandler.Update)
	r.mux.HandleFunc("DELETE /api/avatars/{id}", r.avatarHandler.Delete)

	// Conversation routes
	r.mux.HandleFunc("GET /api/conversations", r.conversationHandler.List)
	r.mux.HandleFunc("POST /api/conversations", r.conversationHandler.Create)
	r.mux.HandleFunc("GET /api/conversations/{id}", r.conversationHandler.Get)
	r.mux.HandleFunc("DELETE /api/conversations/{id}", r.conversationHandler.Delete)

	// Message routes
	r.mux.HandleFunc("GET /api/conversations/{id}/messages", r.conversationHandler.GetMessages)
	r.mux.HandleFunc("POST /api/conversations/{id}/messages", r.conversationHandler.SendMessage)

	// Interrupt route
	r.mux.HandleFunc("POST /api/conversations/{id}/interrupt", r.conversationHandler.Interrupt)

	// Conversation avatar routes
	r.mux.HandleFunc("GET /api/conversations/{id}/avatars", r.conversationAvatarHandler.ListAvatars)
	r.mux.HandleFunc("POST /api/conversations/{id}/avatars", r.conversationAvatarHandler.AddAvatar)
	r.mux.HandleFunc("DELETE /api/conversations/{id}/avatars/{avatar_id}", r.conversationAvatarHandler.RemoveAvatar)

	// SSE events route
	r.mux.HandleFunc("GET /api/conversations/{id}/events", r.eventsHandler.HandleEvents)

	// Static file serving (for frontend)
	if r.staticDir != "" {
		r.mux.HandleFunc("GET /", r.serveStatic)
	}
}

// serveStatic serves static files from the static directory
func (r *Router) serveStatic(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	filePath := filepath.Join(r.staticDir, path)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Serve index.html for SPA routing
		filePath = filepath.Join(r.staticDir, "index.html")
	}

	http.ServeFile(w, req, filePath)
}

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()

	// Add CORS headers for development
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if req.Method == "OPTIONS" {
		log.Printf("[HTTP] CORS preflight method=OPTIONS path=%s", req.URL.Path)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Skip logging for static files, health checks, and SSE endpoints
	shouldLog := strings.HasPrefix(req.URL.Path, "/api/") && !strings.HasSuffix(req.URL.Path, "/events")

	if shouldLog {
		log.Printf("[HTTP] Request started method=%s path=%s", req.Method, req.URL.Path)
	}

	// Wrap response writer to capture status code
	wrapped := newResponseWriter(w)
	r.mux.ServeHTTP(wrapped, req)

	if shouldLog {
		log.Printf("[HTTP] Request completed method=%s path=%s status=%d duration=%v",
			req.Method, req.URL.Path, wrapped.statusCode, time.Since(start))
	}
}

// GetBroadcaster returns the event broadcaster
func (r *Router) GetBroadcaster() *EventBroadcaster {
	return r.broadcaster
}
