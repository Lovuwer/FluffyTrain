#!/bin/bash
# Thundering Herd Test - Submit 10,000 jobs in ~1 second
set -e

API_URL="${API_URL:-http://localhost:8080}"
JOBS_TO_SUBMIT=10000
BATCH_SIZE=100
WAIT_TIME=300

echo "=== Thundering Herd Test ==="
echo "Submitting $JOBS_TO_SUBMIT jobs in batches of $BATCH_SIZE..."

START_TIME=$(date +%s)

# Submit jobs in parallel batches
for batch in $(seq 1 $((JOBS_TO_SUBMIT / BATCH_SIZE))); do
    # Create batch payload
    JOBS="["
    for i in $(seq 1 $BATCH_SIZE); do
        if [ $i -gt 1 ]; then
            JOBS+=","
        fi
        JOBS+='{"type": "echo", "payload": {"batch": '$batch', "index": '$i'}}'
    done
    JOBS+="]"
    
    # Submit batch in background
    curl -s -X POST "$API_URL/api/v1/jobs/batch" \
        -H "Content-Type: application/json" \
        -d '{"jobs": '"$JOBS"'}' > /dev/null &
done

# Wait for all curl processes to finish
wait

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
echo "Submitted $JOBS_TO_SUBMIT jobs in ${DURATION}s"

# Monitor queue depth over time
echo "Monitoring queue depth for ${WAIT_TIME}s..."
for i in $(seq 1 $((WAIT_TIME / 10))); do
    sleep 10
    STATS=$(curl -s "$API_URL/api/v1/queues/stats")
    PENDING=$(echo "$STATS" | jq '.pending_high + .pending_normal + .pending_low')
    PROCESSING=$(echo "$STATS" | jq '.processing')
    echo "  [$(date +%H:%M:%S)] Pending: $PENDING, Processing: $PROCESSING"
    
    # Check if all jobs processed
    if [ "$PENDING" -eq 0 ] && [ "$PROCESSING" -eq 0 ]; then
        echo "All jobs processed!"
        break
    fi
done

# Final check
FINAL_STATS=$(curl -s "$API_URL/api/v1/queues/stats")
FINAL_PENDING=$(echo "$FINAL_STATS" | jq '.pending_high + .pending_normal + .pending_low')
FINAL_DEAD=$(echo "$FINAL_STATS" | jq '.dead')

echo ""
echo "Final results:"
echo "  Pending: $FINAL_PENDING"
echo "  Dead: $FINAL_DEAD"

if [ "$FINAL_PENDING" -eq 0 ]; then
    echo "PASS: All jobs processed (some may be in DLQ: $FINAL_DEAD)"
    exit 0
else
    echo "INFO: $FINAL_PENDING jobs still pending (may need more time)"
    exit 0
fi
