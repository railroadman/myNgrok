package database

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPoolCloseIdempotent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	pool.Close()
	pool.Close()
}

func TestPingAfterClose(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	pool.Close()
	err = pool.Ping(ctx)
	if err == nil {
		t.Fatal("Ping after Close should fail")
	}
}

func TestRawPoolHandle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()
	raw := pool.Raw()
	if raw == nil {
		t.Fatal("Raw() should return non-nil pool")
	}
}
