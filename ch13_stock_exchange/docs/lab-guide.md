# Lab Guide: Stock Exchange System

## Prerequisites

- Go 1.25+
- Docker & Docker Compose
- k6 (for load testing): `brew install k6` or [k6.io/docs/get-started](https://k6.io/docs/get-started/installation/)
- curl and jq (for manual API testing)

## Step 1: Build and Run

### Option A: Docker Compose (recommended)

```bash
make docker-up
```

This starts:
- **exchange-service** on `localhost:8080` (API) and `localhost:9090` (metrics)
- **Prometheus** on `localhost:9091`
- **Grafana** on `localhost:3000` (admin/admin)

### Option B: Run locally

```bash
make deps
make run
```

## Step 2: Initialize Wallets

```bash
make init-wallets
```

Or manually:
```bash
curl -X POST http://localhost:8080/v1/wallet/init \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user1", "cash_balance": 10000000, "holdings": {"AAPL": 5000}}'

curl -X POST http://localhost:8080/v1/wallet/init \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user2", "cash_balance": 10000000, "holdings": {"AAPL": 5000}}'
```

## Step 3: Place Orders

### Place a sell order
```bash
curl -X POST http://localhost:8080/v1/order \
  -H "Content-Type: application/json" \
  -d '{"symbol": "AAPL", "side": "sell", "price": 10010, "quantity": 1000, "user_id": "user1"}'
```

### Check the order book
```bash
curl http://localhost:8080/v1/marketdata/orderBook/L2?symbol=AAPL&depth=5 | jq .
```

You should see the sell order at price 10010 with quantity 1000.

### Place a matching buy order
```bash
curl -X POST http://localhost:8080/v1/order \
  -H "Content-Type: application/json" \
  -d '{"symbol": "AAPL", "side": "buy", "price": 10010, "quantity": 200, "user_id": "user2"}'
```

## Step 4: Verify Matching

### Check executions
```bash
curl http://localhost:8080/v1/execution?symbol=AAPL | jq .
```

You should see an execution for 200 shares at price 10010.

### Check order book (should show 800 remaining)
```bash
curl http://localhost:8080/v1/marketdata/orderBook/L2?symbol=AAPL&depth=5 | jq .
```

### Check candlestick data
```bash
curl http://localhost:8080/v1/marketdata/candles?symbol=AAPL&count=10 | jq .
```

## Step 5: Cancel an Order

```bash
# Use the order_id from step 3's sell order response
curl -X DELETE http://localhost:8080/v1/order/<order-id> | jq .
```

Verify the book is now empty:
```bash
curl http://localhost:8080/v1/marketdata/orderBook/L2?symbol=AAPL&depth=5 | jq .
```

## Step 6: Check Wallets

```bash
curl http://localhost:8080/v1/wallet/balances | jq .
```

After the 200-share trade at $100.10:
- **user1** (seller): cash increased by $20,020, AAPL shares decreased by 200
- **user2** (buyer): cash decreased by $20,020, AAPL shares increased by 200

## Step 7: Run Automated Tests

### Unit tests
```bash
make test
```

### Smoke test (full simulation scenario)
```bash
make smoke-test
```

### Load test (100 VUs, 1 minute)
```bash
make k6-test
```

## Step 8: Explore Grafana

1. Open http://localhost:3000 (admin/admin)
2. Go to Explore → select Prometheus datasource
3. Try these queries:
   - `rate(exchange_orders_total[1m])` — Order rate
   - `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[1m]))` — p95 latency
   - `exchange_orderbook_depth` — Order book depth
   - `exchange_sequencer_inbound_seq` — Sequence progression

## Cleanup

```bash
make docker-down      # Stop services
make docker-clean     # Remove containers and volumes
```
