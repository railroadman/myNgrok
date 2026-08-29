package server

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestRequestIDGeneratesResponseHeaderAndContextValue(t *testing.T) {
	var contextID string
	handler := requestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { contextID = requestIDFromContext(r.Context()) }))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if !regexp.MustCompile(`^req_[a-f0-9]{24}$`).MatchString(response.Header().Get(requestIDHeader)) || contextID != response.Header().Get(requestIDHeader) {
		t.Fatalf("header=%q context=%q", response.Header().Get(requestIDHeader), contextID)
	}
}

func TestRequestIDAcceptsSafeClientValueAndReplacesUnsafeValue(t *testing.T) {
	handler := requestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	safe := httptest.NewRecorder()
	safeRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	safeRequest.Header.Set(requestIDHeader, "client-request_123")
	handler.ServeHTTP(safe, safeRequest)
	if safe.Header().Get(requestIDHeader) != "client-request_123" {
		t.Fatalf("safe ID was replaced: %q", safe.Header().Get(requestIDHeader))
	}
	unsafe := httptest.NewRecorder()
	unsafeRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	unsafeRequest.Header.Set(requestIDHeader, "invalid value")
	handler.ServeHTTP(unsafe, unsafeRequest)
	if unsafe.Header().Get(requestIDHeader) == "invalid value" {
		t.Fatal("unsafe client request ID was accepted")
	}
}
