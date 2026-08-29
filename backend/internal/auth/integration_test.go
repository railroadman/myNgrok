package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/myngrok/backend/internal/database"
)

func TestAuthServicePostgresLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer pool.Close()
	if err := pool.Migrate(ctx); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	service := NewService(pool.Raw(), "access-secret", "refresh-secret", time.Hour, 24*time.Hour)
	email := fmt.Sprintf("auth-service-%d@example.test", time.Now().UnixNano())
	user, err := service.Register(ctx, email, "secure-password")
	if err != nil || user.Email != email {
		t.Fatalf("register user=%#v err=%v", user, err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	if _, err := service.Register(ctx, user.Email, "secure-password"); err != ErrEmailTaken {
		t.Fatalf("duplicate register error=%v", err)
	}
	loggedIn, tokens, err := service.Login(ctx, user.Email, "secure-password", "test-agent", "203.0.113.10")
	if err != nil || loggedIn.ID != user.ID || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("login user=%#v tokens=%#v err=%v", loggedIn, tokens, err)
	}
	if _, _, err := service.Login(ctx, user.Email, "wrong-password", "", ""); err != ErrInvalidCredentials {
		t.Fatalf("invalid login error=%v", err)
	}
	fromAccess, err := service.UserFromAccessToken(ctx, tokens.AccessToken)
	if err != nil || fromAccess.ID != user.ID {
		t.Fatalf("access user=%#v err=%v", fromAccess, err)
	}
	refreshedUser, refreshed, err := service.Refresh(ctx, tokens.RefreshToken, "test-agent", "203.0.113.10")
	if err != nil || refreshedUser.ID != user.ID || refreshed.AccessToken == "" {
		t.Fatalf("refresh user=%#v tokens=%#v err=%v", refreshedUser, refreshed, err)
	}
	if _, _, err := service.Refresh(ctx, tokens.RefreshToken, "", ""); err != ErrUnauthorized {
		t.Fatalf("reused refresh error=%v", err)
	}
	if err := service.Logout(ctx, refreshed.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Refresh(ctx, refreshed.RefreshToken, "", ""); err != ErrUnauthorized {
		t.Fatalf("logged out refresh error=%v", err)
	}
}
