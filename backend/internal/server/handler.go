package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func NewHandler(logger *slog.Logger, ready func() bool, authHandler, agentTokenHandler, agentConnectHandler, agentsHandler, tunnelsHandler, publicHandler http.Handler, corsAllowedOrigins []string, configuredMetrics ...*Metrics) http.Handler {
	metrics := NewMetrics()
	if len(configuredMetrics) > 0 && configuredMetrics[0] != nil {
		metrics = configuredMetrics[0]
	}
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", metrics)
	mux.HandleFunc("GET /health/live", health)
	mux.HandleFunc("GET /health/ready", readiness(ready))
	mux.Handle("/api/v1/auth/", authHandler)
	mux.Handle("/api/v1/agent-tokens", agentTokenHandler)
	mux.Handle("/api/v1/agent-tokens/", agentTokenHandler)
	mux.Handle("/api/v1/agent/connect", agentConnectHandler)
	mux.Handle("/api/v1/agents", agentsHandler)
	mux.Handle("/api/v1/tunnels", tunnelsHandler)
	mux.Handle("/", publicHandler)
	return requestID(requestLog(logger, securityHeaders(cors(corsAllowedOrigins, metrics.Middleware(mux)))))
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func readiness(ready func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !ready() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
			return
		}
		health(w, nil)
	}
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "request_id", requestIDFromContext(r.Context()), "method", r.Method, "path", r.URL.Path, "remote_ip", clientIP(r.RemoteAddr), "duration_ms", time.Since(started).Milliseconds())
	})
}

func clientIP(remoteAddr string) string {
	if index := strings.LastIndex(remoteAddr, ":"); index > 0 {
		return remoteAddr[:index]
	}
	return remoteAddr
}
