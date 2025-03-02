// Package database provides PostgreSQL connectivity for the Titan job queue system.
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joek3softwares-boop/titan/internal/job"
)

// JobRepository provides database operations for jobs.
type JobRepository struct {
	pool *Pool
}

// NewJobRepository creates a new job repository.
func NewJobRepository(pool *Pool) *JobRepository {
	return &JobRepository{pool: pool}
}

// JobRecord represents a job record in the database.
type JobRecord struct {
	ID          string
	Type        string
	Payload     []byte
	Priority    int
	Status      string
	Attempts    int
	MaxRetries  int
	CreatedAt   time.Time
	ScheduledAt *time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       *string
	Result      []byte
	Metadata    map[string]string
	UniqueKey   *string
}

// Save stores a job in the database.
func (r *JobRepository) Save(ctx context.Context, j *job.Job) error {
	metadataJSON, err := json.Marshal(j.Metadata)
	if err != nil {
		return fmt.Errorf("repository save: failed to marshal metadata: %w", err)
	}

	var scheduledAt, startedAt, completedAt *time.Time
	if !j.ScheduledAt.IsZero() {
		scheduledAt = &j.ScheduledAt
	}

	var errStr *string
	if j.LastError != "" {
		errStr = &j.LastError
	}

	var uniqueKey *string
	if j.UniqueKey != "" {
		uniqueKey = &j.UniqueKey
	}

	query := `
		INSERT INTO jobs (id, type, payload, priority, status, attempts, max_retries, 
			created_at, scheduled_at, started_at, completed_at, error, result, metadata, unique_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			attempts = EXCLUDED.attempts,
			started_at = EXCLUDED.started_at,
			completed_at = EXCLUDED.completed_at,
			error = EXCLUDED.error,
			result = EXCLUDED.result,
			metadata = EXCLUDED.metadata
	`

	_, err = r.pool.Exec(ctx, query,
		j.ID, j.Type, j.Payload, j.Priority, j.Status.String(),
		j.Attempts, j.MaxRetries, j.CreatedAt, scheduledAt,
		startedAt, completedAt, errStr, j.Result, metadataJSON, uniqueKey,
	)
	if err != nil {
		return fmt.Errorf("repository save: %w", err)
	}

	return nil
}

// SaveCompleted stores a completed job in the database.
func (r *JobRepository) SaveCompleted(ctx context.Context, j *job.Job, result []byte) error {
	metadataJSON, err := json.Marshal(j.Metadata)
	if err != nil {
		return fmt.Errorf("repository save completed: failed to marshal metadata: %w", err)
	}

	now := time.Now().UTC()

	var scheduledAt *time.Time
	if !j.ScheduledAt.IsZero() {
		scheduledAt = &j.ScheduledAt
	}

	var uniqueKey *string
	if j.UniqueKey != "" {
		uniqueKey = &j.UniqueKey
	}

	query := `
		INSERT INTO jobs (id, type, payload, priority, status, attempts, max_retries, 
			created_at, scheduled_at, started_at, completed_at, error, result, metadata, unique_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			attempts = EXCLUDED.attempts,
			completed_at = EXCLUDED.completed_at,
			result = EXCLUDED.result,
			metadata = EXCLUDED.metadata
	`

	_, err = r.pool.Exec(ctx, query,
		j.ID, j.Type, j.Payload, j.Priority, "completed",
		j.Attempts, j.MaxRetries, j.CreatedAt, scheduledAt,
		j.UpdatedAt, now, nil, result, metadataJSON, uniqueKey,
	)
	if err != nil {
		return fmt.Errorf("repository save completed: %w", err)
	}

	return nil
}

// SaveFailed stores a failed job in the database.
func (r *JobRepository) SaveFailed(ctx context.Context, j *job.Job, errMsg string) error {
	metadataJSON, err := json.Marshal(j.Metadata)
	if err != nil {
		return fmt.Errorf("repository save failed: failed to marshal metadata: %w", err)
	}

	now := time.Now().UTC()
	status := "failed"
	if j.Status == job.StatusDead {
		status = "dead"
	}

	var scheduledAt *time.Time
	if !j.ScheduledAt.IsZero() {
		scheduledAt = &j.ScheduledAt
	}

	var uniqueKey *string
	if j.UniqueKey != "" {
		uniqueKey = &j.UniqueKey
	}

	query := `
		INSERT INTO jobs (id, type, payload, priority, status, attempts, max_retries, 
			created_at, scheduled_at, started_at, completed_at, error, result, metadata, unique_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			attempts = EXCLUDED.attempts,
			completed_at = EXCLUDED.completed_at,
			error = EXCLUDED.error,
			metadata = EXCLUDED.metadata
	`

	_, err = r.pool.Exec(ctx, query,
		j.ID, j.Type, j.Payload, j.Priority, status,
		j.Attempts, j.MaxRetries, j.CreatedAt, scheduledAt,
		j.UpdatedAt, now, errMsg, nil, metadataJSON, uniqueKey,
	)
	if err != nil {
		return fmt.Errorf("repository save failed: %w", err)
	}

	return nil
}

// GetByID retrieves a job by ID.
func (r *JobRepository) GetByID(ctx context.Context, id string) (*JobRecord, error) {
	query := `
		SELECT id, type, payload, priority, status, attempts, max_retries,
			created_at, scheduled_at, started_at, completed_at, error, result, metadata, unique_key
		FROM jobs WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanJob(row)
}

// ListFilter provides filtering options for listing jobs.
type ListFilter struct {
	Status    string
	Type      string
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}

// List retrieves jobs with optional filters.
func (r *JobRepository) List(ctx context.Context, filter ListFilter) ([]*JobRecord, error) {
	query := `
		SELECT id, type, payload, priority, status, attempts, max_retries,
			created_at, scheduled_at, started_at, completed_at, error, result, metadata, unique_key
		FROM jobs WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}

	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
		argIdx++
	}

	if filter.StartDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, filter.StartDate)
		argIdx++
	}

	if filter.EndDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, filter.EndDate)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repository list: %w", err)
	}
	defer rows.Close()

	var jobs []*JobRecord
	for rows.Next() {
		job, err := r.scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// JobStats holds job statistics.
type JobStats struct {
	TotalCount       int64          `json:"total_count"`
	StatusCounts     map[string]int64 `json:"status_counts"`
	TypeCounts       map[string]int64 `json:"type_counts"`
	AvgDurationMs    float64        `json:"avg_duration_ms"`
	SuccessRate      float64        `json:"success_rate"`
}

// GetStats retrieves job statistics.
func (r *JobRepository) GetStats(ctx context.Context) (*JobStats, error) {
	stats := &JobStats{
		StatusCounts: make(map[string]int64),
		TypeCounts:   make(map[string]int64),
	}

	// Get total count
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM jobs").Scan(&stats.TotalCount)
	if err != nil {
		return nil, fmt.Errorf("repository stats: %w", err)
	}

	// Get status counts
	rows, err := r.pool.Query(ctx, "SELECT status, COUNT(*) FROM jobs GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("repository stats: %w", err)
	}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("repository stats: %w", err)
		}
		stats.StatusCounts[status] = count
	}
	rows.Close()

	// Get type counts
	rows, err = r.pool.Query(ctx, "SELECT type, COUNT(*) FROM jobs GROUP BY type ORDER BY COUNT(*) DESC LIMIT 10")
	if err != nil {
		return nil, fmt.Errorf("repository stats: %w", err)
	}
	for rows.Next() {
		var jobType string
		var count int64
		if err := rows.Scan(&jobType, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("repository stats: %w", err)
		}
		stats.TypeCounts[jobType] = count
	}
	rows.Close()

	// Get average duration
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at)) * 1000), 0)
		FROM jobs WHERE completed_at IS NOT NULL AND started_at IS NOT NULL
	`).Scan(&stats.AvgDurationMs)
	if err != nil {
		return nil, fmt.Errorf("repository stats: %w", err)
	}

	// Calculate success rate
	completed := stats.StatusCounts["completed"]
	failed := stats.StatusCounts["failed"] + stats.StatusCounts["dead"]
	total := completed + failed
	if total > 0 {
		stats.SuccessRate = float64(completed) / float64(total) * 100
	}

	return stats, nil
}

// scanJob scans a job row into a JobRecord.
func (r *JobRepository) scanJob(row pgx.Row) (*JobRecord, error) {
	var j JobRecord
	var metadataJSON []byte

	err := row.Scan(
		&j.ID, &j.Type, &j.Payload, &j.Priority, &j.Status,
		&j.Attempts, &j.MaxRetries, &j.CreatedAt, &j.ScheduledAt,
		&j.StartedAt, &j.CompletedAt, &j.Error, &j.Result,
		&metadataJSON, &j.UniqueKey,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("job not found")
		}
		return nil, fmt.Errorf("repository scan: %w", err)
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &j.Metadata); err != nil {
			return nil, fmt.Errorf("repository scan: failed to unmarshal metadata: %w", err)
		}
	}

	return &j, nil
}
