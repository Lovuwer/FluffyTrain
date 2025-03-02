# Titan API Reference

Complete REST API documentation for the Titan job queue system.

## Base URL

```
http://localhost:8080
```

## Authentication

Currently no authentication is required. For production, add authentication middleware.

---

## Health Endpoints

### Liveness Check

```
GET /health
```

Simple check that the process is running. Always returns 200.

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Readiness Check

```
GET /ready
```

Checks connectivity to Redis and PostgreSQL. Returns 503 if any dependency is down.

**Response (Healthy):**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "components": [
    {"name": "redis", "status": "healthy", "latency": "1.234ms"},
    {"name": "postgres", "status": "healthy", "latency": "2.345ms"}
  ]
}
```

**Response (Unhealthy - 503):**
```json
{
  "status": "unhealthy",
  "version": "1.0.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "components": [
    {"name": "redis", "status": "unhealthy", "error": "connection refused"},
    {"name": "postgres", "status": "healthy", "latency": "2.345ms"}
  ]
}
```

---

## Job Endpoints

### Submit Job

```
POST /api/v1/jobs
```

Submit a new job for processing.

**Request Body:**
```json
{
  "type": "send_email",
  "payload": {"to": "user@example.com", "subject": "Hello"},
  "priority": 5,
  "max_retries": 5,
  "scheduled_at": "2024-01-15T12:00:00Z",
  "unique_key": "email-user@example.com-welcome",
  "metadata": {"trace_id": "abc123"}
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | Yes | - | Job handler type |
| `payload` | object | No | null | Job-specific data |
| `priority` | int | No | 5 | 1=low, 5=normal, 10=critical |
| `max_retries` | int | No | 5 | Max retry attempts |
| `scheduled_at` | string | No | now | ISO8601 timestamp |
| `unique_key` | string | No | - | Deduplication key |
| `metadata` | object | No | {} | Key-value metadata |

**Response (201):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "send_email",
  "status": "pending",
  "created_at": "2024-01-15T10:30:00Z"
}
```

**Response (400) - Validation Error:**
```json
{
  "error": "validation error",
  "details": "type is required"
}
```

**Response (409) - Duplicate Job:**
```json
{
  "error": "duplicate job",
  "details": "job with unique_key already exists",
  "existing_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

### Submit Batch Jobs

```
POST /api/v1/jobs/batch
```

Submit multiple jobs in a single request.

**Request Body:**
```json
{
  "jobs": [
    {"type": "send_email", "payload": {"to": "user1@example.com"}},
    {"type": "send_email", "payload": {"to": "user2@example.com"}},
    {"type": "process_image", "payload": {"url": "https://..."}}
  ]
}
```

**Response (201):**
```json
{
  "jobs": [
    {"id": "...", "type": "send_email", "status": "pending", "created_at": "..."},
    {"id": "...", "type": "send_email", "status": "pending", "created_at": "..."},
    {"id": "...", "type": "process_image", "status": "pending", "created_at": "..."}
  ],
  "created": 3
}
```

### Get Job

```
GET /api/v1/jobs/{id}
```

Get job details by ID.

**Response (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "send_email",
  "payload": {"to": "user@example.com"},
  "priority": 5,
  "status": "completed",
  "attempts": 1,
  "max_retries": 5,
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:05Z",
  "scheduled_at": "2024-01-15T10:30:00Z",
  "metadata": {"trace_id": "abc123"},
  "result": {"message_id": "xyz789"}
}
```

**Response (404):**
```json
{
  "error": "job not found",
  "details": "redis get: key not found"
}
```

### Delete Job

```
DELETE /api/v1/jobs/{id}
```

Cancel a pending job. Only works for jobs in `pending` status.

**Response (204):** No content

**Response (404):**
```json
{
  "error": "job not found",
  "details": "..."
}
```

**Response (409):**
```json
{
  "error": "cannot delete job",
  "details": "job is not pending"
}
```

### Get Job Result

```
GET /api/v1/jobs/{id}/result
```

Get the result of a completed job.

**Response (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "result": {"message_id": "xyz789"}
}
```

**Response (404):**
```json
{
  "error": "result not available",
  "details": "job is not completed"
}
```

---

## Queue Endpoints

### Get Queue Stats

```
GET /api/v1/queues/stats
```

Get statistics about all queues.

**Response (200):**
```json
{
  "pending_high": 5,
  "pending_normal": 42,
  "pending_low": 128,
  "processing": 10,
  "dead": 3
}
```

---

## Dead Letter Queue Endpoints

### List DLQ Jobs

```
GET /api/v1/dlq?offset=0&limit=20
```

List jobs in the dead letter queue.

**Query Parameters:**

| Parameter | Default | Max | Description |
|-----------|---------|-----|-------------|
| `offset` | 0 | - | Pagination offset |
| `limit` | 20 | 100 | Items per page |

**Response (200):**
```json
{
  "jobs": [
    {
      "id": "...",
      "type": "send_email",
      "status": "dead",
      "attempts": 5,
      "last_error": "connection refused",
      "created_at": "2024-01-15T10:30:00Z"
    }
  ],
  "offset": 0,
  "limit": 20,
  "count": 1
}
```

### Retry DLQ Job

```
POST /api/v1/dlq/{id}/retry
```

Move a job from DLQ back to pending queue.

**Response (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "message": "job has been re-queued"
}
```

---

## Error Response Format

All errors follow this format:

```json
{
  "error": "short error message",
  "details": "detailed error description"
}
```

## HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 204 | No Content (success with no body) |
| 400 | Bad Request (validation error) |
| 404 | Not Found |
| 409 | Conflict (e.g., job not pending) |
| 429 | Too Many Requests (rate limited) |
| 500 | Internal Server Error |
| 503 | Service Unavailable (dependency down) |
