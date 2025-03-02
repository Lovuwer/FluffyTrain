// Titan Spike Test
// Verify: 0 -> 500 jobs/sec instantly
// Run with: k6 run tests/load/spike.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const jobSubmitDuration = new Trend('job_submit_duration');
const jobsSubmitted = new Counter('jobs_submitted');

const API_URL = __ENV.API_URL || 'http://localhost:8080';

export const options = {
  scenarios: {
    spike_test: {
      executor: 'ramping-arrival-rate',
      startRate: 0,
      timeUnit: '1s',
      preAllocatedVUs: 200,
      maxVUs: 500,
      stages: [
        { duration: '10s', target: 0 },      // Warm up
        { duration: '1s', target: 500 },     // Spike!
        { duration: '1m', target: 500 },     // Sustained spike
        { duration: '10s', target: 0 },      // Cool down
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],   // Allow higher latency during spike
    http_req_failed: ['rate<0.05'],     // Allow up to 5% failures during spike
  },
};

export default function () {
  const payload = JSON.stringify({
    type: 'echo',
    payload: { message: `Spike test ${Date.now()}` },
  });

  const start = Date.now();
  
  const res = http.post(`${API_URL}/api/v1/jobs`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  jobSubmitDuration.add(Date.now() - start);

  const success = check(res, {
    'status is 201': (r) => r.status === 201,
  });

  if (success) {
    jobsSubmitted.add(1);
  }
}

export function handleSummary(data) {
  return {
    'tests/load/results/spike-summary.json': JSON.stringify(data, null, 2),
  };
}
