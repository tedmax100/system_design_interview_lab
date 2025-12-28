import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { v7 as uuidv7 } from 'k6/x/uuid';

// Custom metrics
const transferSuccess = new Counter('transfer_success');
const transferFailed = new Counter('transfer_failed');
const transferRate = new Rate('transfer_success_rate');
const transferDuration = new Trend('transfer_duration');

// Stress test configuration - ramp up load progressively
export const options = {
    scenarios: {
        stress_test: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '30s', target: 50 },   // Ramp up to 50 users
                { duration: '1m', target: 50 },    // Hold at 50
                { duration: '30s', target: 100 },  // Ramp up to 100 users
                { duration: '1m', target: 100 },   // Hold at 100
                { duration: '30s', target: 200 },  // Ramp up to 200 users
                { duration: '1m', target: 200 },   // Hold at 200
                { duration: '30s', target: 0 },    // Ramp down
            ],
        },
    },
    thresholds: {
        http_req_duration: ['p(99)<1000'], // 99% under 1s
        transfer_success_rate: ['rate>0.9'], // 90% success rate under stress
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// More accounts for stress testing
const accounts = [];
for (let i = 0; i < 20; i++) {
    accounts.push(`user${i}`);
}

function getRandomAccount() {
    return accounts[Math.floor(Math.random() * accounts.length)];
}

function getTwoAccounts() {
    const from = getRandomAccount();
    let to = getRandomAccount();
    while (to === from) {
        to = getRandomAccount();
    }
    return { from, to };
}

export function setup() {
    console.log('Setting up stress test accounts...');

    accounts.forEach(account => {
        const res = http.post(`${BASE_URL}/v1/wallet/init`, JSON.stringify({
            account: account,
            balance: 10000000, // 100000.00 in cents - higher balance for stress test
        }), {
            headers: { 'Content-Type': 'application/json' },
        });

        check(res, {
            'account initialized': (r) => r.status === 200,
        });
    });

    const balancesRes = http.get(`${BASE_URL}/v1/wallet/balances`);
    const balances = JSON.parse(balancesRes.body);
    console.log(`Initial total balance: ${balances.total_balance}`);
    console.log(`Number of accounts: ${balances.account_count}`);

    return { initialTotal: balances.total_balance };
}

export default function() {
    const { from, to } = getTwoAccounts();
    const amount = Math.floor(Math.random() * 1000) + 1; // 1-1000 cents
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
        'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
    });

    if (success) {
        try {
            const body = JSON.parse(res.body);
            if (res.status === 200 && body.success) {
                transferSuccess.add(1);
                transferRate.add(1);
            } else {
                transferFailed.add(1);
                transferRate.add(1); // Still processed successfully
            }
        } catch {
            transferFailed.add(1);
            transferRate.add(0);
        }
    } else {
        transferFailed.add(1);
        transferRate.add(0);
    }
}

export function teardown(data) {
    console.log('Verifying final state after stress test...');

    // Wait a moment for any pending operations
    sleep(2);

    const balancesRes = http.get(`${BASE_URL}/v1/wallet/balances`);
    const balances = JSON.parse(balancesRes.body);

    console.log(`Final total balance: ${balances.total_balance}`);
    console.log(`Initial total balance: ${data.initialTotal}`);

    if (balances.total_balance === data.initialTotal) {
        console.log('✓ PASS: Total balance unchanged - data integrity verified!');
    } else {
        console.log('✗ FAIL: Total balance changed! Data integrity violation!');
        console.log(`Difference: ${balances.total_balance - data.initialTotal}`);
    }
}
