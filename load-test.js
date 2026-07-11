import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const registrationRate = new Rate('registration_success_rate');
const loginRate = new Rate('login_success_rate');
const requestDuration = new Trend('request_duration');
const totalRequests = new Counter('total_requests');

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TARGET_RPS = __ENV.TARGET_RPS || 100;

// Pre-generated user pool (created during setup)
const users = new SharedArray('users', function () {
  const arr = [];
  for (let i = 0; i < 200; i++) {
    arr.push({
      email: `loaduser_${i}_${__VU}@test.com`,
      password: 'LoadTest123!',
      full_name: `Load User ${i}`,
    });
  }
  return arr;
});

// Staged ramp-up:
// 0s - start
// 30s - ramp up to target
// 1m - hold at target
// 30s - spike to 2x target
// 30s - hold spike
// 30s - ramp down
export const options = {
  stages: [
    { duration: '30s', target: Math.min(TARGET_RPS, 50) },
    { duration: '1m', target: Math.min(TARGET_RPS, 50) },
    { duration: '30s', target: Math.min(TARGET_RPS * 2, 200) },
    { duration: '30s', target: Math.min(TARGET_RPS * 2, 200) },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000', 'p(99)<5000'],
    http_req_failed: ['rate<0.10'],
    login_success_rate: ['rate>0.80'],
  },
  noVUConnectionReuse: true,
};

// Common headers
const headers = { 'Content-Type': 'application/json', 'X-Device-ID': 'k6-load-test' };

// Store authenticated tokens per VU
const authTokens = {};

function randomUser() {
  return users[Math.floor(Math.random() * users.length)];
}

export default function () {
  const vu = __VU;
  const iter = __ITER;

  // Phase 1: Register (20% of iterations)
  if (iter % 5 === 0) {
    const user = randomUser();
    const uniqueEmail = `${iter}_${vu}_${Date.now()}@loadtest.com`;
    const registerPayload = JSON.stringify({
      email: uniqueEmail,
      password: 'LoadTestPass123!',
      full_name: `Load User ${vu}_${iter}`,
    });

    const r1 = http.post(`${BASE_URL}/api/v1/auth/register`, registerPayload, { headers });
    check(r1, { 'register succeeded': (r) => r.status === 201 || r.status === 409 });
    registrationRate.add(r1.status === 201);
    requestDuration.add(r1.timings.duration);
    totalRequests.add(1);

    if (r1.status === 201) {
      authTokens[uniqueEmail] = {
        access: r1.json('access_token'),
        refresh: r1.json('refresh_token'),
      };
    }
  }

  // Phase 2: Login (40% of iterations)
  if (iter % 5 === 1 || iter % 5 === 2) {
    // Use admin credentials for guaranteed success
    const loginPayload = JSON.stringify({
      email: __ENV.ADMIN_EMAIL || 'admin@example.com',
      password: __ENV.ADMIN_PASSWORD || 'admin123!',
    });

    const r2 = http.post(`${BASE_URL}/api/v1/auth/login`, loginPayload, {
      headers: { ...headers, 'X-Device-ID': `k6-device-${vu}` },
    });

    const success = r2.status === 200;
    loginRate.add(success);
    requestDuration.add(r2.timings.duration);
    totalRequests.add(1);

    check(r2, { 'login succeeded': success });

    if (success) {
      authTokens['admin'] = {
        access: r2.json('access_token'),
        refresh: r2.json('refresh_token'),
      };
    }
  }

  // Phase 3: Authenticated requests (40% of iterations)
  const token = authTokens['admin']?.access;
  if (token && (iter % 5 === 3 || iter % 5 === 4)) {
    const authHeaders = { ...headers, Authorization: `Bearer ${token}` };

    // Hit various endpoints
    const endpoints = [
      { url: `${BASE_URL}/api/v1/admin/users?limit=10`, method: 'GET' },
      { url: `${BASE_URL}/api/v1/admin/roles?limit=10`, method: 'GET' },
      { url: `${BASE_URL}/api/v1/admin/permissions?limit=10`, method: 'GET' },
      { url: `${BASE_URL}/health`, method: 'GET' },
      { url: `${BASE_URL}/api/v1/incidents/?limit=10`, method: 'GET' },
    ];

    const ep = endpoints[Math.floor(Math.random() * endpoints.length)];
    const r3 = http.get(ep.url, { headers: authHeaders });

    requestDuration.add(r3.timings.duration);
    totalRequests.add(1);

    check(r3, {
      [`${ep.url} status 2xx`]: (r) => r.status >= 200 && r.status < 300,
    });
  }

  sleep(Math.random() * 0.5 + 0.1); // 100ms-600ms think time
}
