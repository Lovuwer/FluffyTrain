// Package middleware provides HTTP middleware for the Titan API.
package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimit returns a middleware that limits requests per second.
func RateLimit(requestsPerSecond int) func(next http.Handler) http.Handler {
	if requestsPerSecond <= 0 {
		// No rate limiting
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	limiter := newTokenBucket(requestsPerSecond)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, `{"error": "rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func newTokenBucket(requestsPerSecond int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(requestsPerSecond),
		maxTokens:  float64(requestsPerSecond),
		refillRate: float64(requestsPerSecond),
		lastRefill: time.Now(),
	}
}

func (tb *tokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now

	// Check if we have tokens
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}

	return false
}
