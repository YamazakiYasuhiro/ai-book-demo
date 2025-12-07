package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOpenAIConfig_ValidFile(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	secretsDir := filepath.Join(tmpDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		t.Fatalf("failed to create secrets dir: %v", err)
	}

	// Create test config file
	configPath := filepath.Join(secretsDir, "openai.yaml")
	content := []byte(`api_key: "test-api-key-12345"`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := loadOpenAIConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.APIKey != "test-api-key-12345" {
		t.Errorf("expected api_key 'test-api-key-12345', got '%s'", cfg.APIKey)
	}
}

func TestLoadOpenAIConfig_FileNotFound(t *testing.T) {
	_, err := loadOpenAIConfig("/nonexistent/path/openai.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoad_WithEnvVars(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	secretsDir := filepath.Join(tmpDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		t.Fatalf("failed to create secrets dir: %v", err)
	}

	// Create test config file
	configPath := filepath.Join(secretsDir, "openai.yaml")
	content := []byte(`api_key: "env-test-key"`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Set environment variables
	os.Setenv("SETTINGS_DIR", tmpDir)
	os.Setenv("DB_PATH", "/custom/db/path.db")
	os.Setenv("STATIC_DIR", "/custom/static")
	defer func() {
		os.Unsetenv("SETTINGS_DIR")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("STATIC_DIR")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.DBPath != "/custom/db/path.db" {
		t.Errorf("expected DB_PATH '/custom/db/path.db', got '%s'", cfg.DBPath)
	}

	if cfg.StaticDir != "/custom/static" {
		t.Errorf("expected STATIC_DIR '/custom/static', got '%s'", cfg.StaticDir)
	}

	if cfg.OpenAI.APIKey != "env-test-key" {
		t.Errorf("expected OpenAI API key 'env-test-key', got '%s'", cfg.OpenAI.APIKey)
	}
}

