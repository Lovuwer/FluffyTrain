// Package ratelimit provides rate limiting functionality for the Titan job queue system.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/joek3softwares-boop/titan/internal/redis"
)

// RedisLimiter implements a distributed rate limiter using Redis.
type RedisLimiter struct {
	client    redis.Client
	keyPrefix string
	rate      float64
	burst     int
	window    time.Duration
}

// RedisConfig configures the Redis rate limiter.
type RedisConfig struct {
	// KeyPrefix is the Redis key prefix for rate limit keys.
	KeyPrefix string

	// Rate is the number of requests per second.
	Rate float64

	// Burst is the maximum burst size.
	Burst int

	// Window is the sliding window size.
	Window time.Duration
}

// DefaultRedisConfig returns a RedisConfig with sensible defaults.
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		KeyPrefix: "titan:ratelimit:",
		Rate:      100,
		Burst:     50,
		Window:    time.Second,
	}
}

// NewRedisLimiter creates a new distributed rate limiter.
func NewRedisLimiter(client redis.Client, cfg RedisConfig) *RedisLimiter {
	return &RedisLimiter{
		client:    client,
		keyPrefix: cfg.KeyPrefix,
		rate:      cfg.Rate,
		burst:     cfg.Burst,
		window:    cfg.Window,
	}
}

// Allow checks if a request is allowed for the given key.
func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return l.AllowN(ctx, key, 1)
}

// AllowN checks if n requests are allowed for the given key.
func (l *RedisLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	fullKey := l.keyPrefix + key
	now := time.Now()
	windowStart := now.Add(-l.window).UnixMilli()

	// Lua script for atomic rate limiting
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local cost = tonumber(ARGV[4])
		local window_ms = tonumber(ARGV[5])
		
		-- Remove old entries outside the window
		redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)
		
		-- Count current requests in window
		local count = redis.call('ZCARD', key)
		
		-- Check if we're over limit
		if count + cost > limit then
			return 0
		end
		
		-- Add new entries
		for i = 1, cost do
			redis.call('ZADD', key, now, now .. ':' .. i .. ':' .. math.random())
		end
		
		-- Set expiry on the key
		redis.call('PEXPIRE', key, window_ms)
		
		return 1
	`

	result, err := l.client.Eval(ctx, script,
		[]string{fullKey},
		now.UnixMilli(),
		windowStart,
		int(l.rate),
		n,
		int(l.window.Milliseconds()),
	)
	if err != nil {
		return false, fmt.Errorf("redis limiter: %w", err)
	}

	allowed, ok := result.(int64)
	if !ok {
		return false, fmt.Errorf("redis limiter: unexpected result type")
	}

	return allowed == 1, nil
}

// Wait blocks until a request is allowed.
func (l *RedisLimiter) Wait(ctx context.Context, key string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			allowed, err := l.Allow(ctx, key)
			if err != nil {
				return err
			}
			if allowed {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// GetCount returns the current request count for a key.
func (l *RedisLimiter) GetCount(ctx context.Context, key string) (int64, error) {
	fullKey := l.keyPrefix + key
	now := time.Now()
	windowStart := now.Add(-l.window).UnixMilli()

	// Clean up old entries first
	_, err := l.client.Eval(ctx, `redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])`,
		[]string{fullKey},
		windowStart,
	)
	if err != nil {
		return 0, fmt.Errorf("redis limiter get count: %w", err)
	}

	count, err := l.client.ZCard(ctx, fullKey)
	if err != nil {
		return 0, fmt.Errorf("redis limiter get count: %w", err)
	}

	return count, nil
}

// Reset resets the rate limit for a key.
func (l *RedisLimiter) Reset(ctx context.Context, key string) error {
	fullKey := l.keyPrefix + key
	return l.client.Del(ctx, fullKey)
}

// GlobalLimiter provides global rate limiting across all workers.
type GlobalLimiter struct {
	redis *RedisLimiter
	local *Limiter
}

// NewGlobalLimiter creates a limiter that combines local and distributed limiting.
func NewGlobalLimiter(client redis.Client, localCfg Config, redisCfg RedisConfig) *GlobalLimiter {
	return &GlobalLimiter{
		redis: NewRedisLimiter(client, redisCfg),
		local: NewLimiter(localCfg),
	}
}

// Allow checks both local and global limits.
func (l *GlobalLimiter) Allow(ctx context.Context, key string) (bool, error) {
	// Check local limit first (fast path)
	if !l.local.Allow() {
		return false, nil
	}

	// Then check global limit
	return l.redis.Allow(ctx, key)
}
