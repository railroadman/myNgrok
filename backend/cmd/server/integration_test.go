package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/myngrok/backend/internal/config"
)

func TestRunWithValidConfig(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}

	cfg := config.Config{
		Environment:          "test",
		HTTPAddr:             "127.0.0.1:0",
		PublicBaseDomain:     "test.local",
		DatabaseURL:          databaseURL,
		Auth:                 config.AuthConfig{AccessSecret: "test-secret-123", RefreshSecret: "test-refresh-456", AccessTokenTTL: 1 * time.Hour, RefreshTokenTTL: 24 * time.Hour},
		TunnelRequestTimeout: 5 * time.Second,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, logger) }()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Fatalf("run should exit cleanly on context cancellation, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run should exit quickly after context cancellation")
	}
}

func TestRunMissingDatabase(t *testing.T) {
	cfg := config.Config{
		Environment:      "test",
		HTTPAddr:         ":9999",
		PublicBaseDomain: "test.local",
		DatabaseURL:      "", // Missing
		Auth:             config.AuthConfig{AccessSecret: "s", RefreshSecret: "r", AccessTokenTTL: 1 * time.Hour, RefreshTokenTTL: 24 * time.Hour},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := run(ctx, cfg, logger)
	if err == nil {
		t.Fatal("run should fail on missing database URL")
	}
}

func TestFlushTrafficNoop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Should not panic on nil inputs
	flushTraffic(context.Background(), nil, nil, logger)
}
