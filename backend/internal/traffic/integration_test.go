package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/myngrok/backend/internal/auth"
	"github.com/myngrok/backend/internal/database"
)

func TestServicePostgresAddDeltaAccumulatesAndGetTotalsReturnsThem(t *testing.T) {
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
	email := fmt.Sprintf("traffic-test-%d@example.test", time.Now().UnixNano())
	var userID string
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO users (email,password_hash) VALUES ($1,'test') RETURNING id::text`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)

	service := NewService(pool.Raw())

	empty, err := service.GetTotals(ctx, userID)
	if err != nil || empty != (Totals{}) {
		t.Fatalf("empty totals=%#v err=%v", empty, err)
	}

	if err := service.AddDelta(ctx, userID, Metrics{RequestsTotal: 2, RequestBytes: 100, ResponseBytes: 300}); err != nil {
		t.Fatal(err)
	}
	if err := service.AddDelta(ctx, userID, Metrics{RequestsTotal: 1, RequestBytes: 50, ResponseBytes: 150}); err != nil {
		t.Fatal(err)
	}

	totals, err := service.GetTotals(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if totals.RequestsTotal != 3 || totals.RequestBytes != 150 || totals.ResponseBytes != 450 {
		t.Fatalf("totals=%#v", totals)
	}
}

func TestHTTPHandlerPostgresRequiresAuthAndReturnsTotals(t *testing.T) {
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
	email := fmt.Sprintf("traffic-http-%d@example.test", time.Now().UnixNano())
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

	service := NewService(pool.Raw())
	if err := service.AddDelta(ctx, user.ID, Metrics{RequestsTotal: 1, RequestBytes: 1024, ResponseBytes: 2048}); err != nil {
		t.Fatal(err)
	}

	handler := NewHTTPHandler(service, authService)

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/traffic", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/traffic", nil)
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data Totals `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.RequestBytes != 1024 || payload.Data.ResponseBytes != 2048 {
		t.Fatalf("payload=%#v", payload.Data)
	}
}
