// Package api provides HTTP handlers for the Titan job queue API.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/joek3softwares-boop/titan/internal/api/handlers"
	"github.com/joek3softwares-boop/titan/internal/api/middleware"
	"github.com/joek3softwares-boop/titan/internal/queue"
)

// RouterConfig configures the router.
type RouterConfig struct {
	Queue         queue.Queue
	Logger        *slog.Logger
	RateLimitRate int // requests per second
}

// NewRouter creates a new HTTP router with all API endpoints.
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(cfg.Logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RateLimit(cfg.RateLimitRate))

	// Health endpoints
	r.Get("/health", handlers.Health)
	r.Get("/ready", handlers.Ready(cfg.Queue))

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Jobs
		r.Route("/jobs", func(r chi.Router) {
			r.Post("/", handlers.CreateJob(cfg.Queue, cfg.Logger))
			r.Post("/batch", handlers.CreateJobBatch(cfg.Queue, cfg.Logger))
			r.Get("/{id}", handlers.GetJob(cfg.Queue))
			r.Delete("/{id}", handlers.DeleteJob(cfg.Queue, cfg.Logger))
			r.Get("/{id}/result", handlers.GetJobResult(cfg.Queue))
		})

		// Queue stats
		r.Get("/queues/stats", handlers.GetQueueStats(cfg.Queue))

		// Dead Letter Queue
		r.Route("/dlq", func(r chi.Router) {
			r.Get("/", handlers.ListDLQ(cfg.Queue))
			r.Post("/{id}/retry", handlers.RetryDLQJob(cfg.Queue, cfg.Logger))
		})
	})

	return r
}
