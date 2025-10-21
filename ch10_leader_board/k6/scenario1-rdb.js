import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const scoreUpdateDuration = new Trend('score_update_duration');
const getLeaderboardDuration = new Trend('get_leaderboard_duration');
const getUserRankDuration = new Trend('get_user_rank_duration');

// Test configuration
export const options = {
  scenarios: {
    // Scenario 1: Ramp up to peak load
    ramp_up: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 10 },  // Warm up
        { duration: '1m', target: 50 },   // Ramp to moderate load
        { duration: '2m', target: 100 },  // Ramp to high load
        { duration: '2m', target: 150 },  // Peak load (simulating 2500 QPS with mixed operations)
        { duration: '1m', target: 50 },   // Ramp down
        { duration: '30s', target: 0 },   // Cool down
      ],
      gracefulRampDown: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<50'], // 95% of requests should be below 50ms (SLO target)
    http_req_failed: ['rate<0.05'],   // Error rate should be less than 5%
    score_update_duration: ['p(95)<100'],
    get_leaderboard_duration: ['p(95)<50'],
    get_user_rank_duration: ['p(95)<100'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Generate random user IDs
function randomUser() {
  const userId = `user_${Math.floor(Math.random() * 10000)}`;
  return userId;
}

// Generate match ID for idempotency
function generateMatchId() {
  return `match_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}

export default function () {
  // Simulate different operations with realistic distribution
  const operation = Math.random();

  if (operation < 0.7) {
    // 70% - Score updates (primary write operation)
    const payload = JSON.stringify({
      user_id: randomUser(),
      points: 1,
      match_id: generateMatchId(),
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
      },
      tags: { name: 'UpdateScore' },
    };

    const startTime = new Date();
    const res = http.post(`${BASE_URL}/v1/scores`, payload, params);
    scoreUpdateDuration.add(new Date() - startTime);

    check(res, {
      'score update status is 200': (r) => r.status === 200,
      'score update has success': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.success === true;
        } catch (e) {
          return false;
        }
      },
    }) || errorRate.add(1);

  } else if (operation < 0.85) {
    // 15% - Get top 10 leaderboard (frequent read)
    const params = {
      tags: { name: 'GetLeaderboard' },
    };

    const startTime = new Date();
    const res = http.get(`${BASE_URL}/v1/scores`, params);
    getLeaderboardDuration.add(new Date() - startTime);

    check(res, {
      'get leaderboard status is 200': (r) => r.status === 200,
      'leaderboard has data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.status === 'success' && Array.isArray(body.data.leaderboard);
        } catch (e) {
          return false;
        }
      },
    }) || errorRate.add(1);

  } else {
    // 15% - Get user rank (heavy operation in RDB)
    const userId = randomUser();
    const params = {
      tags: { name: 'GetUserRank' },
    };

    const startTime = new Date();
    const res = http.get(`${BASE_URL}/v1/scores/${userId}`, params);
    getUserRankDuration.add(new Date() - startTime);

    // User might not exist, so 404 is acceptable
    check(res, {
      'get user rank status is 200 or 404': (r) => r.status === 200 || r.status === 404,
    }) || errorRate.add(1);
  }

  // Random sleep to simulate realistic user behavior
  sleep(Math.random() * 0.5 + 0.1); // 0.1-0.6 seconds
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'summary-scenario1.json': JSON.stringify(data, null, 2),
  };
}

// Helper function for text summary
function textSummary(data, options) {
  let output = '\n';
  output += '======== Scenario 1: PostgreSQL Only - Load Test Results ========\n\n';

  output += 'Test Summary:\n';
  output += `  Total Requests: ${data.metrics.http_reqs?.values?.count || 0}\n`;
  output += `  Failed Requests: ${data.metrics.http_req_failed?.values?.rate ?
    (data.metrics.http_req_failed.values.rate * 100).toFixed(2) : 0}%\n`;
  output += `  Avg Request Duration: ${data.metrics.http_req_duration?.values?.avg?.toFixed(2) || 0}ms\n`;
  output += `  P95 Request Duration: ${data.metrics.http_req_duration?.values?.['p(95)']?.toFixed(2) || 0}ms\n`;
  output += `  P99 Request Duration: ${data.metrics.http_req_duration?.values?.['p(99)']?.toFixed(2) || 0}ms\n\n`;

  output += 'Operation-specific metrics:\n';
  output += `  Score Update P95: ${data.metrics.score_update_duration?.values?.['p(95)']?.toFixed(2) || 0}ms\n`;
  output += `  Get Leaderboard P95: ${data.metrics.get_leaderboard_duration?.values?.['p(95)']?.toFixed(2) || 0}ms\n`;
  output += `  Get User Rank P95: ${data.metrics.get_user_rank_duration?.values?.['p(95)']?.toFixed(2) || 0}ms\n\n`;

  output += '==================================================================\n';

  return output;
}
