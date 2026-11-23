// =============================================================================
// JokerArena P3 · D1/D2 · Carga sobre el ranking público
//   GET /api/leaderboard/top?limit=10  →  gateway → leaderboard-stats-ms → Redis/Neo4j
//
// La respuesta trae "cached": true cuando sale de Redis (cache-aside, TTL 30 s).
// La métrica ranking_cache_hit dice qué fracción de respuestas salió de la caché:
//   - modo "cache":     se espera ~100 % (solo la primera petición va a Neo4j).
//   - modo "sin-cache": se espera 0 % (el servicio se levanta sin Redis, ver README).
//
// Ejemplo:
//   k6 run -e BASE_URL=http://localhost:8080 -e VUS=50 -e MODO=cache perf/http/ranking.js
// =============================================================================
import http from 'k6/http';
import { check } from 'k6';
import { Rate } from 'k6/metrics';
import { BASE_URL, opcionesCarga } from './lib/config.js';

export const options = opcionesCarga('ranking');

const cacheHit = new Rate('ranking_cache_hit');

export default function () {
  const res = http.get(`${BASE_URL}/api/leaderboard/top?limit=10`, {
    tags: { name: 'GET /api/leaderboard/top' },
  });
  const ok = check(res, {
    'ranking responde 200': (r) => r.status === 200,
    'ranking trae jugadores': (r) => r.status === 200 && Array.isArray(r.json('players')),
  });
  if (ok) {
    cacheHit.add(res.json('cached') === true);
  }
}
