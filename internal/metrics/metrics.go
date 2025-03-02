// package metrics - prometheus metrics for titan
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// JobsSubmitted - total jobs submitted by priority and type
	JobsSubmitted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "titan_jobs_submitted_total",
			Help: "total jobs submitted",
		},
		[]string{"priority", "type"},
	)

	// JobsProcessed - total jobs processed by status and type
	JobsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "titan_jobs_processed_total",
			Help: "total jobs processed",
		},
		[]string{"status", "type"},
	)

	// JobDuration - histogram of job processing time
	JobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "titan_jobs_duration_seconds",
			Help:    "job processing duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type"},
	)

	// QueueDepth - current queue depth by priority
	QueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "titan_queue_depth",
			Help: "current queue depth",
		},
		[]string{"priority"},
	)

	// WorkersActive - number of active workers
	WorkersActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "titan_workers_active",
			Help: "number of active workers",
		},
	)

	// DLQSize - current dead letter queue size
	DLQSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "titan_dlq_size",
			Help: "dead letter queue size",
		},
	)

	// RedisConnections - current redis connections
	RedisConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "titan_redis_connections",
			Help: "redis connection pool size",
		},
	)

	// RedisErrors - redis errors by operation
	RedisErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "titan_redis_errors_total",
			Help: "redis errors by operation",
		},
		[]string{"operation"},
	)
)

// RecordJobSubmitted records a job submission
func RecordJobSubmitted(priority, jobType string) {
	JobsSubmitted.WithLabelValues(priority, jobType).Inc()
}

// RecordJobProcessed records a processed job
func RecordJobProcessed(status, jobType string) {
	JobsProcessed.WithLabelValues(status, jobType).Inc()
}

// RecordJobDuration records job processing duration
func RecordJobDuration(jobType string, seconds float64) {
	JobDuration.WithLabelValues(jobType).Observe(seconds)
}

// SetQueueDepth sets queue depth for a priority
func SetQueueDepth(priority string, depth float64) {
	QueueDepth.WithLabelValues(priority).Set(depth)
}

// SetWorkersActive sets the number of active workers
func SetWorkersActive(count float64) {
	WorkersActive.Set(count)
}

// SetDLQSize sets the dlq size
func SetDLQSize(size float64) {
	DLQSize.Set(size)
}

// RecordRedisError records a redis error
func RecordRedisError(operation string) {
	RedisErrors.WithLabelValues(operation).Inc()
}
