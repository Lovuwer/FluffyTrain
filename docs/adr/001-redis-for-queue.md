# ADR-001: Redis for Queue Backend

## Status

Accepted

## Context

We need a message broker for the Titan job queue. Options considered:

1. **Redis** - In-memory data store with list/sorted set operations
2. **RabbitMQ** - Traditional message queue with AMQP
3. **Apache Kafka** - Distributed streaming platform
4. **PostgreSQL** - Using LISTEN/NOTIFY or polling
5. **Amazon SQS** - Managed queue service

## Decision

We chose Redis as the queue backend.

## Rationale

### Why Redis

1. **Atomic Operations**: LMOVE provides atomic job claim
2. **Speed**: Sub-millisecond operations for queue ops
3. **Data Structures**: Lists for FIFO, sorted sets for scheduling
4. **Simplicity**: Single dependency for queue + locks + dedup
5. **Familiarity**: Well-understood in the Go ecosystem
6. **Persistence**: AOF provides durability when needed

### Why Not Others

**RabbitMQ**:
- More complex to operate
- Overkill for simple job queue pattern
- Additional infrastructure component

**Kafka**:
- Designed for streaming, not job queues
- Consumer groups don't fit our model
- Heavier infrastructure

**PostgreSQL**:
- Polling is inefficient
- LISTEN/NOTIFY loses messages on disconnect
- Contention under high throughput

**SQS**:
- Vendor lock-in
- Higher latency (~10-20ms)
- Cost at scale

## Consequences

### Positive

- Single Redis instance handles 100K+ ops/sec
- Lua scripts enable complex atomic operations
- Built-in TTL for locks and dedup keys
- Pub/sub available for future real-time features

### Negative

- Single point of failure (mitigated by Redis Sentinel/Cluster)
- Memory-bound (job data stored in RAM)
- No built-in dead letter queue (we implement it)

### Mitigations

- Use Redis Sentinel for HA
- Store only job IDs in queues, not payloads
- Implement watchdog for orphan detection
- Write completed jobs to PostgreSQL for history
