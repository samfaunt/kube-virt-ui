package auth

import (
	"sync"
	"time"
)

// FailureLimiter locks a key out after max failures within window.
// State is in memory, which is correct for the single-replica deployment.
type FailureLimiter struct {
	max    int
	window time.Duration

	mu      sync.Mutex
	entries map[string]*failures
}

type failures struct {
	count int
	reset time.Time
}

func NewFailureLimiter(max int, window time.Duration) *FailureLimiter {
	return &FailureLimiter{max: max, window: window, entries: map[string]*failures{}}
}

// Allowed reports whether key may make another attempt.
func (l *FailureLimiter) Allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return true
	}
	if time.Now().After(e.reset) {
		delete(l.entries, key)
		return true
	}
	return e.count < l.max
}

func (l *FailureLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e, ok := l.entries[key]
	if !ok || now.After(e.reset) {
		e = &failures{reset: now.Add(l.window)}
		l.entries[key] = e
	}
	e.count++
	if len(l.entries) > 10_000 {
		l.sweep(now)
	}
}

func (l *FailureLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *FailureLimiter) sweep(now time.Time) {
	for k, e := range l.entries {
		if now.After(e.reset) {
			delete(l.entries, k)
		}
	}
}
