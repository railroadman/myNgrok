package tunnels

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/myngrok/backend/internal/auth"
	"github.com/myngrok/backend/internal/database"
)

func TestServicePostgresTunnelReconnectLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	email := fmt.Sprintf("tunnel-test-%d@example.test", time.Now().UnixNano())
	var userID, tokenID, agentID string
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO users (email,password_hash) VALUES ($1,'test') RETURNING id::text`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO agent_tokens (user_id,name,token_prefix,token_hash) VALUES ($1,'test','tkn_test','hash') RETURNING id::text`, userID).Scan(&tokenID); err != nil {
		t.Fatal(err)
	}
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO agents (user_id,agent_token_id,instance_id,hostname,os,arch,agent_version,connected) VALUES ($1,$2,'instance','host','linux','amd64','test',TRUE) RETURNING id::text`, userID, tokenID).Scan(&agentID); err != nil {
		t.Fatal(err)
	}
	service := NewService(pool.Raw())
	first, err := service.ReopenForSession(ctx, agentID, "127.0.0.1:8080")
	if err != nil || first.Status != "open" || first.Subdomain == "" {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	service.CloseAgentTunnels(ctx, agentID)
	second, err := service.ReopenForSession(ctx, agentID, "127.0.0.1:8080")
	if err != nil || second.ID != first.ID || second.Subdomain != first.Subdomain || second.Status != "open" {
		t.Fatalf("second=%#v first=%#v err=%v", second, first, err)
	}
	items, err := service.List(ctx, userID)
	if err != nil || len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("items=%#v err=%v", items, err)
	}
}

func TestHTTPHandlerListsAuthenticatedUserTunnels(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	authService := auth.NewService(pool.Raw(), "access", "refresh", time.Hour, time.Hour)
	email := fmt.Sprintf("tunnel-http-%d@example.test", time.Now().UnixNano())
	user, err := authService.Register(ctx, email, "secure-password")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	_, tokens, err := authService.Login(ctx, email, "secure-password", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var tokenID, agentID string
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO agent_tokens (user_id,name,token_prefix,token_hash) VALUES ($1,'test','tkn_test','hash') RETURNING id::text`, user.ID).Scan(&tokenID); err != nil {
		t.Fatal(err)
	}
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO agents (user_id,agent_token_id,instance_id,hostname,os,arch,agent_version,connected) VALUES ($1,$2,'http-instance','win-pc','windows','amd64','test',TRUE) RETURNING id::text`, user.ID, tokenID).Scan(&agentID); err != nil {
		t.Fatal(err)
	}
	service := NewService(pool.Raw())
	opened, err := service.ReopenForSession(ctx, agentID, "127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHTTPHandler(service, authService)

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/tunnels", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tunnels", nil)
	listRequest.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, listRequest)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), opened.Subdomain) {
		t.Fatalf("list status=%d body=%s", listed.Code, listed.Body.String())
	}

	wrongRoute := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tunnels", nil)
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(wrongRoute, request)
	if wrongRoute.Code != http.StatusNotFound {
		t.Fatalf("wrong route status=%d", wrongRoute.Code)
	}
}
