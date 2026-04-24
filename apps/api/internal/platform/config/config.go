package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL    string
	HTTPListenAddr string
	LogLevel       string
	AppEnv         string

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
	CookieSecure     bool

	BootstrapAdminEmail    string
	BootstrapAdminPassword string
}

// Load reads configuration from environment variables and applies defaults.
// DATABASE_URL, JWT_ACCESS_SECRET, and JWT_REFRESH_SECRET are required.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		HTTPListenAddr:   getenvDefault("HTTP_LISTEN_ADDR", ":8080"),
		LogLevel:         getenvDefault("LOG_LEVEL", "info"),
		AppEnv:           getenvDefault("APP_ENV", "development"),
		JWTAccessSecret:  os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if cfg.JWTAccessSecret == "" {
		return nil, errors.New("JWT_ACCESS_SECRET is required")
	}
	if cfg.JWTRefreshSecret == "" {
		return nil, errors.New("JWT_REFRESH_SECRET is required")
	}

	accessTTL, err := parseDuration("JWT_ACCESS_TTL", "15m")
	if err != nil {
		return nil, err
	}
	refreshTTL, err := parseDuration("JWT_REFRESH_TTL", "168h")
	if err != nil {
		return nil, err
	}
	cfg.JWTAccessTTL = accessTTL
	cfg.JWTRefreshTTL = refreshTTL

	secure, _ := strconv.ParseBool(getenvDefault("COOKIE_SECURE", "false"))
	cfg.CookieSecure = secure

	cfg.BootstrapAdminEmail = os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
	cfg.BootstrapAdminPassword = os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")

	return cfg, nil
}

func parseDuration(key, fallback string) (time.Duration, error) {
	val := getenvDefault(key, fallback)
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid duration (%s): %w", key, val, err)
	}
	return d, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
