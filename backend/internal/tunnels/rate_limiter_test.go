package tunnels

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/myngrok/backend/internal/gateway"
)

func TestPublicRateLimiterLimitsPerTunnelAndIP(t *testing.T) {
	limiter := newPublicRateLimiter(1, time.Minute)
	now := time.Now()
	if allowed, _ := limiter.allow("tun_1:203.0.113.10", now); !allowed {
		t.Fatal("first request rejected")
	}
	if allowed, _ := limiter.allow("tun_1:203.0.113.10", now); allowed {
		t.Fatal("second request accepted")
	}
	if allowed, _ := limiter.allow("tun_1:203.0.113.11", now); !allowed {
		t.Fatal("different client IP rejected")
	}
	if allowed, _ := limiter.allow("tun_2:203.0.113.10", now); !allowed {
		t.Fatal("different tunnel rejected")
	}
}

func TestPublicHandlerReturns429WhenTunnelRateLimitIsExceeded(t *testing.T) {
	registry := NewRegistry()
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo", SessionID: "missing"})
	handler := NewPublicHandler("tunnel.example.test", registry, gateway.NewSessionManager())
	handler.limiter = newPublicRateLimiter(1, time.Minute)
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://demo.tunnel.example.test/", nil)
		r.Host = "demo.tunnel.example.test"
		r.RemoteAddr = "203.0.113.10:1234"
		return r
	}
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, request())
	if first.Code != http.StatusBadGateway {
		t.Fatalf("first status=%d", first.Code)
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request())
	if second.Code != http.StatusTooManyRequests || second.Header().Get("Retry-After") == "" {
		t.Fatalf("second status=%d retry-after=%q", second.Code, second.Header().Get("Retry-After"))
	}
}
