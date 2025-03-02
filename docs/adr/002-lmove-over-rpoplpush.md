# ADR-002: LMOVE over RPOPLPUSH

## Status

Accepted

## Context

We need to atomically move jobs from pending to processing queue. Redis offers two commands:

1. `RPOPLPUSH` - Deprecated since Redis 6.2
2. `LMOVE` - Replacement for RPOPLPUSH with more options

## Decision

Use `LMOVE` and `BLMOVE` for all queue operations.

## Rationale

### Why LMOVE

1. **Future-proof**: RPOPLPUSH is deprecated
2. **Flexibility**: Supports LEFT/RIGHT for both source and destination
3. **Blocking variant**: BLMOVE allows workers to wait for jobs
4. **Redis 7.x**: Our docker-compose uses Redis 7.4

### Command Comparison

```
# Old way (deprecated)
RPOPLPUSH source destination

# New way
LMOVE source destination RIGHT LEFT
BLMOVE source destination RIGHT LEFT timeout
```

The new syntax is more explicit about direction.

## Consequences

### Positive

- No deprecation warnings in logs
- More control over queue direction
- BLMOVE reduces polling overhead

### Negative

- Requires Redis 6.2+
- Older clients may not support LMOVE

### Implementation

```go
// Non-blocking claim attempt
jobID, err := client.LMove(ctx, pendingKey, processingKey, "RIGHT", "LEFT")

// Blocking claim with timeout
jobID, err := client.BLMove(ctx, pendingKey, processingKey, "RIGHT", "LEFT", 5*time.Second)
```
