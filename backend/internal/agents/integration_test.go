package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/myngrok/backend/internal/auth"
	"github.com/myngrok/backend/internal/database"
	"github.com/myngrok/backend/internal/protocol"
)

func TestServicePostgresLifecycle(t *testing.T) {
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
	email := fmt.Sprintf("agent-test-%d@example.test", time.Now().UnixNano())
	var userID string
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO users (email,password_hash) VALUES ($1,'test') RETURNING id::text`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	raw := "tkn_agent_integration"
	sum := sha256.Sum256([]byte(raw))
	var tokenID string
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO agent_tokens (user_id,name,token_prefix,token_hash) VALUES ($1,'test','tkn_agent_', $2) RETURNING id::text`, userID, hex.EncodeToString(sum[:])).Scan(&tokenID); err != nil {
		t.Fatal(err)
	}
	service := NewService(pool.Raw())
	hello := protocol.ClientHelloPayload{InstanceID: "instance-test", Hostname: "host", OS: "linux", Arch: "amd64", AgentVersion: "test"}
	agentID, err := service.Connect(ctx, raw, hello)
	if err != nil || agentID == "" {
		t.Fatalf("connect id=%q err=%v", agentID, err)
	}
	service.Touch(ctx, agentID)
	items, err := service.List(ctx, userID)
	if err != nil || len(items) != 1 || !items[0].Connected {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	service.Disconnect(ctx, agentID)
	items, err = service.List(ctx, userID)
	if err != nil || items[0].Connected {
		t.Fatalf("items=%#v err=%v", items, err)
	}
}

func TestHTTPHandlerListsOnlyAuthenticatedUserAgents(t *testing.T) {
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
	email := fmt.Sprintf("agent-http-%d@example.test", time.Now().UnixNano())
	user, err := authService.Register(ctx, email, "secure-password")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	_, tokens, err := authService.Login(ctx, email, "secure-password", "", "")
	if err != nil {
		t.Fatal(err)
	}
	rawToken := "tkn_agent_http"
	sum := sha256.Sum256([]byte(rawToken))
	if _, err := pool.Raw().Exec(ctx, `INSERT INTO agent_tokens (user_id,name,token_prefix,token_hash) VALUES ($1,'test','tkn_agent_', $2)`, user.ID, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	service := NewService(pool.Raw())
	if _, err := service.Connect(ctx, rawToken, protocol.ClientHelloPayload{InstanceID: "instance-http", Hostname: "win-pc", OS: "windows", Arch: "amd64", AgentVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	handler := NewHTTPHandler(service, authService)

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	listRequest.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, listRequest)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), "win-pc") {
		t.Fatalf("list status=%d body=%s", listed.Code, listed.Body.String())
	}

	wrongRoute := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agents", nil)
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(wrongRoute, request)
	if wrongRoute.Code != http.StatusNotFound {
		t.Fatalf("wrong route status=%d", wrongRoute.Code)
	}
}
