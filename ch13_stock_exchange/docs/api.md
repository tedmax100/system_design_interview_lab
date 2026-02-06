# API Reference

Base URL: `http://localhost:8080`

## Health Check

```
GET /health
```

Response:
```json
{ "status": "ok", "service": "stock-exchange" }
```

---

## Place Order

```
POST /v1/order
```

Request body:
```json
{
  "symbol": "AAPL",
  "side": "buy",
  "price": 10010,
  "quantity": 200,
  "user_id": "user1"
}
```

- `price` is in cents (10010 = $100.10)
- `side` must be `"buy"` or `"sell"`

Response (201 Created):
```json
{
  "order_id": "550e8400-e29b-41d4-a716-446655440000",
  "symbol": "AAPL",
  "side": "buy",
  "price": 10010,
  "quantity": 200,
  "filled_quantity": 200,
  "remaining_quantity": 0,
  "status": "filled",
  "user_id": "user1",
  "created_at": "2025-01-15T10:30:00Z",
  "sequence_id": 0
}
```

Note: `status` at response time reflects the order's state *before* the sequencer processes it. Use `/v1/execution` to confirm matches.

---

## Cancel Order

```
DELETE /v1/order/:id
```

Response (200 OK):
```json
{
  "order_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "canceled",
  ...
}
```

---

## Get Executions

```
GET /v1/execution?symbol=AAPL&order_id=xxx&since=2025-01-15T00:00:00Z
```

All query parameters are optional. `since` uses RFC3339 format.

Response:
```json
[
  {
    "exec_id": "order123-exec-1",
    "order_id": "order123",
    "symbol": "AAPL",
    "side": "buy",
    "price": 10010,
    "quantity": 200,
    "maker_order_id": "sell456",
    "taker_order_id": "order123",
    "timestamp": "2025-01-15T10:30:00Z",
    "sequence_id": 1
  }
]
```

---

## L2 Order Book

```
GET /v1/marketdata/orderBook/L2?symbol=AAPL&depth=5
```

- `symbol` (required)
- `depth` (optional, default 10)

Response:
```json
{
  "symbol": "AAPL",
  "bids": [
    { "price": 10000, "quantity": 500 },
    { "price": 9990, "quantity": 300 }
  ],
  "asks": [
    { "price": 10010, "quantity": 800 },
    { "price": 10020, "quantity": 200 }
  ]
}
```

---

## Candlestick Data

```
GET /v1/marketdata/candles?symbol=AAPL&count=10
```

- `symbol` (required)
- `count` (optional, default 100)

Response:
```json
[
  {
    "symbol": "AAPL",
    "open": 10010,
    "high": 10020,
    "low": 10005,
    "close": 10015,
    "volume": 1500,
    "timestamp": "2025-01-15T10:30:00Z",
    "interval": "1m"
  }
]
```

---

## Initialize Wallet (Lab Helper)

```
POST /v1/wallet/init
```

Request body:
```json
{
  "user_id": "user1",
  "cash_balance": 10000000,
  "holdings": {
    "AAPL": 5000,
    "GOOG": 3000
  }
}
```

- `cash_balance` is in cents (10000000 = $100,000.00)

---

## Get Wallet Balances (Lab Helper)

```
GET /v1/wallet/balances
GET /v1/wallet/balances?user_id=user1
```

Response (all users):
```json
[
  {
    "user_id": "user1",
    "cash_balance": 10000000,
    "holdings": { "AAPL": 5000 }
  }
]
```
