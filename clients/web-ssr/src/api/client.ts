import { DEFAULT_TARGET_SCORE } from '../lib/constants';
import type { AuthResponse, LobbyRoom } from '../lib/types';

/** Error del cliente REST. status 0 = no se pudo contactar el gateway. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(status: number, message: string, code = '') {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

/** Todas las rutas pasan por el gateway (/api/... reescrito por Next al gateway). */
export async function request<T>(path: string, init?: RequestInit & { token?: string }): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      ...init,
      headers: {
        'Content-Type': 'application/json',
        ...(init?.token ? { Authorization: `Bearer ${init.token}` } : {}),
        ...(init?.headers ?? {}),
      },
    });
  } catch {
    throw new ApiError(0, 'No se pudo conectar con el servidor. ¿Está encendido?');
  }
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    // cuerpo no JSON
  }
  if (!res.ok) {
    const b = (body ?? {}) as { error?: string; message?: string };
    const message =
      res.status === 503
        ? 'Un servicio no está disponible. Inténtalo de nuevo en unos segundos.'
        : b.message || `Error inesperado del servidor (${res.status}).`;
    throw new ApiError(res.status, message, b.error ?? '');
  }
  return body as T;
}

/** F1: entra con un alias. Si el alias ya existe, inicia sesión con él. */
export async function enter(alias: string, password?: string): Promise<AuthResponse> {
  const body = JSON.stringify(password ? { alias, password } : { alias });
  try {
    return await request<AuthResponse>('/api/auth/register', { method: 'POST', body });
  } catch (err) {
    if (err instanceof ApiError && err.code === 'alias_taken') {
      return request<AuthResponse>('/api/auth/login', { method: 'POST', body });
    }
    throw err;
  }
}

export type Difficulty = 'easy' | 'normal' | 'hard';

/** F2: crea una sala en rooms-lobby-ms. */
export const createRoom = (token: string, withBot: boolean, targetScore = DEFAULT_TARGET_SCORE, difficulty: Difficulty = 'normal') =>
  request<LobbyRoom>('/api/rooms', {
    method: 'POST',
    token,
    body: JSON.stringify({ mode: withBot ? 'bot' : 'pvp', target_score: targetScore, difficulty }),
  });

/** Consulta una sala para validar el código antes de entrar. */
export const getRoom = (token: string, code: string) =>
  request<LobbyRoom>(`/api/rooms/${encodeURIComponent(code)}`, { token });
