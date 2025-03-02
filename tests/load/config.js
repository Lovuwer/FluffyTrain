// Titan Load Test Configuration
// Run with: k6 run tests/load/sustained.js

export const config = {
  apiUrl: __ENV.API_URL || 'http://localhost:8080',
  
  // Sustained load test
  sustained: {
    vus: 50,           // Virtual users
    duration: '5m',    // Test duration
    rps: 200,          // Target requests per second
  },
  
  // Spike test
  spike: {
    stages: [
      { duration: '10s', target: 0 },
      { duration: '1s', target: 500 },   // Spike to 500 VUs
      { duration: '1m', target: 500 },
      { duration: '10s', target: 0 },
    ],
  },
  
  // Soak test
  soak: {
    vus: 20,
    duration: '1h',
    rps: 50,
  },
  
  // Thresholds
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<500'],
    http_req_failed: ['rate<0.01'],
    job_submit_duration: ['p(95)<50'],
  },
  
  // Job types for mixed workload
  jobTypes: [
    { type: 'echo', weight: 50 },
    { type: 'sleep', weight: 30, payload: { seconds: 1 } },
    { type: 'http_request', weight: 15 },
    { type: 'email_mock', weight: 5, payload: { to: 'test@example.com' } },
  ],
};

export function selectJobType() {
  const rand = Math.random() * 100;
  let cumulative = 0;
  
  for (const jt of config.jobTypes) {
    cumulative += jt.weight;
    if (rand < cumulative) {
      return jt;
    }
  }
  
  return config.jobTypes[0];
}
