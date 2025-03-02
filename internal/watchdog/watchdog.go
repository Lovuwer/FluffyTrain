// package watchdog - recovers stuck/orphaned jobs
package watchdog

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/joek3softwares-boop/titan/internal/job"
	"github.com/joek3softwares-boop/titan/internal/queue"
	"github.com/joek3softwares-boop/titan/internal/redis"
)

// Watchdog monitors and recovers stuck jobs.
type Watchdog struct {
	client           redis.Client
	queue            queue.Queue
	leader           LeaderElection
	cfg              Config
	logger           *slog.Logger
	stopCh           chan struct{}
	wg               sync.WaitGroup
	running          bool
	mu               sync.Mutex
}

// Config configures the watchdog.
type Config struct {
	// ScanInterval is how often to scan for stuck jobs.
	ScanInterval time.Duration

	// VisibilityTimeout is how long a job can be processing before recovery.
	VisibilityTimeout time.Duration

	// MaxRecoverPerScan is the maximum jobs to recover per scan.
	MaxRecoverPerScan int

	// LeaderConfig configures leader election.
	LeaderConfig LeaderConfig

	// Prefix is the Redis key prefix.
	Prefix string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(workerID string) Config {
	return Config{
		ScanInterval:      60 * time.Second,
		VisibilityTimeout: 5 * time.Minute,
		MaxRecoverPerScan: 100,
		LeaderConfig:      DefaultLeaderConfig(workerID),
		Prefix:            "titan",
	}
}

// New creates a new Watchdog.
func New(client redis.Client, q queue.Queue, cfg Config, logger *slog.Logger) *Watchdog {
	if logger == nil {
		logger = slog.Default()
	}

	return &Watchdog{
		client: client,
		queue:  q,
		leader: NewLeaderElection(client, cfg.LeaderConfig),
		cfg:    cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Start begins the watchdog process.
func (w *Watchdog) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	w.wg.Add(1)
	go w.run(ctx)
}

// Stop stops the watchdog process.
func (w *Watchdog) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopCh)
	w.wg.Wait()

	// Release leadership
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = w.leader.Release(ctx)
}

func (w *Watchdog) run(ctx context.Context) {
	defer w.wg.Done()

	scanTicker := time.NewTicker(w.cfg.ScanInterval)
	defer scanTicker.Stop()

	renewTicker := time.NewTicker(w.cfg.LeaderConfig.RenewInterval)
	defer renewTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-renewTicker.C:
			w.tryMaintainLeadership(ctx)
		case <-scanTicker.C:
			w.scanAndRecover(ctx)
		}
	}
}

func (w *Watchdog) tryMaintainLeadership(ctx context.Context) {
	if w.leader.IsLeader() {
		if err := w.leader.Renew(ctx); err != nil {
			w.logger.Warn("failed to renew leadership", "error", err)
		}
	} else {
		acquired, err := w.leader.TryAcquire(ctx)
		if err != nil {
			w.logger.Debug("failed to acquire leadership", "error", err)
		} else if acquired {
			w.logger.Info("acquired watchdog leadership",
				"worker_id", w.cfg.LeaderConfig.LeaderID)
		}
	}
}

func (w *Watchdog) scanAndRecover(ctx context.Context) {
	if !w.leader.IsLeader() {
		return // Only leader scans
	}

	w.logger.Debug("scanning for stuck jobs")

	stuckJobs, err := w.findStuckJobs(ctx)
	if err != nil {
		w.logger.Error("failed to find stuck jobs", "error", err)
		return
	}

	if len(stuckJobs) == 0 {
		return
	}

	w.logger.Warn("found stuck jobs", "count", len(stuckJobs))

	for _, jobID := range stuckJobs {
		if err := w.recoverJob(ctx, jobID); err != nil {
			w.logger.Error("failed to recover job",
				"job_id", jobID,
				"error", err)
		} else {
			w.logger.Warn("recovered stuck job",
				"job_id", jobID)
		}
	}
}

func (w *Watchdog) findStuckJobs(ctx context.Context) ([]string, error) {
	timestampsKey := w.cfg.Prefix + ":processing:timestamps"
	threshold := time.Now().Add(-w.cfg.VisibilityTimeout).Unix()

	stuckJobs, err := w.client.ZRangeByScore(ctx, timestampsKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(threshold, 10),
		Count: int64(w.cfg.MaxRecoverPerScan),
	})
	if err != nil {
		return nil, err
	}

	return stuckJobs, nil
}

func (w *Watchdog) recoverJob(ctx context.Context, jobID string) error {
	// Get job details
	j, err := w.queue.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	// Increment attempt (recovery counts as a failure)
	j.Attempts++

	// Determine if should go to DLQ
	if j.Attempts >= j.MaxRetries {
		j.MarkDead("recovered after visibility timeout - max retries exceeded")
	} else {
		j.Status = job.StatusPending
		j.LastError = "recovered after visibility timeout"
	}

	// Remove from processing tracking
	timestampsKey := w.cfg.Prefix + ":processing:timestamps"
	processingKey := w.cfg.Prefix + ":queue:processing"
	lockKey := w.cfg.Prefix + ":job:" + jobID + ":lock"

	// Remove from sorted set
	if err := w.client.ZRem(ctx, timestampsKey, jobID); err != nil {
		return err
	}

	// Remove from processing list
	if err := w.client.LRem(ctx, processingKey, 1, jobID); err != nil {
		return err
	}

	// Delete lock
	_ = w.client.Del(ctx, lockKey)

	// Update job data
	data, err := j.Marshal()
	if err != nil {
		return err
	}

	jobKey := w.cfg.Prefix + ":job:" + jobID
	if err := w.client.Set(ctx, jobKey, data, 0); err != nil {
		return err
	}

	// Re-queue or move to DLQ
	if j.Status == job.StatusDead {
		dlqKey := w.cfg.Prefix + ":queue:dead"
		return w.client.LPush(ctx, dlqKey, jobID)
	}

	queueKey := queue.GetPriorityQueueKey(w.cfg.Prefix, j.Priority)
	return w.client.LPush(ctx, queueKey, jobID)
}

// IsLeader returns true if this watchdog is the leader.
func (w *Watchdog) IsLeader() bool {
	return w.leader.IsLeader()
}
