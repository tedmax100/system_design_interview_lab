#!/bin/bash

# Simple API test script

BASE_URL=${1:-http://localhost:8080}

echo "Testing Leaderboard API at $BASE_URL"
echo "======================================="

# Test 1: Health check
echo -e "\n1. Health Check"
curl -s "$BASE_URL/health"
echo ""

# Test 2: Update score for a user
echo -e "\n2. Update Score (POST /v1/scores)"
curl -s -X POST "$BASE_URL/v1/scores" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user_1",
    "points": 1,
    "match_id": "test_match_1"
  }' | jq .
echo ""

# Test 3: Update score for another user
echo -e "\n3. Update Score for another user"
curl -s -X POST "$BASE_URL/v1/scores" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user_2",
    "points": 1,
    "match_id": "test_match_2"
  }' | jq .
echo ""

# Test 4: Get top 10 leaderboard
echo -e "\n4. Get Top 10 Leaderboard (GET /v1/scores)"
curl -s "$BASE_URL/v1/scores" | jq .
echo ""

# Test 5: Get specific user rank
echo -e "\n5. Get User Rank (GET /v1/scores/{user_id})"
curl -s "$BASE_URL/v1/scores/test_user_1" | jq .
echo ""

# Test 6: Get metrics
echo -e "\n6. Prometheus Metrics (GET /metrics)"
curl -s "$BASE_URL/metrics" | grep "^http_request" | head -5
echo ""

echo "======================================="
echo "API tests completed!"
echo "======================================="
