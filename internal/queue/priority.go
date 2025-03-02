// Package queue implements a reliable job queue using Redis.
package queue

import (
	"github.com/joek3softwares-boop/titan/internal/job"
)

// Priority levels for queue ordering.
const (
	PriorityHigh   = 10 // Critical priority
	PriorityNormal = 5  // Normal priority  
	PriorityLow    = 1  // Low priority
)

// PriorityQueues returns queue keys in priority order (highest first).
func PriorityQueues(prefix string) []string {
	return []string{
		prefix + ":queue:10:pending", // Critical
		prefix + ":queue:5:pending",  // Normal
		prefix + ":queue:1:pending",  // Low
	}
}

// GetPriorityQueueKey returns the pending queue key for a given priority.
func GetPriorityQueueKey(prefix string, priority int) string {
	// Normalize priority to supported levels
	switch {
	case priority >= PriorityHigh:
		return prefix + ":queue:10:pending"
	case priority >= PriorityNormal:
		return prefix + ":queue:5:pending"
	default:
		return prefix + ":queue:1:pending"
	}
}

// GetPriorityFromJob returns the normalized priority level for a job.
func GetPriorityFromJob(j *job.Job) int {
	switch {
	case j.Priority >= PriorityHigh:
		return PriorityHigh
	case j.Priority >= PriorityNormal:
		return PriorityNormal
	default:
		return PriorityLow
	}
}
