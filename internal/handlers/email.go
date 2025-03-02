// Package handlers provides example job handlers for testing.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// EmailPayload is the payload for the email_mock handler.
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// EmailMock simulates sending an email.
// This is a mock - it doesn't send real emails.
func EmailMock(ctx context.Context, payload []byte) ([]byte, error) {
	var p EmailPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, fmt.Errorf("invalid payload: %w", err)
		}
	}

	if p.To == "" {
		return nil, fmt.Errorf("email 'to' address is required")
	}

	slog.Debug("email_mock handler: starting",
		"to", p.To,
		"subject", p.Subject,
	)

	// Simulate email sending time
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(50 * time.Millisecond):
	}

	slog.Debug("email_mock handler: email sent",
		"to", p.To,
	)

	result := map[string]interface{}{
		"sent":       true,
		"to":         p.To,
		"subject":    p.Subject,
		"message_id": fmt.Sprintf("msg_%d", time.Now().UnixNano()),
	}
	return json.Marshal(result)
}
