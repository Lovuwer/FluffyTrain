package job

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestNewJob(t *testing.T) {
	payload := []byte(`{"email": "test@example.com"}`)
	j := NewJob("send_email", payload)

	if j.ID == "" {
		t.Error("ID should not be empty")
	}
	if j.Type != "send_email" {
		t.Errorf("Type = %v, want send_email", j.Type)
	}
	if string(j.Payload) != string(payload) {
		t.Errorf("Payload = %v, want %v", string(j.Payload), string(payload))
	}
	if j.Priority != PriorityNormal {
		t.Errorf("Priority = %v, want %v", j.Priority, PriorityNormal)
	}
	if j.Status != StatusPending {
		t.Errorf("Status = %v, want pending", j.Status)
	}
	if j.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %v, want %v", j.MaxRetries, DefaultMaxRetries)
	}
}

func TestNewPriorityJob(t *testing.T) {
	j := NewPriorityJob("critical_task", nil, PriorityCritical)

	if j.Priority != PriorityCritical {
		t.Errorf("Priority = %v, want %v", j.Priority, PriorityCritical)
	}
}

func TestNewDelayedJob(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).UTC()
	j := NewDelayedJob("scheduled_task", nil, future)

	if j.ScheduledAt.Before(time.Now()) {
		t.Error("ScheduledAt should be in the future")
	}
}

func TestJobValidate(t *testing.T) {
	tests := []struct {
		name    string
		job     *Job
		wantErr error
	}{
		{
			name:    "valid job",
			job:     NewJob("test", nil),
			wantErr: nil,
		},
		{
			name: "missing ID",
			job: &Job{
				Type:     "test",
				Priority: 5,
			},
			wantErr: ErrMissingID,
		},
		{
			name: "invalid ID",
			job: &Job{
				ID:       "not-a-uuid",
				Type:     "test",
				Priority: 5,
			},
			wantErr: ErrInvalidID,
		},
		{
			name: "missing type",
			job: func() *Job {
				j := NewJob("", nil)
				return j
			}(),
			wantErr: ErrMissingType,
		},
		{
			name: "invalid priority low",
			job: func() *Job {
				j := NewJob("test", nil)
				j.Priority = 0
				return j
			}(),
			wantErr: ErrInvalidPriority,
		},
		{
			name: "invalid priority high",
			job: func() *Job {
				j := NewJob("test", nil)
				j.Priority = 11
				return j
			}(),
			wantErr: ErrInvalidPriority,
		},
		{
			name: "negative max retries",
			job: func() *Job {
				j := NewJob("test", nil)
				j.MaxRetries = -1
				return j
			}(),
			wantErr: ErrInvalidMaxRetries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if tt.wantErr == nil && err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobMarshalUnmarshal(t *testing.T) {
	original := NewJob("test_job", []byte(`{"key": "value"}`))
	original.SetMetadata("trace_id", "abc123")
	original.UniqueKey = "unique-key-1"

	// Marshal
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Unmarshal
	restored, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Compare
	if restored.ID != original.ID {
		t.Errorf("ID = %v, want %v", restored.ID, original.ID)
	}
	if restored.Type != original.Type {
		t.Errorf("Type = %v, want %v", restored.Type, original.Type)
	}
	if string(restored.Payload) != string(original.Payload) {
		t.Errorf("Payload = %v, want %v", string(restored.Payload), string(original.Payload))
	}
	if restored.Priority != original.Priority {
		t.Errorf("Priority = %v, want %v", restored.Priority, original.Priority)
	}
	if restored.Status != original.Status {
		t.Errorf("Status = %v, want %v", restored.Status, original.Status)
	}
	if restored.GetMetadata("trace_id") != "abc123" {
		t.Errorf("Metadata[trace_id] = %v, want abc123", restored.GetMetadata("trace_id"))
	}
	if restored.UniqueKey != "unique-key-1" {
		t.Errorf("UniqueKey = %v, want unique-key-1", restored.UniqueKey)
	}
}

func TestJobStatusTransitions(t *testing.T) {
	j := NewJob("test", nil)

	// Initially pending
	if j.Status != StatusPending {
		t.Errorf("Initial status = %v, want pending", j.Status)
	}

	// Mark processing
	j.MarkProcessing()
	if j.Status != StatusProcessing {
		t.Errorf("Status after MarkProcessing = %v, want processing", j.Status)
	}

	// Mark completed
	j.MarkCompleted([]byte(`{"result": "success"}`))
	if j.Status != StatusCompleted {
		t.Errorf("Status after MarkCompleted = %v, want completed", j.Status)
	}
	if string(j.Result) != `{"result": "success"}` {
		t.Errorf("Result = %v, want success", string(j.Result))
	}
}

func TestJobRetry(t *testing.T) {
	j := NewJob("test", nil)
	j.MaxRetries = 3

	// First failure
	j.MarkFailed("error 1")
	if j.Attempts != 1 {
		t.Errorf("Attempts = %v, want 1", j.Attempts)
	}
	if !j.ShouldRetry() {
		t.Error("ShouldRetry() = false, want true")
	}

	// Second failure
	j.MarkFailed("error 2")
	if j.Attempts != 2 {
		t.Errorf("Attempts = %v, want 2", j.Attempts)
	}
	if !j.ShouldRetry() {
		t.Error("ShouldRetry() = false, want true")
	}

	// Third failure
	j.MarkFailed("error 3")
	if j.Attempts != 3 {
		t.Errorf("Attempts = %v, want 3", j.Attempts)
	}
	if j.ShouldRetry() {
		t.Error("ShouldRetry() = true, want false (max retries reached)")
	}

	// Mark dead
	j.MarkDead("final error")
	if j.Status != StatusDead {
		t.Errorf("Status after MarkDead = %v, want dead", j.Status)
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusPending, "pending"},
		{StatusProcessing, "processing"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusDead, "dead"},
		{Status(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusJSON(t *testing.T) {
	j := NewJob("test", nil)
	j.Status = StatusProcessing

	data, err := json.Marshal(j)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Check that status is a string in JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	status, ok := raw["status"].(string)
	if !ok {
		t.Error("status should be a string in JSON")
	}
	if status != "processing" {
		t.Errorf("status = %v, want processing", status)
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		input string
		want  Status
	}{
		{"pending", StatusPending},
		{"processing", StatusProcessing},
		{"completed", StatusCompleted},
		{"failed", StatusFailed},
		{"dead", StatusDead},
		{"unknown", StatusPending}, // defaults to pending
		{"", StatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseStatus(tt.input); got != tt.want {
				t.Errorf("ParseStatus(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
