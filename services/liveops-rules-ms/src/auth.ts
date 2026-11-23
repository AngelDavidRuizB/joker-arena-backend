/**
 * P3 · Escenario 5 — Zero Trust / autorización en el servicio.
 *
 * liveops-rules-ms ya no confía en que el gateway validó el token: verifica por su
 * cuenta la firma (JWKS de auth-identity-ms), el emisor, el vencimiento y el rol admin
 * antes de aceptar cualquier cambio. Los GET quedan abiertos (el motor consulta el
 * ruleset al arrancar sin token).
 */
import { createPublicKey, verify as cryptoVerify, type JsonWebKey, type KeyObject } from "node:crypto";
import type { FastifyReply, FastifyRequest } from "fastify";
import type { Logger } from "./logger";

// Valores fijos del P3 (sin variables nuevas en .env ni en compose.yml).
export const JWKS_URL = "http://jokerarena-auth-identity-ms:8082/.well-known/jwks.json";
export const ISSUER = "jokerarena-auth";
export const ADMIN_ROLE = "admin";
const CLOCK_SKEW_S = 30;
const CACHE_TTL_MS = 10 * 60_000;
const MIN_REFRESH_MS = 30_000;
const FETCH_TIMEOUT_MS = 3_000;

export interface AdminClaims {
  sub: string;
  alias?: string;
  role: string;
  iss: string;
  exp: number;
  jti?: string;
}

export class AuthError extends Error {
  constructor(public readonly status: 401 | 403 | 503, public readonly reason: string) {
    super(reason);
    this.name = "AuthError";
  }
}

// --------------------------------------------------------------------------- JWKS

type Fetch = (url: string, init?: { signal?: AbortSignal }) => Promise<{ ok: boolean; status: number; json(): Promise<unknown> }>;

export interface KeySource {
  getKey(kid: string | undefined): Promise<KeyObject | null>;
}

export interface JwksOptions {
  url?: string;
  fetchImpl?: Fetch;
  logger: Logger;
  maxAttempts?: number;
  retryBaseMs?: number;
  retryMaxMs?: number;
  sleep?: (ms: number) => Promise<void>;
}

/**
 * Llaves públicas de auth con reintentos (backoff exponencial), caché y recarga
 * cuando llega un `kid` desconocido. No depende del orden de arranque.
 */
export class JwksClient implements KeySource {
  private keys = new Map<string, KeyObject>();
  private fetchedAt = 0;
  private inflight: Promise<void> | null = null;
  private readonly url: string;
  private readonly fetchImpl: Fetch;
  private readonly maxAttempts: number;
  private readonly retryBaseMs: number;
  private readonly retryMaxMs: number;
  private readonly sleep: (ms: number) => Promise<void>;

  constructor(private readonly opts: JwksOptions) {
    this.url = opts.url ?? JWKS_URL;
    this.fetchImpl = opts.fetchImpl ?? (globalThis.fetch as unknown as Fetch);
    this.maxAttempts = opts.maxAttempts ?? 8;
    this.retryBaseMs = opts.retryBaseMs ?? 500;
    this.retryMaxMs = opts.retryMaxMs ?? 15_000;
    this.sleep = opts.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms)));
  }

  /** Precarga en segundo plano; nunca tumba el proceso. */
  warmUp(): void {
    this.refresh().catch((e: Error) =>
      this.opts.logger.warn({ err: e.message }, "JWKS de auth aún no disponible; se reintenta en la primera petición"),
    );
  }

  async getKey(kid: string | undefined): Promise<KeyObject | null> {
    const age = Date.now() - this.fetchedAt;
    if (this.keys.size === 0 || age > CACHE_TTL_MS) await this.refresh();
    else if (kid && !this.keys.has(kid) && age > MIN_REFRESH_MS) await this.refresh(); // rotación de llaves
    if (kid) return this.keys.get(kid) ?? null;
    return this.keys.size === 1 ? [...this.keys.values()][0] : null;
  }

  private refresh(): Promise<void> {
    this.inflight ??= this.load().finally(() => {
      this.inflight = null;
    });
    return this.inflight;
  }

  private async load(): Promise<void> {
    let delay = this.retryBaseMs;
    for (let attempt = 1; ; attempt++) {
      try {
        this.keys = await this.fetchOnce();
        this.fetchedAt = Date.now();
        this.opts.logger.info({ keys: this.keys.size }, "JWKS de auth cargado");
        return;
      } catch (e) {
        this.opts.logger.warn({ attempt, max: this.maxAttempts, err: (e as Error).message }, "no se pudo obtener el JWKS de auth");
        if (attempt >= this.maxAttempts) throw e;
        await this.sleep(delay);
        delay = Math.min(delay * 2, this.retryMaxMs);
      }
    }
  }

  private async fetchOnce(): Promise<Map<string, KeyObject>> {
    const ctrl = new AbortController();
    const t = setTimeout(() => ctrl.abort(), FETCH_TIMEOUT_MS);
    try {
      const res = await this.fetchImpl(this.url, { signal: ctrl.signal });
      if (!res.ok) throw new Error(`JWKS HTTP ${res.status}`);
      const body = (await res.json()) as { keys?: Array<JsonWebKey & { kid?: string; use?: string; alg?: string }> };
      const next = new Map<string, KeyObject>();
      for (const jwk of body.keys ?? []) {
        // Solo llaves RSA de firma RS256: es lo único que emite auth-identity-ms.
        if (jwk.kty !== "RSA" || (jwk.use && jwk.use !== "sig") || (jwk.alg && jwk.alg !== "RS256")) continue;
        next.set(jwk.kid ?? "", createPublicKey({ key: jwk, format: "jwk" }));
      }
      if (next.size === 0) throw new Error("JWKS sin llaves RS256");
      return next;
    } finally {
      clearTimeout(t);
    }
  }
}

// --------------------------------------------------------------------- Verificación

function decodePart<T>(part: string): T {
  return JSON.parse(Buffer.from(part, "base64url").toString("utf8")) as T;
}

/** Verifica firma RS256, emisor, vencimiento y rol admin. Lanza AuthError si algo falla. */
export async function verifyAdminToken(token: string, keys: KeySource, nowMs = Date.now()): Promise<AdminClaims> {
  const parts = token.split(".");
  if (parts.length !== 3) throw new AuthError(401, "token_malformado");

  let header: { alg?: string; kid?: string };
  let claims: Partial<AdminClaims> & { nbf?: number };
  try {
    header = decodePart(parts[0]);
    claims = decodePart(parts[1]);
  } catch {
    throw new AuthError(401, "token_malformado");
  }

  // Solo RS256: se rechazan "none" y HS* (confusión de algoritmo).
  if (header.alg !== "RS256") throw new AuthError(401, "alg_no_permitido");

  let key: KeyObject | null;
  try {
    key = await keys.getKey(header.kid);
  } catch {
    throw new AuthError(503, "jwks_no_disponible"); // fail closed: sin llaves no se acepta ningún cambio
  }
  if (!key) throw new AuthError(401, "kid_desconocido");

  const ok = cryptoVerify("sha256", Buffer.from(`${parts[0]}.${parts[1]}`), key, Buffer.from(parts[2], "base64url"));
  if (!ok) throw new AuthError(401, "firma_invalida");

  const now = Math.floor(nowMs / 1000);
  if (claims.iss !== ISSUER) throw new AuthError(401, "emisor_invalido");
  if (typeof claims.exp !== "number") throw new AuthError(401, "sin_exp");
  if (now > claims.exp + CLOCK_SKEW_S) throw new AuthError(401, "token_vencido");
  if (typeof claims.nbf === "number" && now + CLOCK_SKEW_S < claims.nbf) throw new AuthError(401, "token_aun_no_valido");
  if (typeof claims.sub !== "string") throw new AuthError(401, "sin_sub");
  if (claims.role !== ADMIN_ROLE) throw new AuthError(403, "rol_no_admin");
  return claims as AdminClaims;
}

// ------------------------------------------------------------------- Guard Fastify

declare module "fastify" {
  interface FastifyRequest {
    admin?: AdminClaims;
  }
}

export type AdminGuard = (req: FastifyRequest, reply: FastifyReply) => Promise<void>;

export interface GuardOptions {
  keys: KeySource;
  /** Lista de sesiones cerradas (auth:revoked:<jti> en Redis). Opcional. */
  isRevoked?: (jti: string) => Promise<boolean>;
  now?: () => number;
}

const BODY = {
  401: { error: "unauthorized", message: "Se requiere un token de administrador válido." },
  403: { error: "forbidden", message: "Solo un administrador puede cambiar el ruleset." },
  503: { error: "auth_unavailable", message: "No se pudo verificar el token: auth-identity-ms no responde." },
} as const;

export function createAdminGuard(opts: GuardOptions): AdminGuard {
  const now = opts.now ?? Date.now;
  return async (req, reply) => {
    try {
      const m = /^Bearer\s+(\S+)$/i.exec(req.headers.authorization ?? "");
      if (!m) throw new AuthError(401, "sin_token");
      const claims = await verifyAdminToken(m[1], opts.keys, now());
      if (claims.jti && opts.isRevoked) {
        let revoked = false;
        try {
          revoked = await opts.isRevoked(claims.jti);
        } catch (e) {
          // Igual que auth: Redis es auxiliar; la firma y el vencimiento ya se validaron.
          req.log.warn({ err: (e as Error).message }, "no se pudo consultar la lista de sesiones cerradas");
        }
        if (revoked) throw new AuthError(401, "sesion_cerrada");
      }
      req.admin = claims;
    } catch (e) {
      const err = e instanceof AuthError ? e : new AuthError(401, "error_verificacion");
      // Se registra el motivo, nunca el token.
      req.log.warn(
        { status: err.status, reason: err.reason, method: req.method, url: req.url, ip: req.ip, xff: req.headers["x-forwarded-for"] },
        "cambio rechazado por autorización",
      );
      if (err.status === 401) reply.header("WWW-Authenticate", 'Bearer realm="liveops"');
      return reply.code(err.status).send(BODY[err.status]);
    }
  };
}
