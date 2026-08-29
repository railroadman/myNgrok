package database

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOpenRejectsMissingAndMalformedURLs(t *testing.T) {
	if _, err := Open(context.Background(), ""); err == nil {
		t.Fatal("missing URL was accepted")
	}
	if _, err := Open(context.Background(), "::bad-url"); err == nil {
		t.Fatal("malformed URL was accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Open(ctx, "postgres://tunnel:tunnel@127.0.0.1:1/tunnel?connect_timeout=1"); err == nil {
		t.Fatal("unreachable database was accepted")
	}
}

func TestPoolPostgresLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if pool.Raw() == nil || pool.Ping(ctx) != nil || pool.Migrate(ctx) != nil {
		t.Fatal("database pool lifecycle failed")
	}
	pool.Close()
	if err := pool.Ping(ctx); err == nil {
		t.Fatal("closed pool still passed ping")
	}
}
