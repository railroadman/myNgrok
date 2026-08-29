package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsConfiguredOriginAndPreflight(t *testing.T) {
	handler := cors([]string{"https://app.example.test"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	request.Header.Set("Origin", "https://app.example.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "https://app.example.test" || response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("status=%d headers=%#v", response.Code, response.Header())
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	handler := cors([]string{"https://app.example.test"}, http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.Header.Set("Origin", "https://evil.example.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d", response.Code)
	}
}
