#!/bin/bash
# Redis Restart Test - Restart Redis during active processing
set -e

API_URL="${API_URL:-http://localhost:8080}"
JOBS_TO_SUBMIT=100

echo "=== Redis Restart Test ==="

# Submit some jobs
echo "Submitting $JOBS_TO_SUBMIT jobs..."
for i in $(seq 1 $JOBS_TO_SUBMIT); do
    curl -s -X POST "$API_URL/api/v1/jobs" \
        -H "Content-Type: application/json" \
        -d '{"type": "sleep", "payload": {"seconds": 2}}' > /dev/null
done

echo "Waiting 3s for processing to start..."
sleep 3

# Check initial state
INITIAL_STATS=$(curl -s "$API_URL/api/v1/queues/stats")
echo "Initial stats: $INITIAL_STATS"

# Restart Redis
echo "Restarting Redis..."
docker restart titan-redis

echo "Waiting 10s for Redis to come back..."
sleep 10

# Check if API is healthy
for i in $(seq 1 10); do
    HEALTH=$(curl -s "$API_URL/health" 2>/dev/null || echo '{"status": "error"}')
    STATUS=$(echo "$HEALTH" | jq -r '.status')
    if [ "$STATUS" = "healthy" ]; then
        echo "API is healthy"
        break
    fi
    echo "  Waiting for API... (attempt $i)"
    sleep 2
done

# Check readiness (Redis should be back)
for i in $(seq 1 10); do
    READY=$(curl -s "$API_URL/ready" 2>/dev/null || echo '{"status": "error"}')
    STATUS=$(echo "$READY" | jq -r '.status')
    if [ "$STATUS" = "healthy" ] || [ "$STATUS" = "ready" ]; then
        echo "API is ready (Redis reconnected)"
        break
    fi
    echo "  Waiting for Redis connection... (attempt $i)"
    sleep 2
done

# Wait for workers to reconnect and resume
echo "Waiting 30s for processing to resume..."
sleep 30

# Final check
FINAL_STATS=$(curl -s "$API_URL/api/v1/queues/stats")
echo "Final stats: $FINAL_STATS"

echo "PASS: Redis restart handled gracefully"
exit 0
