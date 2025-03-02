// package worker - job processing worker
package worker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joek3softwares-boop/titan/internal/job"
	"github.com/joek3softwares-boop/titan/internal/logging"
	"github.com/joek3softwares-boop/titan/internal/queue"
	"github.com/joek3softwares-boop/titan/internal/redis"
	"github.com/joek3softwares-boop/titan/internal/watchdog"
)

// Config configures the worker.
type Config struct {
	// ID is a unique identifier for this worker instance.
	ID string

	// Concurrency is the number of concurrent job processors.
	Concurrency int

	// PollInterval is how long to wait when no jobs are available.
	PollInterval time.Duration

	// JobTimeout is the maximum time a job can run.
	JobTimeout time.Duration

	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	ShutdownTimeout time.Duration

	// BackoffConfig for retry calculations.
	BackoffConfig queue.BackoffConfig
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(id string) Config {
	return Config{
		ID:              id,
		Concurrency:     10,
		PollInterval:    1 * time.Second,
		JobTimeout:      30 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		BackoffConfig:   queue.DefaultBackoffConfig(),
	}
}

// Worker processes jobs from the queue.
type Worker struct {
	cfg        Config
	registry   *Registry
	queue      queue.Queue
	redis      redis.Client
	watchdog   *watchdog.Watchdog
	logger     *slog.Logger
	wg         sync.WaitGroup
	stopCh     chan struct{}
	running    atomic.Bool
	processing atomic.Int32
}

// New creates a new Worker.
func New(cfg Config, registry *Registry, q queue.Queue, redisClient redis.Client, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}

	// Create watchdog
	watchdogCfg := watchdog.DefaultConfig(cfg.ID)
	wd := watchdog.New(redisClient, q, watchdogCfg, logger)

	return &Worker{
		cfg:      cfg,
		registry: registry,
		queue:    q,
		redis:    redisClient,
		watchdog: wd,
		logger:   logger.With("worker_id", cfg.ID),
		stopCh:   make(chan struct{}),
	}
}

// Run starts the worker and blocks until Stop is called.
func (w *Worker) Run(ctx context.Context) error {
	if !w.running.CompareAndSwap(false, true) {
		return nil // Already running
	}

	w.logger.Info("starting worker",
		"concurrency", w.cfg.Concurrency,
		"job_timeout", w.cfg.JobTimeout,
	)

	// Start watchdog
	w.watchdog.Start(ctx)

	// Start worker goroutines
	for i := 0; i < w.cfg.Concurrency; i++ {
		w.wg.Add(1)
		go w.processLoop(ctx, i)
	}

	// Wait for stop signal
	<-w.stopCh

	w.logger.Info("stopping worker")

	// Wait for in-progress jobs
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("all jobs completed")
	case <-time.After(w.cfg.ShutdownTimeout):
		w.logger.Warn("shutdown timeout - some jobs may be incomplete")
	}

	// Stop watchdog
	w.watchdog.Stop()

	w.running.Store(false)
	w.logger.Info("worker stopped")

	return nil
}

// Stop signals the worker to stop processing.
func (w *Worker) Stop() {
	if w.running.Load() {
		close(w.stopCh)
	}
}

// IsHealthy returns true if the worker is healthy.
func (w *Worker) IsHealthy() bool {
	return w.running.Load() && w.redis.IsHealthy()
}

// ProcessingCount returns the number of jobs currently being processed.
func (w *Worker) ProcessingCount() int {
	return int(w.processing.Load())
}

func (w *Worker) processLoop(ctx context.Context, workerNum int) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			if !w.redis.IsHealthy() {
				w.logger.Warn("redis down, backing off")
				time.Sleep(w.cfg.PollInterval)
				continue
			}

			j, err := w.queue.Claim(ctx, w.cfg.PollInterval)
			if err != nil {
				w.logger.Error("claim failed", "err", err)
				time.Sleep(w.cfg.PollInterval)
				continue
			}

			if j == nil {
				continue
			}

			w.processJob(ctx, j)
		}
	}
}

func (w *Worker) processJob(ctx context.Context, j *job.Job) {
	w.processing.Add(1)
	defer w.processing.Add(-1)

	jobCtx := logging.WithJobID(ctx, j.ID)
	jobCtx = logging.WithWorkerID(jobCtx, w.cfg.ID)
	jobCtx, cancel := context.WithTimeout(jobCtx, w.cfg.JobTimeout)
	defer cancel()

	start := time.Now()
	logging.LogJobStarted(w.logger, j.ID, j.Type, w.cfg.ID)
	handler, ok := w.registry.Get(j.Type)
	if !ok {
		w.handleJobFailure(ctx, j, "unknown job type: "+j.Type, start)
		return
	}

	result, err := w.executeHandler(jobCtx, handler, j)
	if err != nil {
		w.handleJobFailure(ctx, j, err.Error(), start)
		return
	}

	if _, ackErr := queue.AckJob(ctx, w.queue, j, result); ackErr != nil {
		w.logger.Error("ack failed", "job_id", j.ID, "err", ackErr)
		return
	}

	duration := time.Since(start).Seconds() * 1000
	logging.LogJobCompleted(w.logger, j.ID, j.Type, w.cfg.ID, duration)
}

func (w *Worker) executeHandler(ctx context.Context, handler Handler, j *job.Job) (result []byte, err error) {
	defer func() {
		if r := RecoverPanic(w.logger, j.ID, j.Type); r.Recovered {
			err = r.Error
		}
	}()

	return handler.Handle(ctx, j.Payload)
}

func (w *Worker) handleJobFailure(ctx context.Context, j *job.Job, errMsg string, start time.Time) {
	logging.LogJobFailed(w.logger, j.ID, j.Type, w.cfg.ID, j.Attempts+1, j.MaxRetries, errMsg)

	nackResult, nackErr := queue.NackJob(ctx, w.queue, j, errMsg, w.cfg.BackoffConfig)
	if nackErr != nil {
		w.logger.Error("nack failed", "job_id", j.ID, "err", nackErr)
		return
	}

	if nackResult.WillRetry {
		nextRetry := ""
		if nackResult.NextRetryAt != nil {
			nextRetry = *nackResult.NextRetryAt
		}
		logging.LogJobRetrying(w.logger, j.ID, j.Type, nackResult.Attempt, nackResult.MaxRetries, nextRetry)
	} else {
		logging.LogJobDead(w.logger, j.ID, j.Type, nackResult.Attempt, errMsg)
	}
}
