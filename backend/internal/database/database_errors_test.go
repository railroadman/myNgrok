package database

import (
	"context"
	"testing"
	"time"
)

func TestOpenRejectsMissingURL(t *testing.T) {
	_, err := Open(context.Background(), "")
	if err == nil {
		t.Fatal("want error on empty database URL")
	}
}

func TestOpenRejectsMalformedURL(t *testing.T) {
	_, err := Open(context.Background(), "::not-a-url::")
	if err == nil {
		t.Fatal("want error on malformed URL")
	}
}

func TestOpenTimeoutOnUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := Open(ctx, "postgres://127.0.0.1:1/nonexistent?connect_timeout=1")
	if err == nil {
		t.Fatal("want error on unreachable database")
	}
}

func TestRawPoolIsValid(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "postgres://127.0.0.1:1/test?connect_timeout=1"
	p, err := Open(ctx, url)
	if err == nil {
		raw := p.Raw()
		if raw == nil {
			t.Fatal("Raw() should return pool even if connection may fail")
		}
	}
}
