package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

type Config struct {
	Environment          string
	HTTPAddr             string
	PublicBaseDomain     string
	DatabaseURL          string
	LogLevel             slog.Level
	CORSAllowedOrigins   []string
	Auth                 AuthConfig
	TunnelRequestTimeout time.Duration
}

type AuthConfig struct {
	AccessSecret    string
	RefreshSecret   string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:      value("APP_ENV", "development"),
		HTTPAddr:         value("HTTP_ADDR", ":8080"),
		PublicBaseDomain: value("PUBLIC_BASE_DOMAIN", "tunnel.example.test"),
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		Auth: AuthConfig{
			AccessSecret:  value("JWT_ACCESS_SECRET", "development-access-secret-change-me"),
			RefreshSecret: value("JWT_REFRESH_SECRET", "development-refresh-secret-change-me"),
		},
	}
	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
	}
	if cfg.PublicBaseDomain == "" {
		return Config{}, fmt.Errorf("PUBLIC_BASE_DOMAIN must not be empty")
	}
	level, err := parseLogLevel(value("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level
	cfg.CORSAllowedOrigins = commaSeparated(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(cfg.CORSAllowedOrigins) == 0 && cfg.Environment == "development" {
		cfg.CORSAllowedOrigins = []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	}
	if cfg.Auth.AccessTokenTTL, err = time.ParseDuration(value("ACCESS_TOKEN_TTL", "15m")); err != nil || cfg.Auth.AccessTokenTTL <= 0 {
		return Config{}, fmt.Errorf("invalid ACCESS_TOKEN_TTL")
	}
	if cfg.Auth.RefreshTokenTTL, err = time.ParseDuration(value("REFRESH_TOKEN_TTL", "720h")); err != nil || cfg.Auth.RefreshTokenTTL <= 0 {
		return Config{}, fmt.Errorf("invalid REFRESH_TOKEN_TTL")
	}
	if cfg.TunnelRequestTimeout, err = time.ParseDuration(value("TUNNEL_REQUEST_TIMEOUT", "30s")); err != nil || cfg.TunnelRequestTimeout <= 0 {
		return Config{}, fmt.Errorf("invalid TUNNEL_REQUEST_TIMEOUT")
	}
	return cfg, nil
}

func commaSeparated(raw string) []string {
	items := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func value(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseLogLevel(raw string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: %w", raw, err)
	}
	return level, nil
}
