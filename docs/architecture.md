# Titan Architecture

## Overview

Titan is a distributed job queue system designed for high reliability and performance. It uses Redis as the primary message broker and PostgreSQL for job history/audit logging.

## Components

### 1. API Server (`cmd/api`)

The API server is the entry point for job submission. It provides a REST API for:
- Submitting jobs (single and batch)
- Querying job status
- Managing the Dead Letter Queue
- Health and readiness checks

**Key characteristics:**
- Stateless (can run multiple instances behind a load balancer)
- Graceful shutdown with request draining
- Rate limiting per client
- Request ID propagation for tracing

### 2. Worker (`cmd/worker`)

Workers consume jobs from the queue and execute handlers. Each worker:
- Runs N concurrent goroutines (configurable)
- Claims jobs atomically using LMOVE
- Implements panic recovery per job
- Reports heartbeats for watchdog
- Shuts down gracefully (completes in-flight jobs)

### 3. Queue (`internal/queue`)

The queue layer manages job lifecycle using Redis:

```
                    ┌─────────────────────┐
                    │   Scheduled Jobs    │
                    │   (Sorted Set)      │
                    └──────────┬──────────┘
                               │ (when due)
                               ▼
┌──────────────────────────────────────────────────────────┐
│                    Pending Queues                         │
│  ┌────────────┐  ┌────────────┐  ┌────────────────┐      │
│  │ Priority 10│  │ Priority 5 │  │ Priority 1     │      │
│  │ (Critical) │  │ (Normal)   │  │ (Low)          │      │
│  └─────┬──────┘  └─────┬──────┘  └───────┬────────┘      │
│        │               │                  │               │
└────────┴───────────────┴──────────────────┴───────────────┘
                         │
                         │ LMOVE (atomic)
                         ▼
                ┌────────────────────┐
                │  Processing Queue  │
                └─────────┬──────────┘
                          │
            ┌─────────────┼─────────────┐
            │             │             │
            ▼             ▼             ▼
         ┌──────┐     ┌──────┐     ┌──────┐
         │ ACK  │     │ NACK │     │Timeout│
         └──┬───┘     └──┬───┘     └───┬───┘
            │            │             │
            ▼            ▼             ▼
       ┌─────────┐  ┌─────────┐  ┌─────────┐
       │Completed│  │ Retry/  │  │ Retry/  │
       │         │  │  DLQ    │  │  DLQ    │
       └─────────┘  └─────────┘  └─────────┘
```

### 4. Watchdog (`internal/watchdog`)

The watchdog runs as a goroutine in workers and:
- Uses leader election (only one runs across cluster)
- Scans for stuck jobs (exceeded visibility timeout)
- Re-queues orphaned jobs
- Increments attempt counter (recovery counts as failure)

### 5. Scheduler (`internal/scheduler`)

Handles delayed/scheduled jobs:
- Jobs with `scheduled_at` go to a Redis sorted set (score = Unix timestamp)
- Leader polls every second for due jobs
- Moves due jobs to appropriate priority queue

## Data Flow

### Job Submission

```
1. Client POST /api/v1/jobs
2. Validate request
3. Check deduplication (if unique_key set)
4. Create Job object with UUID
5. Serialize to JSON
6. Store job data in Redis (titan:job:{id})
7. Push job ID to priority queue (titan:queue:{priority}:pending)
8. Return job ID to client
```

### Job Processing

```
1. Worker checks Redis health
2. Claim job: LMOVE from pending to processing
3. Set processing lock (titan:job:{id}:lock)
4. Track in sorted set (titan:processing:timestamps)
5. Look up handler for job type
6. Execute handler with timeout context
7. On success:
   - ACK: Remove from processing, update status
   - Store result in job data
   - Write to PostgreSQL
8. On failure:
   - NACK: Increment attempts
   - If attempts < max_retries: Re-queue with backoff
   - Else: Move to DLQ
```

### Retry Mechanism

Exponential backoff with jitter:
```
delay = min(1s * 2^attempt, 1hour) ± 10%
```

Example progression:
- Attempt 1: ~1s
- Attempt 2: ~2s
- Attempt 3: ~4s
- Attempt 4: ~8s
- Attempt 5: ~16s

## Redis Key Structure

| Key Pattern | Type | Description |
|-------------|------|-------------|
| `titan:queue:10:pending` | List | High priority pending jobs |
| `titan:queue:5:pending` | List | Normal priority pending jobs |
| `titan:queue:1:pending` | List | Low priority pending jobs |
| `titan:queue:processing` | List | Jobs being processed |
| `titan:queue:dead` | List | Dead letter queue |
| `titan:job:{id}` | String | Job data (JSON) |
| `titan:job:{id}:lock` | String | Processing lock with TTL |
| `titan:processing:timestamps` | Sorted Set | Job claim times for watchdog |
| `titan:scheduled` | Sorted Set | Scheduled jobs (score = run time) |
| `titan:dedup:{unique_key}` | String | Deduplication key |
| `titan:ratelimit:{key}` | Sorted Set | Rate limit tracking |
| `titan:watchdog:leader` | String | Watchdog leader lock |
| `titan:scheduler:leader` | String | Scheduler leader lock |

## Reliability Guarantees

### At-Least-Once Delivery

Jobs are guaranteed to be processed at least once. In failure scenarios:
- Worker crash: Watchdog re-queues after visibility timeout
- Redis connection loss: Circuit breaker opens, workers stop claiming
- Handler panic: Caught and converted to NACK

### Ordering

Within the same priority level, jobs are processed FIFO. However:
- Higher priority jobs always process first
- Concurrent workers may process out of order
- Retried jobs go to the back of the queue

### Durability

- Job data persists in Redis until explicitly deleted
- Completed/failed jobs written to PostgreSQL for audit
- Redis AOF persistence recommended for production

## Scaling

### Horizontal Scaling

| Component | Scaling Approach |
|-----------|------------------|
| API | Add instances behind load balancer |
| Workers | Add instances (all connect to same Redis) |
| Redis | Cluster mode for high throughput |
| PostgreSQL | Read replicas for reporting |

### Bottlenecks

1. **Redis**: Single-threaded, ~100K ops/sec
2. **Worker concurrency**: CPU/memory per job
3. **Handler latency**: External service calls

### Recommendations

- Start with 3-5 workers for most workloads
- Tune `TITAN_WORKER_CONCURRENCY` based on job type
- Use connection pooling for external services
- Monitor queue depth as capacity indicator
