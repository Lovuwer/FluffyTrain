// Package handlers provides HTTP handlers for the Titan API.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/joek3softwares-boop/titan/internal/job"
	"github.com/joek3softwares-boop/titan/internal/logging"
	"github.com/joek3softwares-boop/titan/internal/queue"
)

// Health returns a health check handler.
func Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"version": "1.0.0",
	})
}

// Ready returns a readiness check handler.
func Ready(q queue.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check queue connectivity
		_, err := q.GetStats(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"status": "not ready",
				"error":  "queue unavailable",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "ready",
		})
	}
}

// CreateJobRequest is the request body for creating a job.
type CreateJobRequest struct {
	Type        string            `json:"type"`
	Payload     json.RawMessage   `json:"payload"`
	Priority    int               `json:"priority,omitempty"`
	ScheduledAt string            `json:"scheduled_at,omitempty"`
	MaxRetries  int               `json:"max_retries,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	UniqueKey   string            `json:"unique_key,omitempty"`
}

// CreateJobResponse is the response for creating a job.
type CreateJobResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// CreateJob returns a handler for creating jobs.
func CreateJob(q queue.Queue, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
			return
		}

		// Validate request
		if req.Type == "" {
			writeError(w, http.StatusBadRequest, "validation error", "type is required")
			return
		}

		// Create job
		j := job.NewJob(req.Type, req.Payload)
		if req.Priority > 0 {
			j.Priority = req.Priority
		}
		if req.MaxRetries > 0 {
			j.MaxRetries = req.MaxRetries
		}
		if req.Metadata != nil {
			j.Metadata = req.Metadata
		}
		if req.UniqueKey != "" {
			j.UniqueKey = req.UniqueKey
		}

		// Enqueue
		if err := q.Enqueue(r.Context(), j); err != nil {
			requestID := logging.GetRequestID(r.Context())
			logger.Error("failed to enqueue job",
				"error", err,
				"job_type", req.Type,
				"request_id", requestID,
			)
			writeError(w, http.StatusInternalServerError, "failed to enqueue job", err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, CreateJobResponse{
			ID:        j.ID,
			Type:      j.Type,
			Status:    j.Status.String(),
			CreatedAt: j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}

// CreateJobBatchRequest is the request body for creating multiple jobs.
type CreateJobBatchRequest struct {
	Jobs []CreateJobRequest `json:"jobs"`
}

// CreateJobBatch returns a handler for creating multiple jobs.
func CreateJobBatch(q queue.Queue, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateJobBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
			return
		}

		if len(req.Jobs) == 0 {
			writeError(w, http.StatusBadRequest, "validation error", "jobs array is required")
			return
		}

		responses := make([]CreateJobResponse, 0, len(req.Jobs))
		for _, jreq := range req.Jobs {
			if jreq.Type == "" {
				continue // Skip invalid jobs
			}

			j := job.NewJob(jreq.Type, jreq.Payload)
			if jreq.Priority > 0 {
				j.Priority = jreq.Priority
			}
			if jreq.MaxRetries > 0 {
				j.MaxRetries = jreq.MaxRetries
			}
			if jreq.Metadata != nil {
				j.Metadata = jreq.Metadata
			}
			if jreq.UniqueKey != "" {
				j.UniqueKey = jreq.UniqueKey
			}

			if err := q.Enqueue(r.Context(), j); err != nil {
				logger.Error("failed to enqueue batch job",
					"error", err,
					"job_type", jreq.Type,
				)
				continue
			}

			responses = append(responses, CreateJobResponse{
				ID:        j.ID,
				Type:      j.Type,
				Status:    j.Status.String(),
				CreatedAt: j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"jobs":    responses,
			"created": len(responses),
		})
	}
}

// GetJob returns a handler for getting a job.
func GetJob(q queue.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "validation error", "id is required")
			return
		}

		j, err := q.GetJob(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "job not found", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, j)
	}
}

// DeleteJob returns a handler for deleting a job.
func DeleteJob(q queue.Queue, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "validation error", "id is required")
			return
		}

		// Check job exists and is pending
		j, err := q.GetJob(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "job not found", err.Error())
			return
		}

		if j.Status != job.StatusPending {
			writeError(w, http.StatusConflict, "cannot delete job", "job is not pending")
			return
		}

		if err := q.DeleteJob(r.Context(), id); err != nil {
			logger.Error("failed to delete job", "error", err, "job_id", id)
			writeError(w, http.StatusInternalServerError, "failed to delete job", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// GetJobResult returns a handler for getting job result.
func GetJobResult(q queue.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "validation error", "id is required")
			return
		}

		j, err := q.GetJob(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "job not found", err.Error())
			return
		}

		if j.Status != job.StatusCompleted {
			writeError(w, http.StatusNotFound, "result not available", "job is not completed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":     j.ID,
			"status": j.Status.String(),
			"result": json.RawMessage(j.Result),
		})
	}
}

// GetQueueStats returns a handler for getting queue statistics.
func GetQueueStats(q queue.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := q.GetStats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get stats", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, stats)
	}
}

// ListDLQ returns a handler for listing dead letter queue jobs.
func ListDLQ(q queue.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		offsetStr := r.URL.Query().Get("offset")
		limitStr := r.URL.Query().Get("limit")

		offset := int64(0)
		limit := int64(20)

		if offsetStr != "" {
			if v, err := strconv.ParseInt(offsetStr, 10, 64); err == nil {
				offset = v
			}
		}
		if limitStr != "" {
			if v, err := strconv.ParseInt(limitStr, 10, 64); err == nil && v > 0 && v <= 100 {
				limit = v
			}
		}

		jobs, err := q.ListDLQ(r.Context(), offset, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list DLQ", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"jobs":   jobs,
			"offset": offset,
			"limit":  limit,
			"count":  len(jobs),
		})
	}
}

// RetryDLQJob returns a handler for retrying a DLQ job.
func RetryDLQJob(q queue.Queue, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "validation error", "id is required")
			return
		}

		if err := q.RetryDLQJob(r.Context(), id); err != nil {
			logger.Error("failed to retry DLQ job", "error", err, "job_id", id)
			writeError(w, http.StatusInternalServerError, "failed to retry job", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":      id,
			"status":  "pending",
			"message": "job has been re-queued",
		})
	}
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"details": details,
	})
}
