package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"multi-avatar-chat/internal/api"
	"multi-avatar-chat/internal/assistant"
	"multi-avatar-chat/internal/config"
	"multi-avatar-chat/internal/db"
	"multi-avatar-chat/internal/watcher"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: Failed to load config: %v (continuing without OpenAI)", err)
		cfg = &config.Config{
			DBPath:    getEnvOrDefault("DB_PATH", "data/app.db"),
			StaticDir: getEnvOrDefault("STATIC_DIR", "static"),
		}
	}

	// Ensure data directory exists
	dbDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize database
	database, err := db.NewDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.Migrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migrated successfully")

	// Initialize OpenAI client (optional)
	var assistantClient *assistant.Client
	if cfg.OpenAI.APIKey != "" {
		assistantClient = assistant.NewClient(cfg.OpenAI.APIKey)
		log.Println("OpenAI client initialized")
	} else {
		log.Println("Warning: OpenAI API key not configured, assistant features disabled")
	}

	// Initialize WatcherManager
	// Default: 0 means random interval (5-20 seconds) for natural responses
	// Set WATCHER_INTERVAL environment variable for fixed interval (e.g., "10s" for testing)
	var watcherInterval time.Duration
	if intervalStr := os.Getenv("WATCHER_INTERVAL"); intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil {
			watcherInterval = d
		}
	}
	watcherManager := watcher.NewManager(database, assistantClient, watcherInterval)
	if watcherInterval == 0 {
		log.Printf("WatcherManager initialized with random interval (5-20 seconds)")
	} else {
		log.Printf("WatcherManager initialized with fixed interval=%v", watcherInterval)
	}

	// Create router (これによりbroadcasterがWatcherManagerに設定される)
	router := api.NewRouter(database, assistantClient, cfg.StaticDir, watcherManager)

	// Initialize all watchers for existing conversations
	// 注意: NewRouterの後に呼ぶことで、broadcasterが設定された状態でウォッチャーが作成される
	ctx := context.Background()
	if err := watcherManager.InitializeAll(ctx); err != nil {
		log.Printf("Warning: Failed to initialize watchers: %v", err)
	}
	log.Printf("Watchers initialized: count=%d", watcherManager.WatcherCount())

	// Setup server
	port := getEnvOrDefault("PORT", "8080")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Handle graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")

		// Shutdown watchers first
		if err := watcherManager.Shutdown(); err != nil {
			log.Printf("Error shutting down watchers: %v", err)
		}

		// Shutdown HTTP server with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}

		close(done)
	}()

	log.Printf("Server starting on port %s", port)
	log.Printf("Static files served from: %s", cfg.StaticDir)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}

	<-done
	log.Println("Server stopped gracefully")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
