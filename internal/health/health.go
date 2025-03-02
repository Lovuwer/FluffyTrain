// Package health provides health check functionality for the Titan job queue system.
package health

import (
	"context"
	"sync"
	"time"
)

// Version is the application version.
const Version = "1.0.0"

// Status represents the overall health status.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// ComponentStatus represents the health status of a single component.
type ComponentStatus struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HealthResponse is the response for health check endpoints.
type HealthResponse struct {
	Status     Status            `json:"status"`
	Version    string            `json:"version"`
	Timestamp  string            `json:"timestamp"`
	Components []ComponentStatus `json:"components,omitempty"`
	WorkerInfo *WorkerInfo       `json:"worker,omitempty"`
}

// WorkerInfo contains worker-specific health information.
type WorkerInfo struct {
	ID              string `json:"id"`
	ProcessingCount int    `json:"processing_count"`
	IsLeader        bool   `json:"is_leader"`
}

// Checker is a function that checks the health of a component.
type Checker func(ctx context.Context) ComponentStatus

// Check performs health checks on all registered checkers.
type Check struct {
	checkers []Checker
	mu       sync.RWMutex
}

// NewCheck creates a new health check.
func NewCheck() *Check {
	return &Check{
		checkers: make([]Checker, 0),
	}
}

// Register adds a health checker.
func (c *Check) Register(checker Checker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkers = append(c.checkers, checker)
}

// Liveness returns a simple liveness response (always healthy if process is running).
func (c *Check) Liveness() HealthResponse {
	return HealthResponse{
		Status:    StatusHealthy,
		Version:   Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// Readiness checks all registered components and returns overall status.
func (c *Check) Readiness(ctx context.Context) HealthResponse {
	c.mu.RLock()
	checkers := make([]Checker, len(c.checkers))
	copy(checkers, c.checkers)
	c.mu.RUnlock()

	// Use a timeout context for all checks
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Run all checks concurrently
	var wg sync.WaitGroup
	results := make([]ComponentStatus, len(checkers))

	for i, checker := range checkers {
		wg.Add(1)
		go func(idx int, check Checker) {
			defer wg.Done()
			results[idx] = check(checkCtx)
		}(i, checker)
	}

	wg.Wait()

	// Determine overall status
	overallStatus := StatusHealthy
	for _, result := range results {
		if result.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
			break
		}
		if result.Status == StatusDegraded {
			overallStatus = StatusDegraded
		}
	}

	return HealthResponse{
		Status:     overallStatus,
		Version:    Version,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Components: results,
	}
}

// ReadinessWithWorkerInfo checks readiness and includes worker info.
func (c *Check) ReadinessWithWorkerInfo(ctx context.Context, workerInfo *WorkerInfo) HealthResponse {
	resp := c.Readiness(ctx)
	resp.WorkerInfo = workerInfo
	return resp
}
