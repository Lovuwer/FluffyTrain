# Titan Chaos Testing

This directory contains chaos testing scripts to verify Titan's fault tolerance.

## Prerequisites

- Docker Compose running (`cd deployments && docker-compose up -d`)
- `curl` and `jq` installed
- Bash shell

## Available Tests

### 1. Poison Pill (`poison_pill.sh`)

Submits jobs that intentionally crash handlers to verify panic recovery.

```bash
./scripts/chaos/poison_pill.sh
```

**Expected Behavior:**
- Worker doesn't crash
- Jobs move to DLQ after max retries
- Other jobs continue processing

### 2. Sudden Death (`sudden_death.sh`)

Kills random workers during job processing.

```bash
./scripts/chaos/sudden_death.sh
```

**Expected Behavior:**
- In-flight jobs recovered by watchdog
- Jobs reappear in pending queue
- No job loss

### 3. Thundering Herd (`thundering_herd.sh`)

Submits 10,000 jobs in ~1 second to test burst handling.

```bash
./scripts/chaos/thundering_herd.sh
```

**Expected Behavior:**
- All jobs eventually processed
- Rate limiting kicks in
- No OOM errors

### 4. Network Partition (`network_partition.sh`)

Blocks worker-to-Redis traffic temporarily.

```bash
./scripts/chaos/network_partition.sh
```

**Expected Behavior:**
- Circuit breaker opens
- Workers stop claiming jobs
- Resume when connection restored

### 5. Redis Restart (`redis_restart.sh`)

Restarts Redis during active processing.

```bash
./scripts/chaos/redis_restart.sh
```

**Expected Behavior:**
- Workers reconnect automatically
- Jobs in memory may be lost (expected)
- Processing resumes

### 6. Slow Consumer (`slow_consumer.sh`)

Uses sleep handler to simulate slow processing.

```bash
./scripts/chaos/slow_consumer.sh
```

**Expected Behavior:**
- Queue depth increases
- No timeout errors (if within limits)
- Eventually all jobs complete

## Running All Tests

```bash
cd scripts/chaos
for script in *.sh; do
  echo "Running $script..."
  ./$script
  echo ""
done
```

## Interpreting Results

Each script outputs:
- `PASS` - Test behaved as expected
- `FAIL` - Unexpected behavior detected
- Test-specific metrics

## Cleanup

If tests leave the system in a bad state:

```bash
cd deployments
docker-compose down
docker-compose up -d
```
