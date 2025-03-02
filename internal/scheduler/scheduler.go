// Package scheduler provides scheduled job functionality for the Titan job queue system.
package scheduler

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/joek3softwares-boop/titan/internal/job"
	"github.com/joek3softwares-boop/titan/internal/queue"
	"github.com/joek3softwares-boop/titan/internal/redis"
	"github.com/joek3softwares-boop/titan/internal/watchdog"
)

// Config configures the scheduler.
type Config struct {
	// PollInterval is how often to check for due jobs.
	PollInterval time.Duration

	// BatchSize is the maximum number of jobs to move per poll.
	BatchSize int

	// KeyPrefix is the Redis key prefix.
	KeyPrefix string

	// LeaderConfig for leader election.
	LeaderConfig watchdog.LeaderConfig
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(schedulerID string) Config {
	return Config{
		PollInterval: 1 * time.Second,
		BatchSize:    100,
		KeyPrefix:    "titan",
		LeaderConfig: watchdog.LeaderConfig{
			LockKey:       "titan:scheduler:leader",
			LeaderID:      schedulerID,
			TTL:           30 * time.Second,
			RenewInterval: 10 * time.Second,
		},
	}
}

// Scheduler moves scheduled jobs to pending queues when they are due.
type Scheduler struct {
	client  redis.Client
	queue   queue.Queue
	leader  watchdog.LeaderElection
	config  Config
	logger  *slog.Logger
	stopCh  chan struct{}
	wg      sync.WaitGroup
	running bool
	mu      sync.Mutex
}

// New creates a new Scheduler.
func New(client redis.Client, q queue.Queue, cfg Config, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Scheduler{
		client: client,
		queue:  q,
		leader: watchdog.NewLeaderElection(client, cfg.LeaderConfig),
		config: cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Start begins the scheduler process.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run(ctx)
}

// Stop stops the scheduler process.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()

	// Release leadership
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.leader.Release(ctx)
}

func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()

	pollTicker := time.NewTicker(s.config.PollInterval)
	defer pollTicker.Stop()

	renewTicker := time.NewTicker(s.config.LeaderConfig.RenewInterval)
	defer renewTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-renewTicker.C:
			s.tryMaintainLeadership(ctx)
		case <-pollTicker.C:
			s.pollAndEnqueue(ctx)
		}
	}
}

func (s *Scheduler) tryMaintainLeadership(ctx context.Context) {
	if s.leader.IsLeader() {
		if err := s.leader.Renew(ctx); err != nil {
			s.logger.Warn("failed to renew scheduler leadership", "error", err)
		}
	} else {
		acquired, err := s.leader.TryAcquire(ctx)
		if err != nil {
			s.logger.Debug("failed to acquire scheduler leadership", "error", err)
		} else if acquired {
			s.logger.Info("acquired scheduler leadership",
				"scheduler_id", s.config.LeaderConfig.LeaderID)
		}
	}
}

func (s *Scheduler) pollAndEnqueue(ctx context.Context) {
	if !s.leader.IsLeader() {
		return // Only leader polls
	}

	now := time.Now().Unix()
	scheduledKey := s.config.KeyPrefix + ":scheduled"

	// Get jobs that are due
	jobIDs, err := s.client.ZRangeByScore(ctx, scheduledKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(now, 10),
		Count: int64(s.config.BatchSize),
	})
	if err != nil {
		s.logger.Error("failed to get scheduled jobs", "error", err)
		return
	}

	if len(jobIDs) == 0 {
		return
	}

	s.logger.Debug("moving scheduled jobs to pending", "count", len(jobIDs))

	for _, jobID := range jobIDs {
		if err := s.moveJobToPending(ctx, jobID); err != nil {
			s.logger.Error("failed to move scheduled job",
				"job_id", jobID,
				"error", err,
			)
			continue
		}

		// Remove from scheduled set
		if err := s.client.ZRem(ctx, scheduledKey, jobID); err != nil {
			s.logger.Error("failed to remove from scheduled set",
				"job_id", jobID,
				"error", err,
			)
		}
	}
}

func (s *Scheduler) moveJobToPending(ctx context.Context, jobID string) error {
	// Get job details
	j, err := s.queue.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	// Update status and add to pending queue
	j.Status = job.StatusPending

	// Re-enqueue the job
	return s.queue.Enqueue(ctx, j)
}

// Schedule adds a job to the scheduled queue.
func (s *Scheduler) Schedule(ctx context.Context, j *job.Job, runAt time.Time) error {
	scheduledKey := s.config.KeyPrefix + ":scheduled"

	// Store job data first
	data, err := j.Marshal()
	if err != nil {
		return err
	}

	jobKey := s.config.KeyPrefix + ":job:" + j.ID
	if err := s.client.Set(ctx, jobKey, data, 0); err != nil {
		return err
	}

	// Add to scheduled sorted set
	return s.client.ZAdd(ctx, scheduledKey, redis.Z{
		Score:  float64(runAt.Unix()),
		Member: j.ID,
	})
}

// ScheduleDelay schedules a job to run after a delay.
func (s *Scheduler) ScheduleDelay(ctx context.Context, j *job.Job, delay time.Duration) error {
	return s.Schedule(ctx, j, time.Now().Add(delay))
}

// Cancel removes a job from the scheduled queue.
func (s *Scheduler) Cancel(ctx context.Context, jobID string) error {
	scheduledKey := s.config.KeyPrefix + ":scheduled"
	return s.client.ZRem(ctx, scheduledKey, jobID)
}

// GetScheduledCount returns the number of scheduled jobs.
func (s *Scheduler) GetScheduledCount(ctx context.Context) (int64, error) {
	scheduledKey := s.config.KeyPrefix + ":scheduled"
	return s.client.ZCard(ctx, scheduledKey)
}

// IsLeader returns true if this scheduler is the leader.
func (s *Scheduler) IsLeader() bool {
	return s.leader.IsLeader()
}
