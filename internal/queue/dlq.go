// Package queue implements a reliable job queue using Redis.
package queue

import (
	"context"
	"fmt"

	"github.com/joek3softwares-boop/titan/internal/job"
)

// DLQOperations provides operations for the Dead Letter Queue.
type DLQOperations interface {
	// List returns jobs in the DLQ with pagination.
	List(ctx context.Context, offset, limit int64) ([]*job.Job, error)

	// Count returns the number of jobs in the DLQ.
	Count(ctx context.Context) (int64, error)

	// Retry moves a job from DLQ back to the pending queue.
	Retry(ctx context.Context, jobID string) error

	// Delete permanently removes a job from the DLQ.
	Delete(ctx context.Context, jobID string) error

	// Purge removes all jobs from the DLQ.
	Purge(ctx context.Context) (int64, error)
}

// dlq implements DLQOperations.
type dlq struct {
	queue *queue
}

// NewDLQOperations creates a new DLQOperations instance.
func NewDLQOperations(q Queue) (DLQOperations, error) {
	qq, ok := q.(*queue)
	if !ok {
		return nil, fmt.Errorf("dlq: invalid queue type")
	}
	return &dlq{queue: qq}, nil
}

func (d *dlq) List(ctx context.Context, offset, limit int64) ([]*job.Job, error) {
	return d.queue.ListDLQ(ctx, offset, limit)
}

func (d *dlq) Count(ctx context.Context) (int64, error) {
	return d.queue.client.LLen(ctx, d.queue.dlqKey())
}

func (d *dlq) Retry(ctx context.Context, jobID string) error {
	return d.queue.RetryDLQJob(ctx, jobID)
}

func (d *dlq) Delete(ctx context.Context, jobID string) error {
	// Remove from DLQ list
	if err := d.queue.client.LRem(ctx, d.queue.dlqKey(), 1, jobID); err != nil {
		return fmt.Errorf("dlq delete: failed to remove from list: %w", err)
	}

	// Delete job data
	if err := d.queue.client.Del(ctx, d.queue.jobKey(jobID)); err != nil {
		return fmt.Errorf("dlq delete: failed to delete job data: %w", err)
	}

	return nil
}

func (d *dlq) Purge(ctx context.Context) (int64, error) {
	// Get all job IDs in DLQ
	jobIDs, err := d.queue.client.LRange(ctx, d.queue.dlqKey(), 0, -1)
	if err != nil {
		return 0, fmt.Errorf("dlq purge: failed to list jobs: %w", err)
	}

	// Delete each job's data
	for _, id := range jobIDs {
		_ = d.queue.client.Del(ctx, d.queue.jobKey(id))
	}

	// Clear the DLQ list
	if err := d.queue.client.Del(ctx, d.queue.dlqKey()); err != nil {
		return 0, fmt.Errorf("dlq purge: failed to clear dlq: %w", err)
	}

	return int64(len(jobIDs)), nil
}
