import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = (__ENV.BASE_URL || 'http://localhost:8080').replace(/\/+$/, '');
const VUS = Number(__ENV.VUS || 50);
const DURATION = __ENV.DURATION || '1m';

export const options = {
  tags: { replicas: __ENV.REPLICAS || '1' },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  scenarios: {
    leaderboard_read: {
      executor: 'constant-vus',
      vus: VUS,
      duration: DURATION,
    },
  },
};

export default function () {
  const response = http.get(`${BASE_URL}/api/leaderboard/top?limit=10`, {
    tags: { name: 'leaderboard_top' },
  });

  check(response, {
    'ranking responds HTTP 200': (result) => result.status === 200,
  });
}
