import http from 'k6/http';
import { sleep } from 'k6';

// This script initializes test data in the leaderboard
export const options = {
  vus: 10,
  iterations: 1000, // Create 1000 users with scores
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

function generateMatchId() {
  return `init_match_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}

export default function () {
  const userId = `player_${__VU}_${__ITER}`;

  // Each user wins multiple games to get realistic score distribution
  const wins = Math.floor(Math.random() * 100) + 1; // 1-100 wins

  for (let i = 0; i < wins; i++) {
    const payload = JSON.stringify({
      user_id: userId,
      points: 1,
      match_id: generateMatchId(),
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
      },
    };

    http.post(`${BASE_URL}/v1/scores`, payload, params);
    sleep(0.01); // Small delay to avoid overwhelming the database
  }
}

console.log('Data initialization complete!');
