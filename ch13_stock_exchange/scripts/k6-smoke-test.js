import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate==1.0'],
  },
};

export function setup() {
  // Initialize wallets for testing
  const users = [
    { user_id: 'seller1', cash_balance: 10000000, holdings: { AAPL: 5000 } },
    { user_id: 'buyer1', cash_balance: 10000000, holdings: {} },
  ];

  for (const user of users) {
    const res = http.post(`${BASE_URL}/v1/wallet/init`, JSON.stringify(user), {
      headers: { 'Content-Type': 'application/json' },
    });
    check(res, { [`init wallet ${user.user_id}`]: (r) => r.status === 200 });
  }

  sleep(0.5);
}

export default function () {
  // Step 1: Place sell order (AAPL, $100.10, qty 1000)
  console.log('Step 1: Place sell order');
  const sellRes = http.post(
    `${BASE_URL}/v1/order`,
    JSON.stringify({
      symbol: 'AAPL',
      side: 'sell',
      price: 10010, // $100.10 in cents
      quantity: 1000,
      user_id: 'seller1',
    }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  check(sellRes, {
    'sell order created': (r) => r.status === 201,
    'sell order has ID': (r) => JSON.parse(r.body).order_id !== undefined,
  });

  const sellOrder = JSON.parse(sellRes.body);
  console.log(`  Sell order ID: ${sellOrder.order_id}`);

  sleep(0.2);

  // Step 2: Verify order book shows the sell order
  console.log('Step 2: Verify order book');
  const bookRes1 = http.get(`${BASE_URL}/v1/marketdata/orderBook/L2?symbol=AAPL&depth=5`);
  check(bookRes1, {
    'order book OK': (r) => r.status === 200,
    'sell order in book': (r) => {
      const book = JSON.parse(r.body);
      return book.asks.length > 0 && book.asks[0].price === 10010 && book.asks[0].quantity === 1000;
    },
  });

  const book1 = JSON.parse(bookRes1.body);
  console.log(`  Asks: ${JSON.stringify(book1.asks)}`);

  // Step 3: Place buy order (AAPL, $100.10, qty 200) → should match
  console.log('Step 3: Place buy order (should match)');
  const buyRes = http.post(
    `${BASE_URL}/v1/order`,
    JSON.stringify({
      symbol: 'AAPL',
      side: 'buy',
      price: 10010,
      quantity: 200,
      user_id: 'buyer1',
    }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  check(buyRes, {
    'buy order created': (r) => r.status === 201,
  });

  const buyOrder = JSON.parse(buyRes.body);
  console.log(`  Buy order ID: ${buyOrder.order_id}`);

  sleep(0.5); // Wait for matching engine to process

  // Step 4: Verify execution created
  console.log('Step 4: Verify execution');
  const execRes = http.get(`${BASE_URL}/v1/execution?symbol=AAPL`);
  check(execRes, {
    'executions OK': (r) => r.status === 200,
    'has executions': (r) => JSON.parse(r.body).length > 0,
    'execution quantity correct': (r) => {
      const execs = JSON.parse(r.body);
      return execs.some((e) => e.quantity === 200 && e.price === 10010);
    },
  });

  const execs = JSON.parse(execRes.body);
  console.log(`  Executions: ${execs.length}`);

  // Step 4b: Verify order book updated (remaining 800)
  console.log('Step 4b: Verify order book updated');
  const bookRes2 = http.get(`${BASE_URL}/v1/marketdata/orderBook/L2?symbol=AAPL&depth=5`);
  check(bookRes2, {
    'book updated OK': (r) => r.status === 200,
    'remaining qty 800': (r) => {
      const book = JSON.parse(r.body);
      return book.asks.length > 0 && book.asks[0].quantity === 800;
    },
  });

  const book2 = JSON.parse(bookRes2.body);
  console.log(`  Asks after match: ${JSON.stringify(book2.asks)}`);

  // Step 5: Verify candlestick data
  console.log('Step 5: Verify candlestick data');
  const candleRes = http.get(`${BASE_URL}/v1/marketdata/candles?symbol=AAPL&count=10`);
  check(candleRes, {
    'candles OK': (r) => r.status === 200,
    'has candle data': (r) => JSON.parse(r.body).length > 0,
    'candle price correct': (r) => {
      const candles = JSON.parse(r.body);
      return candles.some((c) => c.open === 10010);
    },
  });

  const candles = JSON.parse(candleRes.body);
  console.log(`  Candles: ${JSON.stringify(candles)}`);

  // Step 6: Cancel remaining sell order
  console.log('Step 6: Cancel remaining sell order');
  const cancelRes = http.del(`${BASE_URL}/v1/order/${sellOrder.order_id}`);
  check(cancelRes, {
    'cancel OK': (r) => r.status === 200,
    'order canceled': (r) => JSON.parse(r.body).status === 'canceled',
  });

  sleep(0.2);

  // Verify order book is empty
  console.log('Step 6b: Verify order book empty');
  const bookRes3 = http.get(`${BASE_URL}/v1/marketdata/orderBook/L2?symbol=AAPL&depth=5`);
  check(bookRes3, {
    'final book OK': (r) => r.status === 200,
    'no asks remaining': (r) => {
      const book = JSON.parse(r.body);
      return book.asks.length === 0;
    },
  });

  // Verify wallet balances
  console.log('Step 7: Verify wallet balances');
  const walletRes = http.get(`${BASE_URL}/v1/wallet/balances`);
  check(walletRes, {
    'wallets OK': (r) => r.status === 200,
  });

  const wallets = JSON.parse(walletRes.body);
  console.log(`  Wallets: ${JSON.stringify(wallets)}`);

  console.log('Smoke test completed!');
}
