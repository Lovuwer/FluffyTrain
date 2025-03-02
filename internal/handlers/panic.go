// Package handlers provides example job handlers for testing.
package handlers

import (
	"context"
	"log/slog"
)

// Panic triggers a panic. Used for recovery testing.
func Panic(ctx context.Context, payload []byte) ([]byte, error) {
	slog.Debug("panic handler: about to panic")

	// Check context cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	panic("intentional panic for testing")
}

// DivideByZero triggers a divide by zero panic. The classic "poison pill".
func DivideByZero(ctx context.Context, payload []byte) ([]byte, error) {
	slog.Debug("divide_by_zero handler: about to divide by zero")

	// Check context cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Trigger integer divide by zero
	x := 1
	y := 0
	_ = x / y // This will panic

	return nil, nil // Never reached
}
