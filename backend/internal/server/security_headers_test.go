package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersProtectControlPlaneWithoutChangingPublicTunnelTraffic(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	api := httptest.NewRecorder()
	handler.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	for name, expected := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	} {
		if api.Header().Get(name) != expected {
			t.Errorf("%s=%q, want %q", name, api.Header().Get(name), expected)
		}
	}
	public := httptest.NewRecorder()
	handler.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/", nil))
	if public.Header().Get("Content-Security-Policy") != "" {
		t.Fatalf("public tunnel response received control-plane CSP: %#v", public.Header())
	}
}
