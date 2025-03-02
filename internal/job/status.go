// Package job defines data structures and serialization for jobs in the Titan queue.
package job

// Status represents the current state of a job.
type Status int

const (
	// StatusPending indicates the job is waiting to be processed.
	StatusPending Status = iota
	// StatusProcessing indicates the job is currently being processed.
	StatusProcessing
	// StatusCompleted indicates the job has been successfully processed.
	StatusCompleted
	// StatusFailed indicates the job processing failed but may be retried.
	StatusFailed
	// StatusDead indicates the job has exhausted all retries.
	StatusDead
)

// String returns the string representation of the status.
func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusProcessing:
		return "processing"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusDead:
		return "dead"
	default:
		return "unknown"
	}
}

// ParseStatus parses a string into a Status.
func ParseStatus(s string) Status {
	switch s {
	case "pending":
		return StatusPending
	case "processing":
		return StatusProcessing
	case "completed":
		return StatusCompleted
	case "failed":
		return StatusFailed
	case "dead":
		return StatusDead
	default:
		return StatusPending
	}
}

// MarshalJSON implements json.Marshaler.
func (s Status) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *Status) UnmarshalJSON(data []byte) error {
	// Remove quotes
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	*s = ParseStatus(str)
	return nil
}
