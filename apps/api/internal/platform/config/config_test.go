package config_test

import (
	"testing"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
)

func TestLoad_defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://a:b@localhost:5432/c?sslmode=disable")
	t.Setenv("HTTP_LISTEN_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("APP_ENV", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPListenAddr != ":8080" {
		t.Errorf("HTTPListenAddr = %q, want :8080", cfg.HTTPListenAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.DatabaseURL == "" {
		t.Errorf("DatabaseURL must not be empty")
	}
}

func TestLoad_missingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error when DATABASE_URL is empty")
	}
}
