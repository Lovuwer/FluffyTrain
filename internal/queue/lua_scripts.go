// Package queue implements a reliable job queue using Redis.
package queue

// Lua scripts for atomic queue operations.
// These scripts ensure atomicity of multi-step operations.

// claimJobScript atomically moves a job from pending to processing queue
// and sets a processing lock with TTL.
// KEYS[1] = pending queue key
// KEYS[2] = processing queue key  
// KEYS[3] = job lock key prefix
// KEYS[4] = processing timestamps sorted set
// ARGV[1] = lock TTL in seconds
// ARGV[2] = current timestamp
// Returns: job ID or nil if no job available
const claimJobScript = `
local jobID = redis.call('LMOVE', KEYS[1], KEYS[2], 'RIGHT', 'LEFT')
if jobID then
    -- Set processing lock with TTL
    redis.call('SET', KEYS[3] .. jobID, ARGV[2], 'EX', ARGV[1])
    -- Track processing start time in sorted set
    redis.call('ZADD', KEYS[4], ARGV[2], jobID)
    return jobID
end
return nil
`

// ackJobScript atomically removes a job from processing queue,
// removes the lock, and cleans up tracking.
// KEYS[1] = processing queue key
// KEYS[2] = job lock key
// KEYS[3] = processing timestamps sorted set
// ARGV[1] = job ID
// Returns: 1 if successful, 0 if job not found
const ackJobScript = `
local removed = redis.call('LREM', KEYS[1], 1, ARGV[1])
if removed > 0 then
    redis.call('DEL', KEYS[2])
    redis.call('ZREM', KEYS[3], ARGV[1])
    return 1
end
return 0
`

// nackJobScript atomically handles job failure:
// - Removes from processing queue
// - Either re-queues to pending (for retry) or moves to DLQ
// KEYS[1] = processing queue key
// KEYS[2] = pending queue key (for retry) or DLQ key (if max retries exceeded)
// KEYS[3] = job lock key
// KEYS[4] = processing timestamps sorted set
// ARGV[1] = job ID
// ARGV[2] = "retry" or "dlq"
// Returns: 1 if successful, 0 if job not found
const nackJobScript = `
local removed = redis.call('LREM', KEYS[1], 1, ARGV[1])
if removed > 0 then
    redis.call('DEL', KEYS[3])
    redis.call('ZREM', KEYS[4], ARGV[1])
    if ARGV[2] == "retry" then
        -- Re-queue to pending (at the end for fairness)
        redis.call('LPUSH', KEYS[2], ARGV[1])
    else
        -- Move to DLQ
        redis.call('LPUSH', KEYS[2], ARGV[1])
    end
    return 1
end
return 0
`

// recoverStuckJobsScript finds and recovers jobs stuck in processing.
// KEYS[1] = processing timestamps sorted set
// KEYS[2] = processing queue key
// ARGV[1] = current timestamp
// ARGV[2] = visibility timeout in seconds
// ARGV[3] = max jobs to recover
// Returns: list of recovered job IDs
const recoverStuckJobsScript = `
local threshold = tonumber(ARGV[1]) - tonumber(ARGV[2])
local stuckJobs = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', threshold, 'LIMIT', 0, ARGV[3])
local recovered = {}
for i, jobID in ipairs(stuckJobs) do
    -- Remove from sorted set
    redis.call('ZREM', KEYS[1], jobID)
    table.insert(recovered, jobID)
end
return recovered
`

// deduplicationCheckScript checks if a job with the same unique key already exists.
// KEYS[1] = dedup key
// ARGV[1] = TTL for dedup key
// Returns: 1 if this is a new unique job, 0 if duplicate
const deduplicationCheckScript = `
local exists = redis.call('EXISTS', KEYS[1])
if exists == 1 then
    return 0
end
redis.call('SET', KEYS[1], '1', 'EX', ARGV[1])
return 1
`

// Scripts holds pre-loaded Lua script SHAs for efficient execution.
type Scripts struct {
	ClaimJob        string
	AckJob          string
	NackJob         string
	RecoverStuck    string
	DeduplicationCheck string
}
