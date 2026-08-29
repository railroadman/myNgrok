package auth

import (
	"sync"
	"time"
)

const (
	authRateLimit  = 10
	authRateWindow = time.Minute
)

type rateWindow struct {
	started time.Time
	count   int
}

// authRateLimiter is deliberately local to one backend instance. The MVP runs
// one gateway; a distributed limiter can replace it before horizontal scaling.
type authRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]rateWindow
}

func newAuthRateLimiter(limit int, window time.Duration) *authRateLimiter {
	return &authRateLimiter{limit: limit, window: window, entries: make(map[string]rateWindow)}
}

func (l *authRateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for currentKey, entry := range l.entries {
		if now.Sub(entry.started) >= l.window {
			delete(l.entries, currentKey)
		}
	}
	entry := l.entries[key]
	if entry.started.IsZero() || now.Sub(entry.started) >= l.window {
		l.entries[key] = rateWindow{started: now, count: 1}
		return true, 0
	}
	if entry.count >= l.limit {
		return false, l.window - now.Sub(entry.started)
	}
	entry.count++
	l.entries[key] = entry
	return true, 0
}
