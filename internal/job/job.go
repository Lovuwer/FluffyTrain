// package job - job data structures and serialization
package job

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// priority levels
const (
	PriorityLow      = 1
	PriorityNormal   = 5
	PriorityCritical = 10
)

const DefaultMaxRetries = 5

// Job - a unit of work
type Job struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Payload     []byte            `json:"payload"`
	Priority    int               `json:"priority"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ScheduledAt time.Time         `json:"scheduled_at"`
	Attempts    int               `json:"attempts"`
	MaxRetries  int               `json:"max_retries"`
	LastError   string            `json:"last_error,omitempty"`
	Status      Status            `json:"status"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	UniqueKey   string            `json:"unique_key,omitempty"`
	Result      []byte            `json:"result,omitempty"`
}

// NewJob - create job with default priority and retries
func NewJob(jobType string, payload []byte) *Job {
	now := time.Now().UTC()
	return &Job{
		ID:          uuid.New().String(),
		Type:        jobType,
		Payload:     payload,
		Priority:    PriorityNormal,
		CreatedAt:   now,
		UpdatedAt:   now,
		ScheduledAt: now,
		Attempts:    0,
		MaxRetries:  DefaultMaxRetries,
		Status:      StatusPending,
		Metadata:    make(map[string]string),
	}
}

// NewPriorityJob - create job with custom priority
func NewPriorityJob(jobType string, payload []byte, priority int) *Job {
	job := NewJob(jobType, payload)
	job.Priority = priority
	return job
}

// NewDelayedJob - create job to run at a specific time
func NewDelayedJob(jobType string, payload []byte, scheduledAt time.Time) *Job {
	job := NewJob(jobType, payload)
	job.ScheduledAt = scheduledAt.UTC()
	return job
}

// Validate - check job fields are valid
func (j *Job) Validate() error {
	if j.ID == "" {
		return ErrMissingID
	}

	if _, err := uuid.Parse(j.ID); err != nil {
		return ErrInvalidID
	}

	if j.Type == "" {
		return ErrMissingType
	}

	if j.Priority < 1 || j.Priority > 10 {
		return ErrInvalidPriority
	}

	if j.MaxRetries < 0 {
		return ErrInvalidMaxRetries
	}

	return nil
}

// Marshal - serialize to json
func (j *Job) Marshal() ([]byte, error) {
	data, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("couldn't serialize job: %w", err)
	}
	return data, nil
}

// Unmarshal - deserialize from json
func Unmarshal(data []byte) (*Job, error) {
	var j Job
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("couldn't parse job: %w", err)
	}
	return &j, nil
}

func (j *Job) SetMetadata(key, value string) {
	if j.Metadata == nil {
		j.Metadata = make(map[string]string)
	}
	j.Metadata[key] = value
}

func (j *Job) GetMetadata(key string) string {
	if j.Metadata == nil {
		return ""
	}
	return j.Metadata[key]
}

func (j *Job) MarkProcessing() {
	j.Status = StatusProcessing
	j.UpdatedAt = time.Now().UTC()
}

func (j *Job) MarkCompleted(result []byte) {
	j.Status = StatusCompleted
	j.Result = result
	j.UpdatedAt = time.Now().UTC()
}

func (j *Job) MarkFailed(errMsg string) {
	j.Status = StatusFailed
	j.LastError = errMsg
	j.Attempts++
	j.UpdatedAt = time.Now().UTC()
}

func (j *Job) MarkDead(errMsg string) {
	j.Status = StatusDead
	j.LastError = errMsg
	j.UpdatedAt = time.Now().UTC()
}

func (j *Job) ShouldRetry() bool {
	return j.Attempts < j.MaxRetries
}

// validation errors
var (
	ErrMissingID         = fmt.Errorf("id is required")
	ErrInvalidID         = fmt.Errorf("id must be a valid UUID")
	ErrMissingType       = fmt.Errorf("type is required")
	ErrInvalidPriority   = fmt.Errorf("priority must be between 1 and 10")
	ErrInvalidMaxRetries = fmt.Errorf("max_retries can't be negative")
)
