// Package dedup provides job deduplication functionality for the Titan job queue system.
package dedup

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/joek3softwares-boop/titan/internal/redis"
)

// DefaultTTL is the default deduplication window.
const DefaultTTL = 24 * time.Hour

// Config configures the deduplication service.
type Config struct {
	// TTL is the deduplication window.
	TTL time.Duration

	// KeyPrefix is the Redis key prefix for dedup keys.
	KeyPrefix string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		TTL:       DefaultTTL,
		KeyPrefix: "titan:dedup:",
	}
}

// Result represents the result of a deduplication check.
type Result struct {
	// IsDuplicate is true if this is a duplicate job.
	IsDuplicate bool

	// ExistingJobID is the ID of the existing job if duplicate.
	ExistingJobID string
}

// Service provides job deduplication functionality.
type Service struct {
	client redis.Client
	config Config
	logger *slog.Logger
}

// NewService creates a new deduplication service.
func NewService(client redis.Client, config Config, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		client: client,
		config: config,
		logger: logger,
	}
}

// Check checks if a job with the given unique key already exists.
// If the job doesn't exist, it registers the key and returns false.
// If the job exists, it returns true with the existing job ID.
func (s *Service) Check(ctx context.Context, uniqueKey, jobID string) Result {
	if uniqueKey == "" {
		return Result{IsDuplicate: false}
	}

	key := s.config.KeyPrefix + uniqueKey

	// Try to set the key with NX (only set if not exists)
	acquired, err := s.client.SetNX(ctx, key, jobID, s.config.TTL)
	if err != nil {
		// On error, log and proceed (don't block)
		s.logger.Warn("dedup check failed, proceeding with job",
			"error", err,
			"unique_key", uniqueKey,
			"job_id", jobID,
		)
		return Result{IsDuplicate: false}
	}

	if acquired {
		// Successfully registered the key, not a duplicate
		return Result{IsDuplicate: false}
	}

	// Key already exists, this is a duplicate
	existingID, err := s.client.Get(ctx, key)
	if err != nil {
		s.logger.Warn("failed to get existing job ID",
			"error", err,
			"unique_key", uniqueKey,
		)
		return Result{
			IsDuplicate:   true,
			ExistingJobID: "", // Unknown
		}
	}

	return Result{
		IsDuplicate:   true,
		ExistingJobID: existingID,
	}
}

// Register registers a unique key for a job.
func (s *Service) Register(ctx context.Context, uniqueKey, jobID string) error {
	if uniqueKey == "" {
		return nil
	}

	key := s.config.KeyPrefix + uniqueKey
	return s.client.Set(ctx, key, jobID, s.config.TTL)
}

// Remove removes a unique key (e.g., when a job is cancelled).
func (s *Service) Remove(ctx context.Context, uniqueKey string) error {
	if uniqueKey == "" {
		return nil
	}

	key := s.config.KeyPrefix + uniqueKey
	return s.client.Del(ctx, key)
}

// Exists checks if a unique key already exists.
func (s *Service) Exists(ctx context.Context, uniqueKey string) (bool, error) {
	if uniqueKey == "" {
		return false, nil
	}

	key := s.config.KeyPrefix + uniqueKey
	count, err := s.client.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("dedup exists: %w", err)
	}
	return count > 0, nil
}

// GetJobID retrieves the job ID associated with a unique key.
func (s *Service) GetJobID(ctx context.Context, uniqueKey string) (string, error) {
	if uniqueKey == "" {
		return "", nil
	}

	key := s.config.KeyPrefix + uniqueKey
	id, err := s.client.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("dedup get: %w", err)
	}
	return id, nil
}

// ErrDuplicateJob is returned when a duplicate job is detected.
var ErrDuplicateJob = fmt.Errorf("duplicate job detected")
