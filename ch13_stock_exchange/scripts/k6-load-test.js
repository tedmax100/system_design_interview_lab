import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

const orderLatency = new Trend('order_latency_ms');
const matchRate = new Rate('match_rate');

export const options = {
  stages: [
    { duration: '10s', target: 20 },  // Ramp up
    { duration: '40s', target: 100 }, // Sustained load
    { duration: '10s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<50', 'p(99)<100'],
    checks: ['rate>0.95'],
    order_latency_ms: ['p(95)<50'],
  },
};

const SYMBOLS = ['AAPL', 'GOOG', 'MSFT', 'AMZN', 'META'];
const BASE_PRICES = {
  AAPL: 15000,  // $150.00
  GOOG: 14000,  // $140.00
  MSFT: 38000,  // $380.00
  AMZN: 17500,  // $175.00
  META: 50000,  // $500.00
};

export function setup() {
  // Initialize wallets for load test users
  for (let i = 0; i < 200; i++) {
    const holdings = {};
    for (const sym of SYMBOLS) {
      holdings[sym] = 100000;
    }
    http.post(
      `${BASE_URL}/v1/wallet/init`,
      JSON.stringify({
        user_id: `loaduser${i}`,
        cash_balance: 100000000000, // $1B in cents
        holdings: holdings,
      }),
      { headers: { 'Content-Type': 'application/json' } }
    );
  }
  sleep(1);
}

export default function () {
  const vuId = __VU % 200;
  const userId = `loaduser${vuId}`;
  const symbol = SYMBOLS[Math.floor(Math.random() * SYMBOLS.length)];
  const basePrice = BASE_PRICES[symbol];
  const side = Math.random() > 0.5 ? 'buy' : 'sell';

  // Random price within ±1% of base price
  const priceOffset = Math.floor((Math.random() - 0.5) * basePrice * 0.02);
  const price = basePrice + priceOffset;
  const quantity = Math.floor(Math.random() * 100) + 1;

  // Place order
  const start = Date.now();
  const res = http.post(
    `${BASE_URL}/v1/order`,
    JSON.stringify({
      symbol: symbol,
      side: side,
      price: price,
      quantity: quantity,
      user_id: userId,
    }),
    { headers: { 'Content-Type': 'application/json' } }
  );
  const latency = Date.now() - start;

  check(res, {
    'order accepted': (r) => r.status === 201 || r.status === 400,
  });

  if (res.status === 201) {
    orderLatency.add(latency);

    const order = JSON.parse(res.body);
    // Check if the order was matched
    matchRate.add(order.status === 'filled' || order.status === 'partially_filled');
  }

  // Occasionally check the order book
  if (Math.random() < 0.1) {
    const bookRes = http.get(`${BASE_URL}/v1/marketdata/orderBook/L2?symbol=${symbol}&depth=5`);
    check(bookRes, { 'book OK': (r) => r.status === 200 });
  }

  // Occasionally check candles
  if (Math.random() < 0.05) {
    const candleRes = http.get(`${BASE_URL}/v1/marketdata/candles?symbol=${symbol}&count=10`);
    check(candleRes, { 'candles OK': (r) => r.status === 200 });
  }

  sleep(0.05 + Math.random() * 0.1);
}
