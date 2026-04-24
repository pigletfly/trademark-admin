package config_test

import (
	"testing"
	"time"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
)

func setValidBaseEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://a:b@localhost:5432/c?sslmode=disable")
	t.Setenv("JWT_ACCESS_SECRET", "dev-access")
	t.Setenv("JWT_REFRESH_SECRET", "dev-refresh")
}

func TestLoad_defaults(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("HTTP_LISTEN_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("JWT_ACCESS_TTL", "")
	t.Setenv("JWT_REFRESH_TTL", "")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")

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
	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Errorf("JWTAccessTTL = %v, want 15m", cfg.JWTAccessTTL)
	}
	if cfg.JWTRefreshTTL != 168*time.Hour {
		t.Errorf("JWTRefreshTTL = %v, want 168h", cfg.JWTRefreshTTL)
	}
	if cfg.CookieSecure {
		t.Errorf("CookieSecure should default to false")
	}
	if cfg.BootstrapAdminEmail != "" {
		t.Errorf("BootstrapAdminEmail = %q, want empty", cfg.BootstrapAdminEmail)
	}
	if cfg.BootstrapAdminPassword != "" {
		t.Errorf("BootstrapAdminPassword = %q, want empty", cfg.BootstrapAdminPassword)
	}
}

func TestLoad_missingDatabaseURL(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("DATABASE_URL", "")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoad_missingJWTAccessSecret(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("JWT_ACCESS_SECRET", "")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoad_missingJWTRefreshSecret(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("JWT_REFRESH_SECRET", "")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoad_invalidDuration(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("JWT_ACCESS_TTL", "not-a-duration")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}
