package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type metricValue struct {
	count    uint64
	duration time.Duration
	inFlight uint64
}

// Metrics exposes a small Prometheus-compatible surface without a dependency.
// Labels use route groups rather than request paths to avoid high cardinality.
type Metrics struct {
	mu               sync.Mutex
	values           map[string]*metricValue
	gatewayCollector func() GatewayMetrics
	trafficCollector func() TunnelTrafficMetrics
}

type GatewayMetrics struct {
	ActiveSessions      uint64
	PendingRequests     uint64
	ConnectionsTotal    uint64
	DisconnectionsTotal uint64
}

type TunnelTrafficMetrics struct {
	RequestsTotal uint64
	RequestBytes  uint64
	ResponseBytes uint64
}

func NewMetrics() *Metrics { return &Metrics{values: make(map[string]*metricValue)} }

func (m *Metrics) SetGatewayCollector(collector func() GatewayMetrics) {
	m.mu.Lock()
	m.gatewayCollector = collector
	m.mu.Unlock()
}

func (m *Metrics) SetTunnelTrafficCollector(collector func() TunnelTrafficMetrics) {
	m.mu.Lock()
	m.trafficCollector = collector
	m.mu.Unlock()
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Method + ":" + metricRoute(r.URL.Path)
		m.mu.Lock()
		m.value(key).inFlight++
		m.mu.Unlock()
		started := time.Now()
		next.ServeHTTP(w, r)
		m.mu.Lock()
		value := m.value(key)
		value.inFlight--
		value.count++
		value.duration += time.Since(started)
		m.mu.Unlock()
	})
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintln(w, "# HELP myngrok_http_requests_total Completed HTTP requests by method and route group.")
	fmt.Fprintln(w, "# TYPE myngrok_http_requests_total counter")
	fmt.Fprintln(w, "# HELP myngrok_http_request_duration_seconds_total Total HTTP request duration by method and route group.")
	fmt.Fprintln(w, "# TYPE myngrok_http_request_duration_seconds_total counter")
	fmt.Fprintln(w, "# HELP myngrok_http_requests_in_flight Current in-flight HTTP requests by method and route group.")
	fmt.Fprintln(w, "# TYPE myngrok_http_requests_in_flight gauge")
	m.mu.Lock()
	keys := make([]string, 0, len(m.values))
	for key := range m.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		method, route, _ := strings.Cut(key, ":")
		value := m.values[key]
		fmt.Fprintf(w, "myngrok_http_requests_total{method=%q,route=%q} %d\n", method, route, value.count)
		fmt.Fprintf(w, "myngrok_http_request_duration_seconds_total{method=%q,route=%q} %.9f\n", method, route, value.duration.Seconds())
		fmt.Fprintf(w, "myngrok_http_requests_in_flight{method=%q,route=%q} %d\n", method, route, value.inFlight)
	}
	collector := m.gatewayCollector
	trafficCollector := m.trafficCollector
	m.mu.Unlock()
	if collector != nil {
		gateway := collector()
		fmt.Fprintln(w, "# HELP myngrok_gateway_active_sessions Current active agent sessions.")
		fmt.Fprintln(w, "# TYPE myngrok_gateway_active_sessions gauge")
		fmt.Fprintf(w, "myngrok_gateway_active_sessions %d\n", gateway.ActiveSessions)
		fmt.Fprintln(w, "# HELP myngrok_gateway_pending_requests Current requests awaiting an agent response.")
		fmt.Fprintln(w, "# TYPE myngrok_gateway_pending_requests gauge")
		fmt.Fprintf(w, "myngrok_gateway_pending_requests %d\n", gateway.PendingRequests)
		fmt.Fprintln(w, "# HELP myngrok_gateway_connections_total Agent sessions registered since process start.")
		fmt.Fprintln(w, "# TYPE myngrok_gateway_connections_total counter")
		fmt.Fprintf(w, "myngrok_gateway_connections_total %d\n", gateway.ConnectionsTotal)
		fmt.Fprintln(w, "# HELP myngrok_gateway_disconnections_total Agent sessions disconnected since process start.")
		fmt.Fprintln(w, "# TYPE myngrok_gateway_disconnections_total counter")
		fmt.Fprintf(w, "myngrok_gateway_disconnections_total %d\n", gateway.DisconnectionsTotal)
	}
	if trafficCollector != nil {
		traffic := trafficCollector()
		fmt.Fprintln(w, "# HELP myngrok_tunnel_traffic_requests_total Successfully completed public tunnel requests.")
		fmt.Fprintln(w, "# TYPE myngrok_tunnel_traffic_requests_total counter")
		fmt.Fprintf(w, "myngrok_tunnel_traffic_requests_total %d\n", traffic.RequestsTotal)
		fmt.Fprintln(w, "# HELP myngrok_tunnel_traffic_request_bytes_total Public request body bytes forwarded to agents.")
		fmt.Fprintln(w, "# TYPE myngrok_tunnel_traffic_request_bytes_total counter")
		fmt.Fprintf(w, "myngrok_tunnel_traffic_request_bytes_total %d\n", traffic.RequestBytes)
		fmt.Fprintln(w, "# HELP myngrok_tunnel_traffic_response_bytes_total Agent response body bytes returned to public clients.")
		fmt.Fprintln(w, "# TYPE myngrok_tunnel_traffic_response_bytes_total counter")
		fmt.Fprintf(w, "myngrok_tunnel_traffic_response_bytes_total %d\n", traffic.ResponseBytes)
	}
}

func (m *Metrics) value(key string) *metricValue {
	value := m.values[key]
	if value == nil {
		value = &metricValue{}
		m.values[key] = value
	}
	return value
}

func metricRoute(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/"):
		return "api"
	case strings.HasPrefix(path, "/health/"):
		return "health"
	default:
		return "tunnel"
	}
}
