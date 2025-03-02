// Package handlers provides example job handlers for testing.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// HTTPRequestPayload is the payload for the http_request handler.
type HTTPRequestPayload struct {
	URL     string `json:"url"`
	Method  string `json:"method"`
	Timeout int    `json:"timeout"` // seconds
}

// HTTPRequest simulates making an external HTTP call.
// This is a mock - it doesn't make real external API calls.
func HTTPRequest(ctx context.Context, payload []byte) ([]byte, error) {
	var p HTTPRequestPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, fmt.Errorf("invalid payload: %w", err)
		}
	}

	if p.URL == "" {
		p.URL = "https://example.com/api"
	}
	if p.Method == "" {
		p.Method = "GET"
	}
	if p.Timeout <= 0 {
		p.Timeout = 5
	}

	slog.Debug("http_request handler: starting mock request",
		"url", p.URL,
		"method", p.Method,
	)

	// Simulate network latency
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	slog.Debug("http_request handler: mock request completed",
		"url", p.URL,
	)

	// Return mock response
	result := map[string]interface{}{
		"status_code": 200,
		"url":         p.URL,
		"method":      p.Method,
		"body": map[string]interface{}{
			"mock": true,
			"data": "simulated response",
		},
	}
	return json.Marshal(result)
}
