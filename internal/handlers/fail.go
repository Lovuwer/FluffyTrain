// Package handlers provides example job handlers for testing.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// FailAlways always fails. Used for retry testing.
func FailAlways(ctx context.Context, payload []byte) ([]byte, error) {
	slog.Debug("fail_always handler: failing intentionally")

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return nil, fmt.Errorf("intentional failure")
}

// FailNTimesPayload is the payload for the fail_n_times handler.
type FailNTimesPayload struct {
	FailCount int    `json:"fail_count"`
	JobID     string `json:"job_id"` // Used to track state
}

// Track attempts per job for fail_n_times
var (
	failNTimesState = make(map[string]int)
	failNTimesMu    sync.Mutex
)

// FailNTimes fails N times then succeeds. Used for retry testing.
func FailNTimes(ctx context.Context, payload []byte) ([]byte, error) {
	var p FailNTimesPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, fmt.Errorf("invalid payload: %w", err)
		}
	}

	if p.FailCount <= 0 {
		p.FailCount = 2 // Default to fail twice
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	failNTimesMu.Lock()
	key := p.JobID
	if key == "" {
		key = "default"
	}
	attempts := failNTimesState[key]
	attempts++
	failNTimesState[key] = attempts
	failNTimesMu.Unlock()

	slog.Debug("fail_n_times handler: processing",
		"attempt", attempts,
		"fail_count", p.FailCount,
	)

	if attempts <= p.FailCount {
		return nil, fmt.Errorf("intentional failure %d of %d", attempts, p.FailCount)
	}

	// Clean up state
	failNTimesMu.Lock()
	delete(failNTimesState, key)
	failNTimesMu.Unlock()

	slog.Debug("fail_n_times handler: succeeded after failures",
		"attempts", attempts,
	)

	result := map[string]interface{}{
		"attempts": attempts,
		"message":  "succeeded after failures",
	}
	return json.Marshal(result)
}
