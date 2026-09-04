package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaultTTLs(t *testing.T) {
	os.Clearenv()
	os.Setenv("DATABASE_URL", "postgres://localhost")
	os.Setenv("JWT_ACCESS_SECRET", "secret")
	os.Setenv("JWT_REFRESH_SECRET", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Auth.AccessTokenTTL == 0 || cfg.Auth.RefreshTokenTTL == 0 {
		t.Fatalf("TTLs should have defaults, got %v/%v", cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL)
	}
}

func TestLoadDefaultTunnelTimeout(t *testing.T) {
	os.Clearenv()
	os.Setenv("DATABASE_URL", "postgres://localhost")
	os.Setenv("JWT_ACCESS_SECRET", "secret")
	os.Setenv("JWT_REFRESH_SECRET", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TunnelRequestTimeout != 30*time.Second {
		t.Fatalf("default timeout should be 30s, got %v", cfg.TunnelRequestTimeout)
	}
}

func TestLoadDefaultCORSOrigins(t *testing.T) {
	os.Clearenv()
	os.Setenv("DATABASE_URL", "postgres://localhost")
	os.Setenv("JWT_ACCESS_SECRET", "secret")
	os.Setenv("JWT_REFRESH_SECRET", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		t.Fatal("CORS origins should have default value")
	}
}
