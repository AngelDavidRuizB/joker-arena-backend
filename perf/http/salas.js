// =============================================================================
// JokerArena P3 · D1/D2 · Carga sobre las salas (lobby)
//   GET /api/rooms?limit=20  y  GET /api/rooms/{code}
//   gateway (auth_request → auth-identity-ms /auth/verify) → rooms-lobby-ms → PostgreSQL
//
// Es la ruta protegida más usada: cada petición hace una subpetición de
// validación del JWT al Servicio de Identidad antes de llegar a Salas, así que
// mide gateway + identidad + salas + PostgreSQL.
//
// setup(): registra POOL jugadores (por defecto 20) de forma gradual, porque el
// registro tiene el límite de 10 peticiones por segundo por IP, y crea una sala.
//
// Ejemplo:
//   k6 run -e BASE_URL=http://localhost:8080 -e VUS=50 perf/http/salas.js
// =============================================================================
import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, opcionesCarga, registrarJugador } from './lib/config.js';

export const options = opcionesCarga('salas');

const POOL = Number(__ENV.POOL || 20);

export function setup() {
  const tokens = [];
  for (let i = 0; i < POOL; i++) {
    tokens.push(registrarJugador(http, sleep, 'k6s').token);
    sleep(0.15); // < 10 registros por segundo
  }
  const sala = http.post(`${BASE_URL}/api/rooms`, JSON.stringify({ mode: 'bot' }), {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${tokens[0]}` },
    tags: { name: 'setup_create_room' },
  });
  if (sala.status !== 201) {
    throw new Error(`No se pudo crear la sala de prueba: HTTP ${sala.status} ${sala.body}`);
  }
  return { tokens, code: sala.json('room_code') };
}

export default function (data) {
  const token = data.tokens[(__VU - 1) % data.tokens.length];
  const params = (name) => ({ headers: { Authorization: `Bearer ${token}` }, tags: { name } });

  const lista = http.get(`${BASE_URL}/api/rooms?limit=20`, params('GET /api/rooms'));
  check(lista, { 'lista de salas 200': (r) => r.status === 200 });

  const sala = http.get(`${BASE_URL}/api/rooms/${data.code}`, params('GET /api/rooms/{code}'));
  check(sala, { 'detalle de sala 200': (r) => r.status === 200 });
}
