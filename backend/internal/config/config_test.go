package config

import (
	"os"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	t.Parallel()
	if _, err := parseLogLevel("not-a-level"); err == nil {
		t.Fatal("expected an error for invalid log level")
	}
	if _, err := parseLogLevel("debug"); err != nil {
		t.Fatalf("expected debug to be valid: %v", err)
	}
}

func TestLoadUsesDevelopmentDefaultsAndParsesCORS(t *testing.T) {
	for _, key := range []string{"APP_ENV", "HTTP_ADDR", "PUBLIC_BASE_DOMAIN", "DATABASE_URL", "LOG_LEVEL", "CORS_ALLOWED_ORIGINS", "ACCESS_TOKEN_TTL", "REFRESH_TOKEN_TTL", "TUNNEL_REQUEST_TIMEOUT"} {
		t.Setenv(key, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Environment != "development" || cfg.HTTPAddr != ":8080" || len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("config=%#v", cfg)
	}
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.test, https://admin.example.test")
	cfg, err = Load()
	if err != nil || len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[1] != "https://admin.example.test" {
		t.Fatalf("config=%#v err=%v", cfg, err)
	}
}

func TestLoadRejectsInvalidCriticalValues(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ACCESS_TOKEN_TTL", "invalid")
	if _, err := Load(); err == nil {
		t.Fatal("invalid access TTL was accepted")
	}
	t.Setenv("ACCESS_TOKEN_TTL", "15m")
	t.Setenv("TUNNEL_REQUEST_TIMEOUT", "0s")
	if _, err := Load(); err == nil {
		t.Fatal("zero request timeout was accepted")
	}
	_ = os.Getenv("unused")
}
