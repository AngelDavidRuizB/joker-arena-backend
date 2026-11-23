// =============================================================================
// JokerArena P3 · D1 · Evidencia del límite de peticiones del login
//
// El gateway limita /api/auth/* a 10 peticiones por segundo por IP con una
// ráfaga de 20 (limit_req ... burst=20 nodelay). Por eso el login NO sirve para
// medir capacidad: se prueba aparte, con tasa fija (open model), para mostrar
// que el límite funciona. Las peticiones que exceden el límite reciben 503
// ({"error":"service_unavailable"}) y nunca llegan al Servicio de Identidad.
//
// Esperado con RATE=40 durante 20 s (800 peticiones):
//   aceptadas ≈ 20 (ráfaga) + 10/s × 20 s = 220   ·   rechazadas (503) ≈ 580
//
// Ejemplo:
//   k6 run -e BASE_URL=http://localhost:8080 -e RATE=40 -e DURATION=20s perf/http/login_limite.js
// =============================================================================
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';
import { BASE_URL, MODO, registrarJugador } from './lib/config.js';

const RATE = Number(__ENV.RATE || 40);
const DURATION = __ENV.DURATION || '20s';

export const options = {
  tags: { prueba: 'login_limite', modo: MODO, rate: String(RATE) },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  insecureSkipTLSVerify: true,
  scenarios: {
    login_limite: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 50,
      maxVUs: 200,
      startTime: '3s', // deja que el cubo del límite se vacíe tras el setup
    },
  },
  thresholds: {
    // Que el límite actúe: al menos una petición rechazada con 503.
    login_rechazado_503: ['count>0'],
  },
};

const aceptados = new Counter('login_aceptado_200');
const rechazados = new Counter('login_rechazado_503');
const otros = new Counter('login_otro_codigo');

export function setup() {
  return { alias: registrarJugador(http, sleep, 'k6l').alias };
}

export default function (data) {
  const res = http.post(`${BASE_URL}/api/auth/login`, JSON.stringify({ alias: data.alias }), {
    headers: { 'Content-Type': 'application/json' },
    tags: { name: 'POST /api/auth/login' },
    // 503 es la respuesta esperada del límite: no se cuenta como fallo de k6.
    responseCallback: http.expectedStatuses(200, 503),
  });
  if (res.status === 200) aceptados.add(1);
  else if (res.status === 503) rechazados.add(1);
  else otros.add(1);
  check(res, { 'login 200 o 503 (límite)': (r) => r.status === 200 || r.status === 503 });
}
