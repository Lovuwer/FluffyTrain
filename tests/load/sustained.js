// Titan Sustained Load Test
// Verify: 200 jobs/sec for 5 minutes
// Run with: k6 run tests/load/sustained.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const jobSubmitDuration = new Trend('job_submit_duration');
const jobsSubmitted = new Counter('jobs_submitted');
const jobsFailed = new Counter('jobs_failed');

const API_URL = __ENV.API_URL || 'http://localhost:8080';

export const options = {
  scenarios: {
    sustained_load: {
      executor: 'constant-arrival-rate',
      rate: 200,              // 200 requests per second
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 100,
      maxVUs: 200,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<100'],  // 95% of requests under 100ms
    http_req_failed: ['rate<0.01'],    // Less than 1% failures
    job_submit_duration: ['p(95)<50'], // 95% of submissions under 50ms
  },
};

const jobTypes = ['echo', 'sleep', 'http_request', 'email_mock'];

export default function () {
  const jobType = jobTypes[Math.floor(Math.random() * jobTypes.length)];
  
  const payload = JSON.stringify({
    type: jobType,
    payload: getPayload(jobType),
    priority: Math.floor(Math.random() * 10) + 1,
  });

  const start = Date.now();
  
  const res = http.post(`${API_URL}/api/v1/jobs`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  const duration = Date.now() - start;
  jobSubmitDuration.add(duration);

  const success = check(res, {
    'status is 201': (r) => r.status === 201,
    'has job id': (r) => JSON.parse(r.body).id !== undefined,
  });

  if (success) {
    jobsSubmitted.add(1);
  } else {
    jobsFailed.add(1);
  }
}

function getPayload(jobType) {
  switch (jobType) {
    case 'sleep':
      return { seconds: 1 };
    case 'email_mock':
      return { to: `user${Math.random()}@example.com`, subject: 'Load test' };
    case 'http_request':
      return { url: 'https://example.com/api', method: 'GET' };
    default:
      return { message: `Test message ${Date.now()}` };
  }
}

export function handleSummary(data) {
  return {
    'tests/load/results/sustained-summary.json': JSON.stringify(data, null, 2),
  };
}
