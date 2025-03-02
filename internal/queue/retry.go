// package queue - redis-backed job queue with reliable delivery
package queue

import (
	"math"
	"math/rand/v2"
	"time"
)

// BackoffConfig - exponential backoff settings
type BackoffConfig struct {
	Initial time.Duration // starting delay
	Max     time.Duration // cap the backoff at this
	Factor  float64       // multiplier for each attempt
	Jitter  float64       // randomness to prevent thundering herd (±10% default)
}

// DefaultBackoffConfig - reasonable defaults
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		Initial: 1 * time.Second,
		Max:     1 * time.Hour,
		Factor:  2.0,
		Jitter:  0.1,
	}
}

// CalculateBackoff - figure out how long to wait before retry
func CalculateBackoff(cfg BackoffConfig, attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	backoff := float64(cfg.Initial) * math.Pow(cfg.Factor, float64(attempt))

	if backoff > float64(cfg.Max) {
		backoff = float64(cfg.Max)
	}

	if cfg.Jitter > 0 {
		jitterRange := backoff * cfg.Jitter
		jitter := (rand.Float64() * 2 * jitterRange) - jitterRange
		backoff += jitter
	}

	if backoff < float64(cfg.Initial) {
		backoff = float64(cfg.Initial)
	}

	return time.Duration(backoff)
}

// NextRetryTime - when to retry this job
func NextRetryTime(cfg BackoffConfig, attempt int) time.Time {
	backoff := CalculateBackoff(cfg, attempt)
	return time.Now().Add(backoff)
}
