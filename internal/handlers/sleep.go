// Package handlers provides example job handlers for testing.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// SleepPayload is the payload for the sleep handler.
type SleepPayload struct {
	Seconds int `json:"seconds"`
}

// Sleep sleeps for the specified number of seconds. Used for timeout testing.
func Sleep(ctx context.Context, payload []byte) ([]byte, error) {
	var p SleepPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, fmt.Errorf("invalid payload: %w", err)
		}
	}

	if p.Seconds <= 0 {
		p.Seconds = 1
	}

	slog.Debug("sleep handler: starting",
		"seconds", p.Seconds,
	)

	// Sleep with context cancellation support
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(p.Seconds) * time.Second):
	}

	slog.Debug("sleep handler: completed",
		"seconds", p.Seconds,
	)

	result := map[string]interface{}{
		"slept_seconds": p.Seconds,
	}
	return json.Marshal(result)
}
