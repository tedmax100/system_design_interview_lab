import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { v7 as uuidv7 } from 'k6/x/uuid';

// Custom metrics
const transferSuccess = new Counter('transfer_success');
const transferFailed = new Counter('transfer_failed');
const transferRate = new Rate('transfer_success_rate');
const transferDuration = new Trend('transfer_duration');

// Test configuration
export const options = {
    scenarios: {
        load_test: {
            executor: 'constant-vus',
            vus: 100,           // 100 virtual users
            duration: '1m',     // Run for 1 minute
        },
    },
    thresholds: {
        http_req_duration: ['p(95)<500'],  // 95% of requests should be under 500ms
        transfer_success_rate: ['rate>0.95'], // 95% success rate
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Test accounts (must be initialized before running)
const accounts = ['alice', 'bob', 'charlie'];

// Helper to get random account
function getRandomAccount() {
    return accounts[Math.floor(Math.random() * accounts.length)];
}

// Helper to get two different accounts
function getTwoAccounts() {
    const from = getRandomAccount();
    let to = getRandomAccount();
    while (to === from) {
        to = getRandomAccount();
    }
    return { from, to };
}

// Setup function - initialize test accounts
export function setup() {
    console.log('Setting up test accounts...');

    accounts.forEach(account => {
        const res = http.post(`${BASE_URL}/v1/wallet/init`, JSON.stringify({
            account: account,
            balance: 1000000, // 10000.00 in cents
        }), {
            headers: { 'Content-Type': 'application/json' },
        });

        check(res, {
            'account initialized': (r) => r.status === 200,
        });
    });

    // Get initial total balance
    const balancesRes = http.get(`${BASE_URL}/v1/wallet/balances`);
    const balances = JSON.parse(balancesRes.body);
    console.log(`Initial total balance: ${balances.total_balance}`);

    return { initialTotal: balances.total_balance };
}

// Main test function
export default function() {
    const { from, to } = getTwoAccounts();
    const amount = Math.floor(Math.random() * 100) + 1; // 1-100 cents
    const transactionId = uuidv7();

    const payload = JSON.stringify({
        transaction_id: transactionId,
        from_account: from,
        to_account: to,
        amount: amount,
    });

    const params = {
        headers: { 'Content-Type': 'application/json' },
        tags: { name: 'transfer' },
    };

    const startTime = Date.now();
    const res = http.post(`${BASE_URL}/v1/wallet/transfer`, payload, params);
    const duration = Date.now() - startTime;

    transferDuration.add(duration);

    const success = check(res, {
        'status is 200': (r) => r.status === 200,
        'response has success': (r) => {
            try {
                const body = JSON.parse(r.body);
                return body.success !== undefined;
            } catch {
                return false;
            }
        },
    });

    if (success && res.status === 200) {
        const body = JSON.parse(res.body);
        if (body.success) {
            transferSuccess.add(1);
            transferRate.add(1);
        } else {
            // Business failure (e.g., insufficient funds) - still counts as processed
            transferFailed.add(1);
            transferRate.add(1);
        }
    } else {
        transferFailed.add(1);
        transferRate.add(0);
    }

    sleep(0.01); // Small delay between requests
}

// Teardown function - verify total balance unchanged
export function teardown(data) {
    console.log('Verifying final state...');

    const balancesRes = http.get(`${BASE_URL}/v1/wallet/balances`);
    const balances = JSON.parse(balancesRes.body);

    console.log(`Final total balance: ${balances.total_balance}`);
    console.log(`Initial total balance: ${data.initialTotal}`);

    if (balances.total_balance === data.initialTotal) {
        console.log('✓ Total balance unchanged - data integrity verified!');
    } else {
        console.log('✗ ERROR: Total balance changed! Data integrity violation!');
    }
}
