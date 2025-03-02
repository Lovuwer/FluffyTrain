// package queue - redis-backed job queue with reliable delivery
package queue

import (
	"context"
	"fmt"

	"github.com/joek3softwares-boop/titan/internal/job"
)

// AckResult - result of acknowledging a job
type AckResult struct {
	JobID   string
	Success bool
	Result  []byte
	Error   error
}

// NackResult - result of failing a job
type NackResult struct {
	JobID       string
	Attempt     int
	MaxRetries  int
	WillRetry   bool
	MovedToDLQ  bool
	NextRetryAt *string
	Error       error
}

// AckJob - mark job as done
func AckJob(ctx context.Context, q Queue, j *job.Job, result []byte) (*AckResult, error) {
	if j == nil {
		return nil, fmt.Errorf("can't ack nil job")
	}

	j.MarkCompleted(result)

	if qq, ok := q.(*queue); ok {
		data, err := j.Marshal()
		if err != nil {
			return nil, fmt.Errorf("couldn't serialize job: %w", err)
		}

		if err := qq.client.Set(ctx, qq.jobKey(j.ID), data, 0); err != nil {
			return nil, fmt.Errorf("couldn't update job: %w", err)
		}
	}

	if err := q.Ack(ctx, j.ID); err != nil {
		return &AckResult{
			JobID:   j.ID,
			Success: false,
			Result:  result,
			Error:   err,
		}, err
	}

	return &AckResult{
		JobID:   j.ID,
		Success: true,
		Result:  result,
	}, nil
}

// NackJob - mark job as failed, handles retry logic
func NackJob(ctx context.Context, q Queue, j *job.Job, errMsg string, cfg BackoffConfig) (*NackResult, error) {
	if j == nil {
		return nil, fmt.Errorf("can't nack nil job")
	}

	nextAttempt := j.Attempts + 1
	shouldRetry := nextAttempt < j.MaxRetries

	result := &NackResult{
		JobID:      j.ID,
		Attempt:    nextAttempt,
		MaxRetries: j.MaxRetries,
		WillRetry:  shouldRetry,
		MovedToDLQ: !shouldRetry,
	}

	if shouldRetry {
		nextRetry := NextRetryTime(cfg, nextAttempt).Format("2006-01-02T15:04:05Z07:00")
		result.NextRetryAt = &nextRetry
	}

	if err := q.Nack(ctx, j, errMsg, shouldRetry); err != nil {
		result.Error = err
		return result, err
	}

	return result, nil
}
