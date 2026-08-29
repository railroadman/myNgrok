package server

import (
	"net/http"
	"strings"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health/live" || r.URL.Path == "/health/ready" {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
			w.Header().Set("Cache-Control", "no-store")
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000")
			}
		}
		next.ServeHTTP(w, r)
	})
}
