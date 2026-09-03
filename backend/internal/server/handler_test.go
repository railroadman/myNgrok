package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	t.Parallel()
	handler := NewHandler(slog.Default(), func() bool { return true }, http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), nil)
	for _, path := range []string{"/health/live", "/health/ready"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: got status %d", path, recorder.Code)
		}
	}
}

func TestReadinessReportsUnavailableDatabase(t *testing.T) {
	handler := NewHandler(slog.Default(), func() bool { return false }, http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "{\"status\":\"not ready\"}\n" {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestClientIPHandlesAddressWithoutPort(t *testing.T) {
	if got := clientIP("203.0.113.10:443"); got != "203.0.113.10" {
		t.Fatalf("with port=%q", got)
	}
	if got := clientIP("local-client"); got != "local-client" {
		t.Fatalf("without port=%q", got)
	}
}

func TestRequestLogUsesStructuredFields(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := requestLog(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.RemoteAddr = "203.0.113.10:4567"
	handler.ServeHTTP(httptest.NewRecorder(), request)
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["msg"] != "http request" || entry["method"] != http.MethodPost || entry["path"] != "/api/v1/auth/login" || entry["remote_ip"] != "203.0.113.10" {
		t.Fatalf("log entry=%#v", entry)
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Fatalf("duration is missing: %#v", entry)
	}
}
