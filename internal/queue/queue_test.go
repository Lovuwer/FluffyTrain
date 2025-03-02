package queue

import (
	"testing"
	"time"
)

func TestPriorityQueues(t *testing.T) {
	queues := PriorityQueues("titan")

	if len(queues) != 3 {
		t.Errorf("PriorityQueues() returned %d queues, want 3", len(queues))
	}

	expected := []string{
		"titan:queue:10:pending",
		"titan:queue:5:pending",
		"titan:queue:1:pending",
	}

	for i, q := range queues {
		if q != expected[i] {
			t.Errorf("Queue[%d] = %v, want %v", i, q, expected[i])
		}
	}
}

func TestGetPriorityQueueKey(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{10, "titan:queue:10:pending"},
		{9, "titan:queue:5:pending"},
		{5, "titan:queue:5:pending"},
		{4, "titan:queue:1:pending"},
		{1, "titan:queue:1:pending"},
		{0, "titan:queue:1:pending"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := GetPriorityQueueKey("titan", tt.priority)
			if got != tt.want {
				t.Errorf("GetPriorityQueueKey(%d) = %v, want %v", tt.priority, got, tt.want)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	cfg := BackoffConfig{
		Initial: 1 * time.Second,
		Max:     1 * time.Hour,
		Factor:  2.0,
		Jitter:  0, // Disable jitter for predictable testing
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{10, 1024 * time.Second},
		{20, 1 * time.Hour}, // Should be capped at max
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := CalculateBackoff(cfg, tt.attempt)
			if got != tt.want {
				t.Errorf("CalculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestCalculateBackoffWithJitter(t *testing.T) {
	cfg := BackoffConfig{
		Initial: 1 * time.Second,
		Max:     1 * time.Hour,
		Factor:  2.0,
		Jitter:  0.1, // ±10%
	}

	// Run multiple times to test jitter produces different values
	results := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		d := CalculateBackoff(cfg, 5)
		results[d] = true

		// Check bounds (32s ± 10% = 28.8s to 35.2s)
		expected := 32 * time.Second
		minVal := time.Duration(float64(expected) * 0.9)
		maxVal := time.Duration(float64(expected) * 1.1)
		if d < minVal || d > maxVal {
			t.Errorf("CalculateBackoff with jitter = %v, want between %v and %v", d, minVal, maxVal)
		}
	}

	// Should have some variation
	if len(results) < 5 {
		t.Log("Warning: Jitter may not be producing enough variation")
	}
}

func TestDefaultBackoffConfig(t *testing.T) {
	cfg := DefaultBackoffConfig()

	if cfg.Initial != 1*time.Second {
		t.Errorf("Initial = %v, want 1s", cfg.Initial)
	}
	if cfg.Max != 1*time.Hour {
		t.Errorf("Max = %v, want 1h", cfg.Max)
	}
	if cfg.Factor != 2.0 {
		t.Errorf("Factor = %v, want 2.0", cfg.Factor)
	}
	if cfg.Jitter != 0.1 {
		t.Errorf("Jitter = %v, want 0.1", cfg.Jitter)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Prefix != "titan" {
		t.Errorf("Prefix = %v, want titan", opts.Prefix)
	}
	if opts.VisibilityTimeout != 5*time.Minute {
		t.Errorf("VisibilityTimeout = %v, want 5m", opts.VisibilityTimeout)
	}
	if opts.LockTTL != 10*time.Minute {
		t.Errorf("LockTTL = %v, want 10m", opts.LockTTL)
	}
}
