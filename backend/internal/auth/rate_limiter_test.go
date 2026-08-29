package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthRateLimiterLimitsOneIPWithoutAffectingOthers(t *testing.T) {
	limiter := newAuthRateLimiter(2, time.Minute)
	now := time.Now()
	if allowed, _ := limiter.allow("203.0.113.10", now); !allowed {
		t.Fatal("first attempt was rejected")
	}
	if allowed, _ := limiter.allow("203.0.113.10", now); !allowed {
		t.Fatal("second attempt was rejected")
	}
	if allowed, retry := limiter.allow("203.0.113.10", now); allowed || retry <= 0 {
		t.Fatalf("third attempt allowed=%v retry=%s", allowed, retry)
	}
	if allowed, _ := limiter.allow("203.0.113.11", now); !allowed {
		t.Fatal("different IP was rate limited")
	}
}

func TestAuthHandlerReturns429ForRateLimitedLogin(t *testing.T) {
	handler := &HTTPHandler{limiter: newAuthRateLimiter(1, time.Minute)}
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{"))
		r.RemoteAddr = "203.0.113.10:1234"
		return r
	}
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, request())
	if first.Code != http.StatusBadRequest {
		t.Fatalf("first status=%d", first.Code)
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request())
	if second.Code != http.StatusTooManyRequests || second.Header().Get("Retry-After") == "" {
		t.Fatalf("second status=%d retry-after=%q", second.Code, second.Header().Get("Retry-After"))
	}
}
