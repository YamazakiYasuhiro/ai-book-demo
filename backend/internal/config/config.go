package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// OpenAIConfig holds OpenAI API configuration
type OpenAIConfig struct {
	APIKey string `yaml:"api_key"`
}

// Config holds all application configuration
type Config struct {
	OpenAI      OpenAIConfig
	DBPath      string
	StaticDir   string
	SettingsDir string
}

// Load loads configuration from environment and files
func Load() (*Config, error) {
	settingsDir := os.Getenv("SETTINGS_DIR")
	if settingsDir == "" {
		settingsDir = "settings"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/app.db"
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}

	cfg := &Config{
		DBPath:      dbPath,
		StaticDir:   staticDir,
		SettingsDir: settingsDir,
	}

	// Load OpenAI config
	openaiCfg, err := loadOpenAIConfig(filepath.Join(settingsDir, "secrets", "openai.yaml"))
	if err != nil {
		return nil, err
	}
	cfg.OpenAI = *openaiCfg

	return cfg, nil
}

// loadOpenAIConfig loads OpenAI configuration from a YAML file
func loadOpenAIConfig(path string) (*OpenAIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg OpenAIConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
