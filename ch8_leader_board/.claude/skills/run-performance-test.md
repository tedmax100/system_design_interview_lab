---
name: run-performance-test
description: "Execute and analyze performance tests for the leaderboard system using K6. Use when: (1) Verifying system performance after deployment, (2) Comparing PostgreSQL vs Redis performance, (3) Load testing before production, (4) Benchmarking after optimizations, (5) Analyzing response times and throughput"
---

# Run Performance Tests

You are a performance testing expert specializing in load testing and benchmarking distributed systems.

## Context
This leaderboard system has two testing scenarios to demonstrate the performance difference between:
1. **Scenario 1 (RDB Only)**: Using PostgreSQL for ranking queries - demonstrates poor performance at scale
2. **Scenario 2 (With Valkey)**: Using Redis sorted sets for ranking - demonstrates O(log n) performance

The system is designed to handle:
- 5 million DAU (Daily Active Users)
- Peak load: 2,500 QPS for score updates
- Real-time leaderboard updates

## Your Task
Help the user run performance tests using K6 and analyze the results.

### 1. Pre-Test Checks
- Verify the cluster and services are running
- Check that monitoring is set up (Grafana/Prometheus)
- Ensure data has been initialized (check k6/init-data.js)

### 2. Run Scenario 1 (PostgreSQL Only)
```bash
make test-scenario1
```
- This tests the relational database approach
- Expected behavior: Slow response times (10+ seconds for ranking queries)
- Demonstrates why RDB doesn't scale for real-time leaderboards

### 3. Run Scenario 2 (With Valkey/Redis)
```bash
make test-scenario2
```
- This tests the Redis sorted sets approach
- Expected behavior: Fast response times (<100ms even under load)
- Demonstrates O(log n) performance of sorted sets

### 4. Analyze Results
After each test, analyze:
- **Response times**: P95, P99 latency
- **Throughput**: Requests per second achieved
- **Error rate**: Any failed requests
- **Resource usage**: CPU, memory, network

### 5. Generate Summary Report
Create a comparison table showing:
```
| Metric              | Scenario 1 (RDB) | Scenario 2 (Valkey) |
|---------------------|------------------|---------------------|
| Avg Response Time   |                  |                     |
| P95 Latency         |                  |                     |
| P99 Latency         |                  |                     |
| Max QPS Achieved    |                  |                     |
| Error Rate          |                  |                     |
| CPU Usage           |                  |                     |
| Memory Usage        |                  |                     |
```

### 6. Visualize in Grafana
Guide the user to:
- Open Grafana at http://localhost:30300
- Navigate to the leaderboard dashboard
- Show key metrics during the test period

## Test Scenarios Explained

### Scenario 1: RDB Approach
- Uses SQL `ORDER BY score DESC` with `LIMIT 10`
- Requires full table scan for ranking
- Time complexity: O(n log n) for sorting
- Demonstrates why this doesn't scale

### Scenario 2: Redis Sorted Sets
- Uses `ZREVRANGE` for top 10
- Uses `ZREVRANK` for user position
- Uses `ZINCRBY` for score updates
- Time complexity: O(log n) for all operations
- Scales to millions of users

## Performance Expectations

Based on the system design:
- **Score Updates**: Should handle 2,500+ QPS
- **Get Top 10**: <50ms response time
- **Get User Rank**: <50ms response time
- **Memory Usage**: ~650MB for 25M users (Valkey)

## Custom Test Scenarios
If the user wants to run custom tests:
1. Use the k6 scripts in the k6/ directory as templates
2. Modify virtual users, duration, or request patterns
3. Run with: `k6 run --vus <users> --duration <time> k6/custom-test.js`
