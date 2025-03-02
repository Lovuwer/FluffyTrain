// Package handlers provides example job handlers for testing.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
)

// Echo returns the payload as-is. Used for basic testing.
func Echo(ctx context.Context, payload []byte) ([]byte, error) {
	slog.Debug("echo handler: processing",
		"payload_size", len(payload),
	)

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Return payload as result
	result := map[string]interface{}{
		"echo": json.RawMessage(payload),
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	slog.Debug("echo handler: completed")
	return data, nil
}
