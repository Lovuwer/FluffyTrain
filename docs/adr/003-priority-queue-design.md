# ADR-003: Priority Queue Design

## Status

Accepted

## Context

Jobs can have different priorities. We need to ensure high-priority jobs are processed before lower-priority ones.

Options considered:

1. **Single queue with sorting** - One sorted set, score = priority
2. **Multiple queues** - Separate list per priority level
3. **Weighted fair queuing** - Probabilistic selection based on priority

## Decision

Use multiple queues with priority-ordered polling.

## Rationale

### Why Multiple Queues

1. **Simplicity**: Just LPUSH/LMOVE on lists
2. **No sorting overhead**: O(1) operations
3. **Starvation control**: Can add fairness logic
4. **Atomic claims**: Each queue is independent

### Priority Levels

We use 3 priority buckets:

| Priority | Value | Use Case |
|----------|-------|----------|
| Critical | 10 | System alerts, payments |
| Normal | 5 | Regular jobs (default) |
| Low | 1 | Background tasks, reports |

Jobs with priority 6-10 go to critical queue, 2-5 to normal, 1 to low.

### Polling Order

Workers check queues in order:
1. `titan:queue:10:pending` (critical)
2. `titan:queue:5:pending` (normal)
3. `titan:queue:1:pending` (low)

If critical has jobs, they're always processed first.

## Consequences

### Positive

- High-priority jobs have minimal delay
- Simple to understand and debug
- No complex scheduling algorithms

### Negative

- **Starvation risk**: Low-priority jobs may starve under constant high-priority load
- **Granularity**: Only 3 priority levels

### Mitigations

For starvation, we could implement:
1. **Age-based promotion**: Promote old low-priority jobs
2. **Quota**: Process at least 1 low-priority every N jobs
3. **Separate workers**: Dedicate workers to low-priority

Currently not implemented as starvation is rarely an issue in practice.

## Implementation

```go
func PriorityQueues(prefix string) []string {
    return []string{
        prefix + ":queue:10:pending", // Critical
        prefix + ":queue:5:pending",  // Normal
        prefix + ":queue:1:pending",  // Low
    }
}

// Worker claims in priority order
for _, queueKey := range PriorityQueues(prefix) {
    job, err := claimFromQueue(ctx, queueKey)
    if job != nil {
        return job, nil
    }
}
```
