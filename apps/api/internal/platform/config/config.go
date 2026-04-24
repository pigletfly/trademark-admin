package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL    string
	HTTPListenAddr string
	LogLevel       string
	AppEnv         string
}

// Load reads configuration from environment variables and applies defaults.
// DATABASE_URL is required.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		HTTPListenAddr: getenvDefault("HTTP_LISTEN_ADDR", ":8080"),
		LogLevel:       getenvDefault("LOG_LEVEL", "info"),
		AppEnv:         getenvDefault("APP_ENV", "development"),
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
