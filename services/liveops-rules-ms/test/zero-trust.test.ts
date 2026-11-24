/**
 * P3 · Escenario 5 — Zero Trust: liveops valida por su cuenta firma, emisor,
 * vencimiento y rol admin antes de aceptar cambios. El GET sigue abierto.
 */
import { Writable } from "node:stream";
import pino from "pino";
import { beforeEach, describe, expect, it } from "vitest";
import { buildApp, type HealthChecks } from "../src/app";
import { AuthError, createAdminGuard, JwksClient, verifyAdminToken } from "../src/auth";
import type { EventPublisher, RulesetCache } from "../src/infra";
import { silentLogger } from "../src/logger";
import { defaultValues, type Ruleset, type RulesetValues } from "../src/ruleset";
import { LiveopsService } from "../src/service";
import type { RulesetRepository } from "../src/store";
import { adminClaims, adminHeaders, attackerKeys, bearer, fakeJwksFetch, signToken, testGuard, testJwks } from "./jwt-helpers";

class MemRepo implements RulesetRepository {
  rows: Ruleset[] = [{ version: 1, updated_at: "2026-11-03T00:00:00.000Z", updated_by: "seed", ...defaultValues() }];
  async current() { return this.rows.at(-1) ?? null; }
  async insertNext(expected: number, v: RulesetValues, by: string) {
    if ((this.rows.at(-1)?.version ?? 0) !== expected) return null;
    const rs = { version: expected + 1, updated_at: new Date().toISOString(), updated_by: by, ...structuredClone(v) };
    this.rows.push(rs);
    return rs;
  }
  async history(limit: number) { return [...this.rows].reverse().slice(0, limit); }
  async ping() {}
  async close() {}
}
const cache: RulesetCache = { get: async () => null, set: async () => undefined, ping: async () => undefined, close: async () => undefined };
const pub: EventPublisher = { publish: async () => true, connected: true };
const health: HealthChecks = { postgres: async () => "ok", redis: async () => "ok", rabbitmq: async () => "ok" };
const payload = () => ({ expected_version: 1, ...defaultValues() });

describe("Escenario 5 · autorización en liveops-rules-ms", () => {
  let repo: MemRepo;
  const app = (guard = testGuard(), logger?: pino.Logger) =>
    buildApp(new LiveopsService(repo, cache, pub, silentLogger), health, guard, logger);
  const put = (headers: Record<string, string> = {}, a = app()) =>
    a.inject({ method: "PUT", url: "/liveops/ruleset", payload: payload(), headers });

  beforeEach(() => {
    repo = new MemRepo();
  });

  it("GET del ruleset e historial siguen abiertos (el motor lo consulta sin token)", async () => {
    expect((await app().inject({ method: "GET", url: "/liveops/ruleset" })).statusCode).toBe(200);
    expect((await app().inject({ method: "GET", url: "/liveops/ruleset/history" })).statusCode).toBe(200);
    expect((await app().inject({ method: "GET", url: "/health" })).statusCode).toBe(200);
  });

  it("PUT y POST sin token → 401 y el ruleset no cambia", async () => {
    const r = await put();
    expect(r.statusCode).toBe(401);
    expect(r.headers["www-authenticate"]).toContain("Bearer");
    expect((await app().inject({ method: "POST", url: "/liveops/happy-hour", payload: { enabled: true } })).statusCode).toBe(401);
    expect((await app().inject({ method: "POST", url: "/liveops/jokers/duo/toggle" })).statusCode).toBe(401);
    expect(repo.rows).toHaveLength(1);
  });

  it("token admin válido → 200 y se guarda con el alias del token", async () => {
    const r = await put(adminHeaders({ alias: "Jaisson" }));
    expect(r.statusCode).toBe(200);
    expect(repo.rows.at(-1)?.updated_by).toBe("Jaisson");
  });

  it.each([
    ["firmado con otra llave", () => signToken(adminClaims(), { key: attackerKeys.privateKey })],
    ["vencido", () => signToken(adminClaims({ exp: Math.floor(Date.now() / 1000) - 120 }))],
    ["sin exp", () => signToken(adminClaims({ exp: undefined }))],
    ["de otro emisor", () => signToken(adminClaims({ iss: "otro" }))],
    ["con alg none", () => signToken(adminClaims(), { alg: "none" })],
    ["con alg HS256", () => signToken(adminClaims(), { alg: "HS256" })],
    ["con kid desconocido", () => signToken(adminClaims(), { kid: "otro" })],
    ["malformado", () => "abc.def"],
    ["alterado (rol de jugador cambiado a admin)", () => {
      const [h, , s] = signToken(adminClaims({ role: "player" })).split(".");
      return `${h}.${Buffer.from(JSON.stringify(adminClaims())).toString("base64url")}.${s}`;
    }],
    ["sin firma verificable (formato del P2: x.<payload>.y)", () => `x.${Buffer.from(JSON.stringify(adminClaims())).toString("base64url")}.y`],
  ])("token %s → 401", async (_name, make) => {
    expect((await put(bearer(make()))).statusCode).toBe(401);
    expect(repo.rows).toHaveLength(1);
  });

  it("token válido de jugador (sin rol admin) → 403", async () => {
    expect((await put(adminHeaders({ role: "player" }))).statusCode).toBe(403);
    expect(repo.rows).toHaveLength(1);
  });

  it("sesión cerrada en auth (jti revocado en Redis) → 401", async () => {
    const a = app(testGuard(async (jti) => jti === "jti-1"));
    expect((await put(adminHeaders(), a)).statusCode).toBe(401);
  });

  it("si Redis falla al consultar revocación, la firma ya validada basta (como en auth)", async () => {
    const a = app(testGuard(async () => { throw new Error("redis down"); }));
    expect((await put(adminHeaders(), a)).statusCode).toBe(200);
  });

  it("JWKS con reintentos: auth arranca después que liveops y aun así se valida", async () => {
    const f = fakeJwksFetch(3);
    const a = app(createAdminGuard({ keys: testJwks(f) }));
    expect((await put(adminHeaders(), a)).statusCode).toBe(200);
    expect(f.calls()).toBe(4);
  });

  it("auth caído del todo → 503 (fail closed: no se acepta el cambio)", async () => {
    const down = new JwksClient({ logger: silentLogger, maxAttempts: 2, sleep: async () => undefined, fetchImpl: async () => { throw new Error("ECONNREFUSED"); } });
    expect((await put(adminHeaders(), app(createAdminGuard({ keys: down })))).statusCode).toBe(503);
    expect(repo.rows).toHaveLength(1);
  });

  it("las llaves se cachean: varias peticiones, una sola descarga del JWKS", async () => {
    const f = fakeJwksFetch();
    const a = app(createAdminGuard({ keys: testJwks(f) }));
    await put(adminHeaders(), a);
    await a.inject({ method: "POST", url: "/liveops/happy-hour", payload: { enabled: true, global_mult: 2 }, headers: adminHeaders() });
    expect(f.calls()).toBe(1);
  });

  it("el rechazo queda en el log con el motivo, sin el token", async () => {
    const lines: string[] = [];
    const sink = new Writable({ write(chunk, _e, cb) { lines.push(chunk.toString()); cb(); } });
    const token = signToken(adminClaims({ role: "player" }));
    await put(bearer(token), app(testGuard(), pino({ level: "warn" }, sink)));
    const log = lines.join("");
    expect(log).toContain('"reason":"rol_no_admin"');
    expect(log).toContain("cambio rechazado por autorización");
    for (const part of token.split(".")) expect(log).not.toContain(part);
  });

  it("verifyAdminToken devuelve los claims del admin", async () => {
    const c = await verifyAdminToken(signToken(adminClaims()), testJwks());
    expect(c).toMatchObject({ role: "admin", alias: "admin", iss: "jokerarena-auth" });
    await expect(verifyAdminToken(signToken(adminClaims({ role: "player" })), testJwks())).rejects.toBeInstanceOf(AuthError);
  });
});
