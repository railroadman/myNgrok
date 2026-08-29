package agenttokens

import (
	"context"
	"encoding/json"
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
	email := fmt.Sprintf("token-test-%d@example.test", time.Now().UnixNano())
	var userID string
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO users (email,password_hash) VALUES ($1,'test') RETURNING id::text`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	service := NewService(pool.Raw())
	created, err := service.Create(ctx, userID, "laptop")
	if err != nil || created.Plaintext == "" || created.Prefix == "" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	if _, err := service.Create(ctx, userID, ""); err == nil {
		t.Fatal("empty name was accepted")
	}
	items, err := service.List(ctx, userID)
	if err != nil || len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	_, ok, err := service.Revoke(ctx, userID, created.ID)
	if err != nil || !ok {
		t.Fatalf("revoke ok=%v err=%v", ok, err)
	}
	if ok, err := func() (bool, error) { _, ok, err := service.Revoke(ctx, userID, created.ID); return ok, err }(); err != nil || ok {
		t.Fatalf("second revoke ok=%v err=%v", ok, err)
	}
}

func TestHTTPHandlerPostgresCreateListAndRevoke(t *testing.T) {
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
	email := fmt.Sprintf("token-http-%d@example.test", time.Now().UnixNano())
	authService := auth.NewService(pool.Raw(), "access", "refresh", time.Hour, time.Hour)
	user, err := authService.Register(ctx, email, "secure-password")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	_, tokens, err := authService.Login(ctx, email, "secure-password", "", "")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHTTPHandler(NewService(pool.Raw()), authService, nil)
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agent-tokens", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}

	malformed := httptest.NewRecorder()
	malformedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/agent-tokens", strings.NewReader(`{`))
	malformedRequest.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(malformed, malformedRequest)
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed status=%d", malformed.Code)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent-tokens", strings.NewReader(`{"name":"http-token"}`))
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, request)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var payload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil || payload.Data.ID == "" {
		t.Fatalf("payload=%s err=%v", created.Body.String(), err)
	}
	list := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/agent-tokens", nil)
	listRequest.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(list, listRequest)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d", list.Code)
	}
	revoke := httptest.NewRecorder()
	revokeRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/agent-tokens/"+payload.Data.ID, nil)
	revokeRequest.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(revoke, revokeRequest)
	if revoke.Code != http.StatusNoContent {
		t.Fatalf("revoke status=%d", revoke.Code)
	}

	missing := httptest.NewRecorder()
	missingRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/agent-tokens/"+payload.Data.ID, nil)
	missingRequest.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(missing, missingRequest)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status=%d", missing.Code)
	}

	invalidPath := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodGet, "/api/v1/agent-tokens/extra", nil)
	invalidRequest.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(invalidPath, invalidRequest)
	if invalidPath.Code != http.StatusNotFound {
		t.Fatalf("invalid path status=%d", invalidPath.Code)
	}
}
