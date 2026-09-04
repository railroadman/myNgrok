package auth

import (
	"context"
	"testing"
)

func TestUserFromAccessTokenRejectsEmpty(t *testing.T) {
	s := &Service{}
	_, err := s.UserFromAccessToken(context.Background(), "")
	if err == nil {
		t.Fatal("want error on empty token")
	}
}

func TestUserFromAccessTokenRejectsInvalid(t *testing.T) {
	s := &Service{}
	_, err := s.UserFromAccessToken(context.Background(), "invalid.token.format")
	if err == nil {
		t.Fatal("want error on malformed token")
	}
}

func TestUserFromAccessTokenRejectsWrongSecret(t *testing.T) {
	s := &Service{accessSecret: []byte("secret1")}
	_, err := s.UserFromAccessToken(context.Background(), "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLWlkIn0.invalid")
	if err == nil {
		t.Fatal("want error on wrong secret")
	}
}
