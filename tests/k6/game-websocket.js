import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import ws from 'k6/ws';

const BASE_URL = (__ENV.BASE_URL || 'http://localhost').replace(/\/+$/, '');
const VUS = Number(__ENV.VUS || 10);
const MAX_DURATION = __ENV.MAX_DURATION || '2m';
const WS_TIMEOUT_MS = Number(__ENV.WS_TIMEOUT_MS || 30000);

const playToScored = new Trend('ws_play_to_hand_scored_ms', true);
const sessionsConnected = new Counter('ws_sessions_connected');
const actionsSent = new Counter('ws_play_actions_sent');
const actionsScored = new Counter('ws_play_actions_scored');
const sessionErrors = new Counter('ws_session_errors');
const sessionTimeouts = new Counter('ws_session_timeouts');

export const options = {
  setupTimeout: '2m',
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  scenarios: {
    game_websocket: {
      executor: 'per-vu-iterations',
      vus: VUS,
      iterations: 1,
      maxDuration: MAX_DURATION,
    },
  },
};

function jsonOptions(token, name) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers.Authorization = `Bearer ${token}`;
  return { headers, tags: { name } };
}

export function setup() {
  const fixtures = [];
  const runId = Date.now().toString(36);

  for (let i = 1; i <= VUS; i++) {
    const alias = `k6_${runId}_${i}`;
    const registration = http.post(
      `${BASE_URL}/api/auth/register`,
      JSON.stringify({ alias }),
      jsonOptions(null, 'k6_fixture_register'),
    );

    if (registration.status !== 201) {
      const reason = registration.error || registration.error_code;
      throw new Error(
        `No se pudo registrar ${alias}: HTTP ${registration.status}${reason ? ` (${reason})` : ''}`,
      );
    }

    const identity = registration.json();
    const roomResponse = http.post(
      `${BASE_URL}/api/rooms`,
      JSON.stringify({ mode: 'bot', difficulty: 'easy', target_score: 100000 }),
      jsonOptions(identity.token, 'k6_fixture_create_room'),
    );

    if (roomResponse.status !== 201) {
      const reason = roomResponse.error || roomResponse.error_code;
      throw new Error(
        `No se pudo crear sala para ${alias}: HTTP ${roomResponse.status}${reason ? ` (${reason})` : ''}`,
      );
    }

    const room = roomResponse.json();
    fixtures.push({
      token: identity.token,
      playerId: identity.player.player_id,
      roomCode: room.room_code,
    });

    // Keep registration below the gateway's 10-requests/second rate limit.
    sleep(0.15);
  }

  return fixtures;
}

export default function (fixtures) {
  const fixture = fixtures[__VU - 1];
  if (!fixture) {
    throw new Error(`No hay sala de prueba para VU ${__VU}`);
  }

  const wsBase = BASE_URL.replace(/^http:/, 'ws:').replace(/^https:/, 'wss:');
  const url = `${wsBase}/ws/${fixture.roomCode}?token=${encodeURIComponent(fixture.token)}`;
  let actionSent = false;
  let actionScored = false;
  let playStartedAt = 0;

  const response = ws.connect(url, {}, (socket) => {
    socket.on('open', () => sessionsConnected.add(1));

    socket.on('message', (rawMessage) => {
      let message;
      try {
        message = JSON.parse(rawMessage);
      } catch {
        sessionErrors.add(1);
        socket.close();
        return;
      }

      if (message.type === 'ERROR') {
        sessionErrors.add(1);
        socket.close();
        return;
      }

      if (
        message.type === 'ROOM_STATE_UPDATE' &&
        !actionSent &&
        message.payload.host_id === fixture.playerId &&
        message.payload.cards_in_hand &&
        message.payload.cards_in_hand.length > 0
      ) {
        playStartedAt = Date.now();
        socket.send(JSON.stringify({
          type: 'PLAY_HAND',
          payload: { card_indices: [0] },
        }));
        actionSent = true;
        actionsSent.add(1);
      }

      if (
        message.type === 'HAND_SCORED' &&
        actionSent &&
        !actionScored &&
        message.payload.player_id === fixture.playerId
      ) {
        playToScored.add(Date.now() - playStartedAt);
        actionScored = true;
        actionsScored.add(1);
        socket.close();
      }
    });

    socket.on('error', () => sessionErrors.add(1));
    socket.setTimeout(() => {
      if (!actionScored) {
        sessionTimeouts.add(1);
        socket.close();
      }
    }, WS_TIMEOUT_MS);
  });

  check(response, {
    'WebSocket handshake accepted': (result) => result && result.status === 101,
  });
}
