package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/joek3softwares-boop/titan/internal/config"
	"github.com/joek3softwares-boop/titan/internal/handlers"
	"github.com/joek3softwares-boop/titan/internal/logging"
	"github.com/joek3softwares-boop/titan/internal/queue"
	"github.com/joek3softwares-boop/titan/internal/redis"
	"github.com/joek3softwares-boop/titan/internal/worker"
)

func main() {
	// Generate worker ID
	workerID := "worker-" + uuid.New().String()[:8]

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup logger
	logCfg := logging.Config{
		Level:   cfg.Logging.Level,
		Format:  cfg.Logging.Format,
		Service: "worker",
	}
	logger := logging.New(logCfg)
	logging.SetDefault(logger)

	logger.Info("starting Titan worker",
		"worker_id", workerID,
		"concurrency", cfg.Worker.Concurrency,
		"log_level", cfg.Logging.Level,
	)

	// Connect to Redis
	redisOpts := redis.Options{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	}
	redisClient, err := redis.New(redisOpts)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	// Ping Redis
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := redisClient.Ping(ctx); err != nil {
		logger.Error("failed to ping redis", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to redis", "host", cfg.Redis.Host, "port", cfg.Redis.Port)

	// Create queue
	queueOpts := queue.DefaultOptions()
	q, err := queue.New(redisClient, queueOpts)
	if err != nil {
		logger.Error("failed to create queue", "error", err)
		os.Exit(1)
	}

	// Create handler registry and register handlers
	registry := worker.NewRegistry()
	handlers.RegisterAll(registry)
	logger.Info("registered handlers", "types", registry.Types())

	// Create worker
	workerCfg := worker.Config{
		ID:            workerID,
		Concurrency:   cfg.Worker.Concurrency,
		PollInterval:  cfg.Worker.PollInterval,
		JobTimeout:    30 * cfg.Worker.PollInterval, // 30x poll interval as timeout
		ShutdownTimeout: cfg.Worker.BackoffMax,
		BackoffConfig: queue.BackoffConfig{
			Initial: cfg.Worker.BackoffInitial,
			Max:     cfg.Worker.BackoffMax,
			Factor:  cfg.Worker.BackoffFactor,
			Jitter:  0.1,
		},
	}
	w := worker.New(workerCfg, registry, q, redisClient, logger)

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start worker in goroutine
	go func() {
		if err := w.Run(ctx); err != nil {
			logger.Error("worker error", "error", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("received shutdown signal", "signal", sig)

	// Stop worker
	w.Stop()

	logger.Info("worker stopped")
}
