#!/bin/bash
# Poison Pill Test - Submit jobs that crash handlers
set -e

API_URL="${API_URL:-http://localhost:8080}"
JOBS_TO_SUBMIT=10
WAIT_TIME=60

echo "=== Poison Pill Test ==="
echo "Submitting $JOBS_TO_SUBMIT panic-inducing jobs..."

# Submit panic jobs
for i in $(seq 1 $JOBS_TO_SUBMIT); do
    curl -s -X POST "$API_URL/api/v1/jobs" \
        -H "Content-Type: application/json" \
        -d '{"type": "panic", "payload": {}, "max_retries": 3}' > /dev/null
done

echo "Submitted $JOBS_TO_SUBMIT panic jobs"

# Also submit some divide_by_zero jobs
for i in $(seq 1 $JOBS_TO_SUBMIT); do
    curl -s -X POST "$API_URL/api/v1/jobs" \
        -H "Content-Type: application/json" \
        -d '{"type": "divide_by_zero", "payload": {}, "max_retries": 2}' > /dev/null
done

echo "Submitted $JOBS_TO_SUBMIT divide_by_zero jobs"

# Wait for processing
echo "Waiting ${WAIT_TIME}s for processing..."
sleep $WAIT_TIME

# Check DLQ
DLQ_COUNT=$(curl -s "$API_URL/api/v1/dlq" | jq '.count')
echo "Jobs in DLQ: $DLQ_COUNT"

# Check if workers are still running
WORKERS_RUNNING=$(docker ps --filter "name=titan-worker" --format "{{.Names}}" | wc -l)
echo "Workers still running: $WORKERS_RUNNING"

# Verify
if [ "$DLQ_COUNT" -ge "$((JOBS_TO_SUBMIT * 2))" ] && [ "$WORKERS_RUNNING" -ge 1 ]; then
    echo "PASS: All poison pills moved to DLQ, workers survived"
    exit 0
else
    echo "FAIL: Expected all jobs in DLQ and workers running"
    echo "DLQ count: $DLQ_COUNT (expected >= $((JOBS_TO_SUBMIT * 2)))"
    echo "Workers: $WORKERS_RUNNING (expected >= 1)"
    exit 1
fi
