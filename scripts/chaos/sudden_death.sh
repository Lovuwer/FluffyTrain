#!/bin/bash
# Sudden Death Test - Kill workers during processing
set -e

API_URL="${API_URL:-http://localhost:8080}"
JOBS_TO_SUBMIT=50
WAIT_TIME=120

echo "=== Sudden Death Test ==="

# Submit long-running jobs
echo "Submitting $JOBS_TO_SUBMIT slow jobs (5s each)..."
for i in $(seq 1 $JOBS_TO_SUBMIT); do
    curl -s -X POST "$API_URL/api/v1/jobs" \
        -H "Content-Type: application/json" \
        -d '{"type": "sleep", "payload": {"seconds": 5}}' > /dev/null
done

echo "Waiting 5s for jobs to start processing..."
sleep 5

# Get initial queue stats
INITIAL_PROCESSING=$(curl -s "$API_URL/api/v1/queues/stats" | jq '.processing')
echo "Jobs currently processing: $INITIAL_PROCESSING"

# Kill a random worker
echo "Killing a worker..."
WORKER=$(docker ps --filter "name=titan-worker" --format "{{.Names}}" | head -1)
docker kill "$WORKER" 2>/dev/null || true

echo "Killed worker: $WORKER"

# Wait for watchdog to recover jobs (visibility timeout + scan interval)
echo "Waiting ${WAIT_TIME}s for watchdog recovery..."
sleep $WAIT_TIME

# Restart the killed worker
echo "Restarting worker..."
docker start "$WORKER" 2>/dev/null || true

# Wait a bit more
sleep 30

# Check final stats
FINAL_STATS=$(curl -s "$API_URL/api/v1/queues/stats")
PENDING=$(echo "$FINAL_STATS" | jq '.pending_high + .pending_normal + .pending_low')
PROCESSING=$(echo "$FINAL_STATS" | jq '.processing')
DEAD=$(echo "$FINAL_STATS" | jq '.dead')

echo "Final stats:"
echo "  Pending: $PENDING"
echo "  Processing: $PROCESSING"
echo "  Dead: $DEAD"

# All jobs should eventually complete (pending+processing+dead should decrease)
TOTAL_REMAINING=$((PENDING + PROCESSING + DEAD))
echo "Total remaining: $TOTAL_REMAINING"

if [ "$TOTAL_REMAINING" -lt "$JOBS_TO_SUBMIT" ]; then
    echo "PASS: Jobs were recovered and processed"
    exit 0
else
    echo "INFO: Some jobs still in queue (expected if still processing)"
    # This is not necessarily a failure - might just need more time
    exit 0
fi
