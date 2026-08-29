package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsExposeRequestCountersWithLowCardinalityRoute(t *testing.T) {
	metrics := NewMetrics()
	handler := metrics.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/tunnels/secret-id", nil))
	response := httptest.NewRecorder()
	metrics.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `myngrok_http_requests_total{method="GET",route="api"} 1`) || strings.Contains(body, "secret-id") {
		t.Fatalf("status=%d body=%s", response.Code, body)
	}
}

func TestMetricsExposeGatewayCollector(t *testing.T) {
	metrics := NewMetrics()
	metrics.SetGatewayCollector(func() GatewayMetrics {
		return GatewayMetrics{ActiveSessions: 2, PendingRequests: 3, ConnectionsTotal: 5, DisconnectionsTotal: 1}
	})
	response := httptest.NewRecorder()
	metrics.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, expected := range []string{"myngrok_gateway_active_sessions 2", "myngrok_gateway_pending_requests 3", "myngrok_gateway_connections_total 5", "myngrok_gateway_disconnections_total 1"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("metric %q is missing from %s", expected, response.Body.String())
		}
	}
}

func TestMetricsExposeTunnelTrafficCollector(t *testing.T) {
	metrics := NewMetrics()
	metrics.SetTunnelTrafficCollector(func() TunnelTrafficMetrics {
		return TunnelTrafficMetrics{RequestsTotal: 2, RequestBytes: 12, ResponseBytes: 34}
	})
	response := httptest.NewRecorder()
	metrics.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, expected := range []string{"myngrok_tunnel_traffic_requests_total 2", "myngrok_tunnel_traffic_request_bytes_total 12", "myngrok_tunnel_traffic_response_bytes_total 34"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("metric %q is missing from %s", expected, response.Body.String())
		}
	}
}
