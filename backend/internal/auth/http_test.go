package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/myngrok/backend/internal/database"
)

func TestSecureRefreshCookieUsesHostPrefixAndStrictAttributes(t *testing.T) {
	handler := NewHTTPHandler(nil, true)
	response := httptest.NewRecorder()
	handler.setRefreshCookie(response, Tokens{RefreshToken: "token", RefreshExpiresAt: time.Now().Add(time.Hour)})
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%#v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != secureRefreshCookieName || cookie.Path != "/" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" {
		t.Fatalf("cookie=%#v", cookie)
	}
}

func TestDevelopmentRefreshCookieDoesNotRequireHTTPS(t *testing.T) {
	handler := NewHTTPHandler(nil, false)
	response := httptest.NewRecorder()
	handler.setRefreshCookie(response, Tokens{RefreshToken: "token", RefreshExpiresAt: time.Now().Add(time.Hour)})
	cookie := response.Result().Cookies()[0]
	if cookie.Name != refreshCookieName || cookie.Secure {
		t.Fatalf("cookie=%#v", cookie)
	}
}

func TestHTTPHandlerRejectsMalformedAndUnknownRequests(t *testing.T) {
	handler := NewHTTPHandler(nil, false)
	malformed := httptest.NewRecorder()
	handler.ServeHTTP(malformed, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString("{")))
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed status=%d", malformed.Code)
	}
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/api/v1/auth/unknown", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown status=%d", unknown.Code)
	}
}

func TestHTTPHandlerAuthLifecycle(t *testing.T) {
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
	handler := NewHTTPHandler(NewService(pool.Raw(), "access-secret", "refresh-secret", time.Hour, 24*time.Hour), false)
	email := fmt.Sprintf("http-%d@example.test", time.Now().UnixNano())

	doJSON := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.RemoteAddr = "203.0.113.10:1234"
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	registered := doJSON(http.MethodPost, "/api/v1/auth/register", fmt.Sprintf(`{"email":%q,"password":"secure-password"}`, email))
	if registered.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", registered.Code, registered.Body.String())
	}
	login := doJSON(http.MethodPost, "/api/v1/auth/login", fmt.Sprintf(`{"email":%q,"password":"secure-password"}`, email))
	if login.Code != http.StatusOK || len(login.Result().Cookies()) != 1 {
		t.Fatalf("login status=%d cookies=%#v body=%s", login.Code, login.Result().Cookies(), login.Body.String())
	}
	var loginBody struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.NewDecoder(login.Body).Decode(&loginBody); err != nil || loginBody.Data.AccessToken == "" {
		t.Fatalf("decode login err=%v body=%#v", err, loginBody)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+loginBody.Data.AccessToken)
	me := httptest.NewRecorder()
	handler.ServeHTTP(me, meRequest)
	if me.Code != http.StatusOK {
		t.Fatalf("me status=%d body=%s", me.Code, me.Body.String())
	}

	refreshedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refreshedRequest.AddCookie(login.Result().Cookies()[0])
	refreshed := httptest.NewRecorder()
	handler.ServeHTTP(refreshed, refreshedRequest)
	if refreshed.Code != http.StatusOK || len(refreshed.Result().Cookies()) != 1 {
		t.Fatalf("refresh status=%d cookies=%#v body=%s", refreshed.Code, refreshed.Result().Cookies(), refreshed.Body.String())
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.AddCookie(refreshed.Result().Cookies()[0])
	logout := httptest.NewRecorder()
	handler.ServeHTTP(logout, logoutRequest)
	if logout.Code != http.StatusNoContent || len(logout.Result().Cookies()) != 2 {
		t.Fatalf("logout status=%d cookies=%#v", logout.Code, logout.Result().Cookies())
	}
}

func TestHTTPHandlerReportsAuthenticationErrors(t *testing.T) {
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
	service := NewService(pool.Raw(), "access-secret", "refresh-secret", time.Hour, 24*time.Hour)
	handler := NewHTTPHandler(service, false)
	email := fmt.Sprintf("http-errors-%d@example.test", time.Now().UnixNano())
	if _, err := service.Register(ctx, email, "secure-password"); err != nil {
		t.Fatal(err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE email=$1`, email)

	doJSON := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "198.51.100.4:1234"
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	duplicate := doJSON(http.MethodPost, "/api/v1/auth/register", fmt.Sprintf(`{"email":%q,"password":"secure-password"}`, email))
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
	invalidRegistration := doJSON(http.MethodPost, "/api/v1/auth/register", `{"email":"not-an-email","password":"short"}`)
	if invalidRegistration.Code != http.StatusBadRequest {
		t.Fatalf("invalid registration status=%d", invalidRegistration.Code)
	}
	wrongPassword := doJSON(http.MethodPost, "/api/v1/auth/login", fmt.Sprintf(`{"email":%q,"password":"wrong-password"}`, email))
	if wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status=%d", wrongPassword.Code)
	}
	noRefresh := doJSON(http.MethodPost, "/api/v1/auth/refresh", "")
	if noRefresh.Code != http.StatusUnauthorized || len(noRefresh.Result().Cookies()) != 2 {
		t.Fatalf("refresh status=%d cookies=%#v", noRefresh.Code, noRefresh.Result().Cookies())
	}
	noAccessToken := httptest.NewRecorder()
	handler.ServeHTTP(noAccessToken, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if noAccessToken.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status=%d", noAccessToken.Code)
	}
	invalidAccessToken := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	invalidRequest.Header.Set("Authorization", "Bearer invalid")
	handler.ServeHTTP(invalidAccessToken, invalidRequest)
	if invalidAccessToken.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token status=%d", invalidAccessToken.Code)
	}
	logout := httptest.NewRecorder()
	handler.ServeHTTP(logout, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if logout.Code != http.StatusNoContent || len(logout.Result().Cookies()) != 2 {
		t.Fatalf("logout status=%d cookies=%#v", logout.Code, logout.Result().Cookies())
	}
}
