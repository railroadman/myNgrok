package auth

import (
	"testing"
	"time"
)

func TestJWTSignatureAndClaims(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret")
	want := claims{Subject: "user-id", Type: "access", IssuedAt: 1, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	token, err := signJWT(want, secret)
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	got, err := verifyJWT(token, secret)
	if err != nil {
		t.Fatalf("verify JWT: %v", err)
	}
	if got != want {
		t.Fatalf("claims mismatch: got %#v, want %#v", got, want)
	}
	if _, err := verifyJWT(token, []byte("wrong-secret")); err == nil {
		t.Fatal("expected wrong secret to fail")
	}
}

func TestRegistrationValidation(t *testing.T) {
	t.Parallel()
	if validEmail("missing-at") || validEmail("@example.com") || validEmail("user@") {
		t.Fatal("invalid email accepted")
	}
	if !validEmail("user@example.com") {
		t.Fatal("valid email rejected")
	}
}
