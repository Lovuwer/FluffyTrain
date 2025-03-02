package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joek3softwares-boop/titan/internal/api"
	"github.com/joek3softwares-boop/titan/internal/config"
	"github.com/joek3softwares-boop/titan/internal/logging"
	"github.com/joek3softwares-boop/titan/internal/queue"
	"github.com/joek3softwares-boop/titan/internal/redis"
)

func main() {
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
		Service: "api",
	}
	logger := logging.New(logCfg)
	logging.SetDefault(logger)

	logger.Info("starting Titan API server",
		"port", cfg.API.Port,
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(ctx); err != nil {
		cancel()
		logger.Error("failed to ping redis", "error", err)
		os.Exit(1)
	}
	cancel()
	logger.Info("connected to redis", "host", cfg.Redis.Host, "port", cfg.Redis.Port)

	// Create queue
	queueOpts := queue.DefaultOptions()
	q, err := queue.New(redisClient, queueOpts)
	if err != nil {
		logger.Error("failed to create queue", "error", err)
		os.Exit(1)
	}

	// Create router
	routerCfg := api.RouterConfig{
		Queue:         q,
		Logger:        logger,
		RateLimitRate: 100, // 100 requests per second
	}
	router := api.NewRouter(routerCfg)

	// Create server
	addr := ":" + strconv.Itoa(cfg.API.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.API.ReadTimeout,
		WriteTimeout: cfg.API.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("API server listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down API server")

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	logger.Info("API server stopped")
}
