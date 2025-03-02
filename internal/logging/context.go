// package logging - structured logging for titan
package logging

import (
	"context"
	"log/slog"
)

// Context keys for logging.
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	jobIDKey     contextKey = "job_id"
	workerIDKey  contextKey = "worker_id"
)

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithJobID adds a job ID to the context.
func WithJobID(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, jobIDKey, jobID)
}

// WithWorkerID adds a worker ID to the context.
func WithWorkerID(ctx context.Context, workerID string) context.Context {
	return context.WithValue(ctx, workerIDKey, workerID)
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetJobID retrieves the job ID from context.
func GetJobID(ctx context.Context) string {
	if v := ctx.Value(jobIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetWorkerID retrieves the worker ID from context.
func GetWorkerID(ctx context.Context) string {
	if v := ctx.Value(workerIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// FromContext creates a logger with context values added as attributes.
func FromContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if logger == nil {
		logger = slog.Default()
	}

	attrs := make([]any, 0, 6)

	if rid := GetRequestID(ctx); rid != "" {
		attrs = append(attrs, slog.String("request_id", rid))
	}
	if jid := GetJobID(ctx); jid != "" {
		attrs = append(attrs, slog.String("job_id", jid))
	}
	if wid := GetWorkerID(ctx); wid != "" {
		attrs = append(attrs, slog.String("worker_id", wid))
	}

	if len(attrs) > 0 {
		return logger.With(attrs...)
	}
	return logger
}

// LogRequest logs an HTTP request with standard fields.
func LogRequest(logger *slog.Logger, method, path string, status int, duration float64, requestID string) {
	logger.Info("http request",
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", status),
		slog.Float64("duration_ms", duration),
		slog.String("request_id", requestID),
	)
}

// LogJobStarted logs job processing start.
func LogJobStarted(logger *slog.Logger, jobID, jobType, workerID string) {
	logger.Info("job started",
		slog.String("job_id", jobID),
		slog.String("job_type", jobType),
		slog.String("worker_id", workerID),
	)
}

// LogJobCompleted logs successful job completion.
func LogJobCompleted(logger *slog.Logger, jobID, jobType, workerID string, durationMs float64) {
	logger.Info("job completed",
		slog.String("job_id", jobID),
		slog.String("job_type", jobType),
		slog.String("worker_id", workerID),
		slog.Float64("duration_ms", durationMs),
	)
}

// LogJobFailed logs job failure.
func LogJobFailed(logger *slog.Logger, jobID, jobType, workerID string, attempt, maxRetries int, err string) {
	logger.Error("job failed",
		slog.String("job_id", jobID),
		slog.String("job_type", jobType),
		slog.String("worker_id", workerID),
		slog.Int("attempt", attempt),
		slog.Int("max_retries", maxRetries),
		slog.String("error", err),
	)
}

// LogJobRetrying logs job retry.
func LogJobRetrying(logger *slog.Logger, jobID, jobType string, attempt, maxRetries int, nextRetry string) {
	logger.Warn("job retrying",
		slog.String("job_id", jobID),
		slog.String("job_type", jobType),
		slog.Int("attempt", attempt),
		slog.Int("max_retries", maxRetries),
		slog.String("next_retry", nextRetry),
	)
}

// LogJobDead logs job moved to DLQ.
func LogJobDead(logger *slog.Logger, jobID, jobType string, attempts int, err string) {
	logger.Error("job dead",
		slog.String("job_id", jobID),
		slog.String("job_type", jobType),
		slog.Int("attempts", attempts),
		slog.String("error", err),
	)
}
