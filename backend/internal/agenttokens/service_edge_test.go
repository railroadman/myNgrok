package agenttokens

import (
	"context"
	"strings"
	"testing"
)

func TestCreateRejectsEmptyName(t *testing.T) {
	s := &Service{pool: nil}
	_, err := s.Create(context.Background(), "user-id", "")
	if err == nil {
		t.Fatal("want error on empty name")
	}
}

func TestCreateRejectsWhitespaceName(t *testing.T) {
	s := &Service{pool: nil}
	_, err := s.Create(context.Background(), "user-id", "   ")
	if err == nil {
		t.Fatal("want error on whitespace-only name")
	}
}

func TestCreateRejectsTooLongName(t *testing.T) {
	s := &Service{pool: nil}
	longName := strings.Repeat("x", 129)
	_, err := s.Create(context.Background(), "user-id", longName)
	if err == nil {
		t.Fatal("want error on name > 128 chars")
	}
}

func TestTokenModelDefaults(t *testing.T) {
	tok := Token{ID: "id1", Name: "test"}
	if tok.LastUsedAt != nil || tok.ExpiresAt != nil || tok.RevokedAt != nil {
		t.Fatal("optional timestamps should start nil")
	}
}
