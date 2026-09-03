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

func TestRunRejectsUnavailableDatabaseConfiguration(t *testing.T) {
	cfg := config.Config{DatabaseURL: ""}
	if err := run(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Fatal("run accepted an empty database URL")
	}
}

func TestRunStartsAndGracefullyStops(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	cfg := config.Config{
		Environment:          "development",
		HTTPAddr:             "127.0.0.1:0",
		PublicBaseDomain:     "tunnel.example.test",
		DatabaseURL:          databaseURL,
		Auth:                 config.AuthConfig{AccessSecret: "test-access-secret", RefreshSecret: "test-refresh-secret", AccessTokenTTL: time.Hour, RefreshTokenTTL: time.Hour},
		TunnelRequestTimeout: time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned an error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not stop after context cancellation")
	}
}
