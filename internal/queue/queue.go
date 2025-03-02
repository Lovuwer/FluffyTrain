// package queue - redis-backed job queue with reliable delivery
package queue

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/joek3softwares-boop/titan/internal/job"
	"github.com/joek3softwares-boop/titan/internal/redis"
)

// key prefixes for redis
const (
	DefaultPrefix        = "titan"
	ProcessingQueue      = ":queue:processing"
	DeadLetterQueue      = ":queue:dead"
	JobKeyPrefix         = ":job:"
	JobLockSuffix        = ":lock"
	ProcessingTimestamps = ":processing:timestamps"
	DedupKeyPrefix       = ":dedup:"
)

// Queue - main interface for job queue operations. use for mocking in tests
type Queue interface {
	Enqueue(ctx context.Context, j *job.Job) error
	EnqueueBatch(ctx context.Context, jobs []*job.Job) error
	Claim(ctx context.Context, timeout time.Duration) (*job.Job, error)
	Ack(ctx context.Context, jobID string) error
	Nack(ctx context.Context, j *job.Job, errMsg string, shouldRetry bool) error
	GetJob(ctx context.Context, jobID string) (*job.Job, error)
	DeleteJob(ctx context.Context, jobID string) error
	GetStats(ctx context.Context) (*Stats, error)
	ListDLQ(ctx context.Context, offset, limit int64) ([]*job.Job, error)
	RetryDLQJob(ctx context.Context, jobID string) error
	Close() error
}

// Stats - queue depth info
type Stats struct {
	PendingHigh    int64 `json:"pending_high"`
	PendingNormal  int64 `json:"pending_normal"`
	PendingLow     int64 `json:"pending_low"`
	Processing     int64 `json:"processing"`
	Dead           int64 `json:"dead"`
}

// Options - tweak these for your setup
type Options struct {
	Prefix            string        // redis key prefix
	VisibilityTimeout time.Duration // how long before we recover stuck jobs
	LockTTL           time.Duration // processing lock ttl
	DeduplicationTTL  time.Duration // how long to remember unique keys
}

// DefaultOptions - reasonable defaults, tweak for your setup
func DefaultOptions() Options {
	return Options{
		Prefix:            DefaultPrefix,
		VisibilityTimeout: 5 * time.Minute,
		LockTTL:           10 * time.Minute,
		DeduplicationTTL:  24 * time.Hour,
	}
}

type queue struct {
	client  redis.Client
	opts    Options
	scripts Scripts
}

// New - create a queue with the given redis client
func New(client redis.Client, opts Options) (Queue, error) {
	q := &queue{
		client: client,
		opts:   opts,
	}

	// Pre-load Lua scripts
	ctx := context.Background()
	var err error

	q.scripts.ClaimJob, err = client.ScriptLoad(ctx, claimJobScript)
	if err != nil {
		return nil, fmt.Errorf("couldn't load claim script: %w", err)
	}

	q.scripts.AckJob, err = client.ScriptLoad(ctx, ackJobScript)
	if err != nil {
		return nil, fmt.Errorf("couldn't load ack script: %w", err)
	}

	q.scripts.NackJob, err = client.ScriptLoad(ctx, nackJobScript)
	if err != nil {
		return nil, fmt.Errorf("couldn't load nack script: %w", err)
	}

	q.scripts.RecoverStuck, err = client.ScriptLoad(ctx, recoverStuckJobsScript)
	if err != nil {
		return nil, fmt.Errorf("couldn't load recovery script: %w", err)
	}

	q.scripts.DeduplicationCheck, err = client.ScriptLoad(ctx, deduplicationCheckScript)
	if err != nil {
		return nil, fmt.Errorf("couldn't load dedup script: %w", err)
	}

	return q, nil
}

func (q *queue) jobKey(jobID string) string {
	return q.opts.Prefix + JobKeyPrefix + jobID
}

func (q *queue) lockKey(jobID string) string {
	return q.opts.Prefix + JobKeyPrefix + jobID + JobLockSuffix
}

func (q *queue) processingKey() string {
	return q.opts.Prefix + ProcessingQueue
}

func (q *queue) dlqKey() string {
	return q.opts.Prefix + DeadLetterQueue
}

func (q *queue) timestampsKey() string {
	return q.opts.Prefix + ProcessingTimestamps
}

func (q *queue) dedupKey(uniqueKey string) string {
	return q.opts.Prefix + DedupKeyPrefix + uniqueKey
}

func (q *queue) Enqueue(ctx context.Context, j *job.Job) error {
	if err := j.Validate(); err != nil {
		return fmt.Errorf("enqueue: %w", err)
	}

	// check for duplicates
	if j.UniqueKey != "" {
		isDuplicate, err := q.checkDeduplication(ctx, j.UniqueKey)
		if err != nil {
			return fmt.Errorf("dedup check failed: %w", err)
		}
		if isDuplicate {
			return ErrDuplicateJob
		}
	}

	data, err := j.Marshal()
	if err != nil {
		return fmt.Errorf("couldn't serialize job: %w", err)
	}

	if err := q.client.Set(ctx, q.jobKey(j.ID), data, 0); err != nil {
		return fmt.Errorf("couldn't store job: %w", err)
	}

	queueKey := GetPriorityQueueKey(q.opts.Prefix, j.Priority)
	if err := q.client.LPush(ctx, queueKey, j.ID); err != nil {
		_ = q.client.Del(ctx, q.jobKey(j.ID)) // cleanup on failure
		return fmt.Errorf("couldn't add to queue: %w", err)
	}

	return nil
}

func (q *queue) EnqueueBatch(ctx context.Context, jobs []*job.Job) error {
	for _, j := range jobs {
		if err := q.Enqueue(ctx, j); err != nil {
			return err
		}
	}
	return nil
}

func (q *queue) Claim(ctx context.Context, timeout time.Duration) (*job.Job, error) {
	// try each priority queue
	for _, queueKey := range PriorityQueues(q.opts.Prefix) {
		j, err := q.claimFromQueue(ctx, queueKey)
		if err == nil && j != nil {
			return j, nil
		}
		if err != nil && err != redis.ErrNotFound {
			return nil, err
		}
	}

	// no job found, block wait on high priority queue
	highPriorityQueue := q.opts.Prefix + ":queue:10:pending"
	jobID, err := q.client.BLMove(ctx, highPriorityQueue, q.processingKey(), "RIGHT", "LEFT", timeout)
	if err != nil {
		if err == redis.ErrNotFound {
			return nil, nil // timeout, nothing available
		}
		return nil, fmt.Errorf("blocking move failed: %w", err)
	}

	if jobID == "" {
		return nil, nil
	}

	now := time.Now().Unix()
	lockTTL := int(q.opts.LockTTL.Seconds())
	if err := q.client.Set(ctx, q.lockKey(jobID), strconv.FormatInt(now, 10), time.Duration(lockTTL)*time.Second); err != nil {
		return nil, fmt.Errorf("couldn't set processing lock: %w", err)
	}

	if err := q.client.ZAdd(ctx, q.timestampsKey(), redis.Z{Score: float64(now), Member: jobID}); err != nil {
		return nil, fmt.Errorf("couldn't track timestamp: %w", err)
	}

	return q.getAndUpdateJob(ctx, jobID)
}

func (q *queue) claimFromQueue(ctx context.Context, queueKey string) (*job.Job, error) {
	now := time.Now().Unix()
	lockTTL := int(q.opts.LockTTL.Seconds())

	result, err := q.client.EvalSha(ctx, q.scripts.ClaimJob,
		[]string{queueKey, q.processingKey(), q.opts.Prefix + JobKeyPrefix, q.timestampsKey()},
		lockTTL, now)
	if err != nil {
		return nil, fmt.Errorf("claim script blew up: %w", err)
	}

	if result == nil {
		return nil, redis.ErrNotFound
	}

	jobID, ok := result.(string)
	if !ok {
		return nil, fmt.Errorf("got weird result type from claim, expected string")
	}

	return q.getAndUpdateJob(ctx, jobID)
}

func (q *queue) getAndUpdateJob(ctx context.Context, jobID string) (*job.Job, error) {
	j, err := q.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	j.MarkProcessing()

	data, err := j.Marshal()
	if err != nil {
		return nil, fmt.Errorf("couldn't serialize job: %w", err)
	}

	if err := q.client.Set(ctx, q.jobKey(j.ID), data, 0); err != nil {
		return nil, fmt.Errorf("couldn't update job: %w", err)
	}

	return j, nil
}

func (q *queue) Ack(ctx context.Context, jobID string) error {
	result, err := q.client.EvalSha(ctx, q.scripts.AckJob,
		[]string{q.processingKey(), q.lockKey(jobID), q.timestampsKey()},
		jobID)
	if err != nil {
		return fmt.Errorf("ack script failed: %w", err)
	}

	if result == int64(0) {
		return ErrJobNotFound
	}

	return nil
}

func (q *queue) Nack(ctx context.Context, j *job.Job, errMsg string, shouldRetry bool) error {
	j.MarkFailed(errMsg)

	var destQueue string
	var action string
	if shouldRetry && j.ShouldRetry() {
		destQueue = GetPriorityQueueKey(q.opts.Prefix, j.Priority)
		action = "retry"
	} else {
		j.MarkDead(errMsg)
		destQueue = q.dlqKey()
		action = "dlq"
	}

	data, err := j.Marshal()
	if err != nil {
		return fmt.Errorf("couldn't serialize job: %w", err)
	}

	if err := q.client.Set(ctx, q.jobKey(j.ID), data, 0); err != nil {
		return fmt.Errorf("couldn't update job: %w", err)
	}

	result, err := q.client.EvalSha(ctx, q.scripts.NackJob,
		[]string{q.processingKey(), destQueue, q.lockKey(j.ID), q.timestampsKey()},
		j.ID, action)
	if err != nil {
		return fmt.Errorf("nack script failed: %w", err)
	}

	if result == int64(0) {
		return ErrJobNotFound
	}

	return nil
}

func (q *queue) GetJob(ctx context.Context, jobID string) (*job.Job, error) {
	data, err := q.client.Get(ctx, q.jobKey(jobID))
	if err != nil {
		return nil, fmt.Errorf("couldn't get job: %w", err)
	}

	j, err := job.Unmarshal([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("couldn't parse job: %w", err)
	}

	return j, nil
}

func (q *queue) DeleteJob(ctx context.Context, jobID string) error {
	for _, queueKey := range PriorityQueues(q.opts.Prefix) {
		_ = q.client.LRem(ctx, queueKey, 1, jobID)
	}

	if err := q.client.Del(ctx, q.jobKey(jobID)); err != nil {
		return fmt.Errorf("couldn't delete job: %w", err)
	}

	return nil
}

func (q *queue) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}
	var err error

	queues := PriorityQueues(q.opts.Prefix)
	stats.PendingHigh, err = q.client.LLen(ctx, queues[0])
	if err != nil {
		return nil, fmt.Errorf("couldn't get high queue length: %w", err)
	}

	stats.PendingNormal, err = q.client.LLen(ctx, queues[1])
	if err != nil {
		return nil, fmt.Errorf("couldn't get normal queue length: %w", err)
	}

	stats.PendingLow, err = q.client.LLen(ctx, queues[2])
	if err != nil {
		return nil, fmt.Errorf("couldn't get low queue length: %w", err)
	}

	stats.Processing, err = q.client.LLen(ctx, q.processingKey())
	if err != nil {
		return nil, fmt.Errorf("couldn't get processing count: %w", err)
	}

	stats.Dead, err = q.client.LLen(ctx, q.dlqKey())
	if err != nil {
		return nil, fmt.Errorf("couldn't get dlq count: %w", err)
	}

	return stats, nil
}

func (q *queue) ListDLQ(ctx context.Context, offset, limit int64) ([]*job.Job, error) {
	jobIDs, err := q.client.LRange(ctx, q.dlqKey(), offset, offset+limit-1)
	if err != nil {
		return nil, fmt.Errorf("couldn't list dlq: %w", err)
	}

	jobs := make([]*job.Job, 0, len(jobIDs))
	for _, id := range jobIDs {
		j, err := q.GetJob(ctx, id)
		if err != nil {
			continue // skip broken jobs
		}
		jobs = append(jobs, j)
	}

	return jobs, nil
}

func (q *queue) RetryDLQJob(ctx context.Context, jobID string) error {
	j, err := q.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't get job from dlq: %w", err)
	}

	// reset for retry
	j.Status = job.StatusPending
	j.Attempts = 0
	j.LastError = ""

	data, err := j.Marshal()
	if err != nil {
		return fmt.Errorf("couldn't serialize job: %w", err)
	}

	if err := q.client.Set(ctx, q.jobKey(j.ID), data, 0); err != nil {
		return fmt.Errorf("couldn't update job: %w", err)
	}

	if err := q.client.LRem(ctx, q.dlqKey(), 1, jobID); err != nil {
		return fmt.Errorf("couldn't remove from dlq: %w", err)
	}

	queueKey := GetPriorityQueueKey(q.opts.Prefix, j.Priority)
	if err := q.client.LPush(ctx, queueKey, j.ID); err != nil {
		return fmt.Errorf("couldn't requeue job: %w", err)
	}

	return nil
}

func (q *queue) Close() error {
	return q.client.Close()
}

func (q *queue) checkDeduplication(ctx context.Context, uniqueKey string) (bool, error) {
	ttlSeconds := int(q.opts.DeduplicationTTL.Seconds())
	result, err := q.client.EvalSha(ctx, q.scripts.DeduplicationCheck,
		[]string{q.dedupKey(uniqueKey)},
		ttlSeconds)
	if err != nil {
		return false, err
	}

	// Returns 1 if new (not duplicate), 0 if duplicate
	return result == int64(0), nil
}

// Errors
var (
	ErrJobNotFound  = fmt.Errorf("job not found")
	ErrDuplicateJob = fmt.Errorf("duplicate job")
)
