import 'server-only';

import { cookies } from 'next/headers';
import { SESSION_COOKIE } from '../../store/session';

/**
 * URL interna del API Gateway dentro de `jokerarena-net`. El navegador nunca usa esta
 * constante: las páginas se renderizan en el servidor de Next y es él quien llama al gateway.
 */
const GATEWAY = process.env.GATEWAY_INTERNAL_URL ?? 'http://localhost:8080';

/** Resultado de una consulta SSR: nunca lanza; la página decide qué pintar. */
export type GatewayResult<T> = { ok: true; data: T } | { ok: false; message: string };

/** ¿Hay una sesión en la cookie? No valida el token: eso lo hace el gateway. */
export function hasSession(): boolean {
  return Boolean(cookies().get(SESSION_COOKIE)?.value);
}

/**
 * GET renderizado en el servidor contra el API Gateway con el JWT de la cookie `ja_token`.
 *
 * Con `withAuth` la ausencia de token es un error de sesión (la página redirige a `/`);
 * sin él la ruta es pública (por ejemplo el ranking). `cache: 'no-store'` mantiene el
 * renderizado dinámico: el HTML siempre refleja el estado real de los servicios.
 */
export async function gatewayGet<T>(path: string, withAuth = false): Promise<GatewayResult<T>> {
  const token = cookies().get(SESSION_COOKIE)?.value;
  if (withAuth && !token) return { ok: false, message: 'Inicia sesión para ver esta página.' };

  const base = GATEWAY.replace(/\/$/, '');
  const url = `${base}${path.startsWith('/') ? path : `/${path}`}`;
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (token) headers.Authorization = `Bearer ${token}`;

  try {
    const res = await fetch(url, { headers, cache: 'no-store' });
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as { message?: string } | null;
      return { ok: false, message: body?.message ?? `El servicio respondió ${res.status}.` };
    }
    return { ok: true, data: (await res.json()) as T };
  } catch {
    return { ok: false, message: 'No se pudo contactar el API Gateway.' };
  }
}
