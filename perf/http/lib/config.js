// =============================================================================
// JokerArena P3 · D1/D2 · Configuración común de los scripts HTTP de k6
// Responsable: José Alejandro Solano Garcés
//
// Variables de entorno (todas opcionales):
//   BASE_URL   URL de entrada. P2: http://localhost:8080 · P3: https://localhost
//   VUS        usuarios virtuales simultáneos (10, 50, 100, 200)
//   DURATION   duración de la carga sostenida (por defecto 30s)
//   MODO       etiqueta libre que queda en los resultados (cache, sin-cache, http, https…)
//   NO_REUSE   "1" para abrir una conexión nueva en cada petición (peor caso de TLS)
// =============================================================================

export const BASE_URL = (__ENV.BASE_URL || 'http://localhost:8080').replace(/\/+$/, '');
export const VUS = Number(__ENV.VUS || 10);
export const DURATION = __ENV.DURATION || '30s';
export const MODO = __ENV.MODO || 'default';

/** Opciones comunes: carga cerrada (cada VU repite sin pausa) durante DURATION. */
export function opcionesCarga(nombre) {
  return {
    // Datos del entorno de la prueba, guardados en el JSON del resumen.
    tags: { prueba: nombre, modo: MODO, vus: String(VUS) },
    summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
    noConnectionReuse: __ENV.NO_REUSE === '1',
    // Certificados locales del P3 (CA propia): equivale a --insecure-skip-tls-verify.
    insecureSkipTLSVerify: true,
    scenarios: {
      [nombre]: {
        executor: 'constant-vus',
        vus: VUS,
        duration: DURATION,
        gracefulStop: '10s',
      },
    },
    // Umbrales informativos: no detienen la prueba (abortOnFail=false), pero el
    // resumen dice si se cumplieron. p95 < 500 ms y menos de 1 % de errores.
    thresholds: {
      http_req_failed: [{ threshold: 'rate<0.01', abortOnFail: false }],
      http_req_duration: [{ threshold: 'p(95)<500', abortOnFail: false }],
    },
  };
}

// Contraseña de los jugadores de prueba. Desde el P3 el registro y el login piden
// {"user", "password"}; "alias" se envía también para que los scripts sigan
// funcionando contra el P2, que solo leía ese campo.
export const CLAVE_PRUEBA = 'perf-Clave-2026';

/**
 * Registra un jugador respetando el límite del gateway en /api/auth/*
 * (10 por segundo por IP, ráfaga de 20). Reintenta si recibe 503 (límite).
 */
export function registrarJugador(http, sleep, prefijo) {
  for (let intento = 0; intento < 10; intento++) {
    const alias = `${prefijo}${Date.now().toString(36)}${Math.floor(Math.random() * 1e4)}`.slice(0, 30);
    const res = http.post(`${BASE_URL}/api/auth/register`, JSON.stringify({ user: alias, alias, password: CLAVE_PRUEBA }), {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: 'setup_register' },
    });
    if (res.status === 201) {
      return { alias, token: res.json('token') };
    }
    // 503 = límite de peticiones del gateway; 409 = alias repetido. Se espera y se reintenta.
    sleep(0.5);
  }
  throw new Error(`No se pudo registrar un jugador de prueba en ${BASE_URL}`);
}
