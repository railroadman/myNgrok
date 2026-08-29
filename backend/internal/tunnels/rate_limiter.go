package tunnels

import (
	"sync"
	"time"
)

const (
	publicRateLimit  = 120
	publicRateWindow = time.Minute
)

type publicRateEntry struct {
	started time.Time
	count   int
}

// publicRateLimiter is scoped to one gateway process, matching the MVP's
// single-gateway deployment model.
type publicRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]publicRateEntry
}

func newPublicRateLimiter(limit int, window time.Duration) *publicRateLimiter {
	return &publicRateLimiter{limit: limit, window: window, entries: make(map[string]publicRateEntry)}
}

func (l *publicRateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for currentKey, entry := range l.entries {
		if now.Sub(entry.started) >= l.window {
			delete(l.entries, currentKey)
		}
	}
	entry := l.entries[key]
	if entry.started.IsZero() || now.Sub(entry.started) >= l.window {
		l.entries[key] = publicRateEntry{started: now, count: 1}
		return true, 0
	}
	if entry.count >= l.limit {
		return false, l.window - now.Sub(entry.started)
	}
	entry.count++
	l.entries[key] = entry
	return true, 0
}
