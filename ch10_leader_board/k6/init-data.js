import http from 'k6/http';
import { sleep } from 'k6';

/**
 * 資料初始化腳本 - 50K 用戶規模
 *
 * 目的：增加額外的測試資料到資料庫已有的 50,000 筆基礎資料上
 *
 * 注意：PostgreSQL 已經預填了 50,000 個 player_1 到 player_50000
 * 這個腳本會新增 player_50001 到 player_100000
 */
export const options = {
  scenarios: {
    data_initialization: {
      executor: 'shared-iterations',
      vus: 50,           // 50 個並發虛擬用戶
      iterations: 50000, // 總共 50,000 次迭代（每次創建一個用戶）
      maxDuration: '30m', // 最多 30 分鐘
    },
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

function generateMatchId(userId, matchNum) {
  return `${userId}_init_match_${matchNum}_${Date.now()}`;
}

export default function () {
  // 從 50001 開始編號，避免與預填充資料衝突
  const userIndex = 50001 + __ITER;
  const userId = `player_${userIndex}`;

  // 每個用戶隨機贏得 1-1000 場比賽，分數分布更真實
  const wins = Math.floor(Math.random() * 1000) + 1;

  // 批次更新分數（不是一場一場更新，而是直接更新總分）
  // 這樣可以加快初始化速度
  for (let i = 0; i < wins; i++) {
    const payload = JSON.stringify({
      user_id: userId,
      points: 1,
      match_id: generateMatchId(userId, i),
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
      },
    };

    const res = http.post(`${BASE_URL}/v1/scores`, payload, params);

    // 每 10 場比賽才 sleep 一次，加快速度
    if (i % 10 === 0) {
      sleep(0.01);
    }
  }

  // 顯示進度
  if (__ITER % 1000 === 0) {
    console.log(`Progress: ${__ITER}/50000 users created`);
  }
}

export function handleSummary(data) {
  console.log('\n========================================');
  console.log('Data Initialization Complete!');
  console.log('========================================');
  console.log(`Total requests: ${data.metrics.http_reqs?.values?.count || 0}`);
  console.log(`Failed requests: ${data.metrics.http_req_failed?.values?.count || 0}`);
  console.log(`Average duration: ${data.metrics.http_req_duration?.values?.avg?.toFixed(2) || 0}ms`);
  console.log('========================================\n');

  return {
    'stdout': '',
  };
}
