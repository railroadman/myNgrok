package database

import (
	"context"
	"fmt"
	"os"
	"strings"
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

func TestMigrateAppliesAndSkipsMigrationsInAnIsolatedSchema(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("coverage_migrate_%d", time.Now().UnixNano())
	if _, err := admin.Raw().Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Raw().Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	separator := "?"
	if strings.Contains(databaseURL, "?") {
		separator = "&"
	}
	isolationURL := databaseURL + separator + "search_path=" + schema + "%2Cpublic"
	pool, err := Open(ctx, isolationURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Migrate(ctx); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := pool.Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var applied int
	if err := pool.Raw().QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 6 {
		t.Fatalf("applied migrations=%d, want 6", applied)
	}
}
