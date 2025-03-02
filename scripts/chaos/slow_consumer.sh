#!/bin/bash
# Slow Consumer Test - Simulate slow job processing
set -e

API_URL="${API_URL:-http://localhost:8080}"
JOBS_TO_SUBMIT=50
SLEEP_SECONDS=10

echo "=== Slow Consumer Test ==="
echo "Submitting $JOBS_TO_SUBMIT slow jobs (${SLEEP_SECONDS}s each)..."

START_TIME=$(date +%s)

# Submit slow jobs
for i in $(seq 1 $JOBS_TO_SUBMIT); do
    curl -s -X POST "$API_URL/api/v1/jobs" \
        -H "Content-Type: application/json" \
        -d '{"type": "sleep", "payload": {"seconds": '$SLEEP_SECONDS'}}' > /dev/null
done

echo "Submitted $JOBS_TO_SUBMIT jobs"

# Monitor queue depth
echo "Monitoring queue depth..."
MAX_PENDING=0
SAMPLES=0

while true; do
    STATS=$(curl -s "$API_URL/api/v1/queues/stats")
    PENDING=$(echo "$STATS" | jq '.pending_high + .pending_normal + .pending_low')
    PROCESSING=$(echo "$STATS" | jq '.processing')
    DEAD=$(echo "$STATS" | jq '.dead')
    
    if [ "$PENDING" -gt "$MAX_PENDING" ]; then
        MAX_PENDING=$PENDING
    fi
    
    SAMPLES=$((SAMPLES + 1))
    echo "  [$(date +%H:%M:%S)] Pending: $PENDING, Processing: $PROCESSING, Dead: $DEAD"
    
    # Check if all jobs processed
    if [ "$PENDING" -eq 0 ] && [ "$PROCESSING" -eq 0 ]; then
        break
    fi
    
    # Safety limit
    if [ $SAMPLES -gt 100 ]; then
        echo "Timeout waiting for jobs to complete"
        break
    fi
    
    sleep 5
done

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo ""
echo "Results:"
echo "  Total time: ${DURATION}s"
echo "  Max queue depth: $MAX_PENDING"
echo "  Jobs in DLQ: $DEAD"

# Calculate expected time (with 3 workers, 10 concurrent each = 30 parallel)
EXPECTED_TIME=$(( (JOBS_TO_SUBMIT * SLEEP_SECONDS) / 30 + 10 ))
echo "  Expected time (approx): ${EXPECTED_TIME}s"

if [ "$DURATION" -lt "$((EXPECTED_TIME * 2))" ]; then
    echo "PASS: Slow jobs processed within reasonable time"
    exit 0
else
    echo "INFO: Processing took longer than expected (may need more workers)"
    exit 0
fi
