package config

import (
	"os"
	"path/filepath"
	"strings"
)

const DefaultKeychainService = "com.job-inbox.local"

type Config struct {
	Address            string
	AppOrigin          string
	DataDir            string
	GoogleClientID     string
	GoogleClientSecret string
	OAuthRedirectURL   string
	KeychainService    string
	LocalAIEnabled     bool
	OllamaURL          string
	OllamaModel        string
	WebRoot            string
}

func Load() (Config, error) {
	if err := LoadProjectEnv(); err != nil {
		return Config{}, err
	}
	dataDir, err := os.UserConfigDir()
	if err != nil {
		return Config{}, err
	}
	if override := os.Getenv("JOB_INBOX_DATA_DIR"); override != "" {
		dataDir = override
	}
	return Config{
		Address:            "127.0.0.1:" + env("PORT", "8787"),
		AppOrigin:          env("APP_ORIGIN", "http://localhost:5173"),
		DataDir:            filepath.Join(dataDir, "Job Inbox"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		OAuthRedirectURL:   env("OAUTH_REDIRECT_URL", "http://127.0.0.1:8787/auth/callback"),
		KeychainService:    DefaultKeychainService,
		LocalAIEnabled:     strings.EqualFold(os.Getenv("LOCAL_AI_ENABLED"), "true"),
		OllamaURL:          env("OLLAMA_URL", "http://127.0.0.1:11434"),
		OllamaModel:        env("OLLAMA_MODEL", "qwen3:4b"),
		WebRoot:            "web/dist",
	}, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
