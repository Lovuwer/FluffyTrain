// Package ratelimit provides rate limiting functionality for the Titan job queue system.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a token bucket rate limiter.
type Limiter struct {
	mu          sync.Mutex
	tokens      float64
	maxTokens   float64
	refillRate  float64 // tokens per second
	lastRefill  time.Time
}

// Config configures the rate limiter.
type Config struct {
	// Rate is the number of tokens per second.
	Rate float64

	// Burst is the maximum number of tokens.
	Burst int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Rate:  100, // 100 jobs per second
		Burst: 50,  // allow burst of 50
	}
}

// NewLimiter creates a new token bucket rate limiter.
func NewLimiter(cfg Config) *Limiter {
	return &Limiter{
		tokens:     float64(cfg.Burst),
		maxTokens:  float64(cfg.Burst),
		refillRate: cfg.Rate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token if so.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// AllowN checks if n requests are allowed and consumes n tokens if so.
func (l *Limiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens >= float64(n) {
		l.tokens -= float64(n)
		return true
	}
	return false
}

// Wait blocks until a token is available.
func (l *Limiter) Wait() {
	for !l.Allow() {
		time.Sleep(10 * time.Millisecond)
	}
}

// WaitN blocks until n tokens are available.
func (l *Limiter) WaitN(n int) {
	for !l.AllowN(n) {
		time.Sleep(10 * time.Millisecond)
	}
}

// Reserve returns the time to wait before the next token is available.
func (l *Limiter) Reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens >= 1 {
		l.tokens--
		return 0
	}

	// Calculate wait time
	deficit := 1 - l.tokens
	waitTime := time.Duration(deficit / l.refillRate * float64(time.Second))
	return waitTime
}

// Tokens returns the current number of tokens.
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()
	return l.tokens
}

// refill adds tokens based on elapsed time.
func (l *Limiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.tokens += elapsed * l.refillRate
	if l.tokens > l.maxTokens {
		l.tokens = l.maxTokens
	}
	l.lastRefill = now
}

// Reset resets the limiter to full capacity.
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tokens = l.maxTokens
	l.lastRefill = time.Now()
}
