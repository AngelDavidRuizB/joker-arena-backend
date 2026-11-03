import { beforeEach, describe, expect, it } from "vitest";
import { buildApp, type HealthChecks } from "../src/app";
import type { EventPublisher, RulesetCache } from "../src/infra";
import { silentLogger } from "../src/logger";
import { defaultValues, type Ruleset, type RulesetValues, validateValues, ValidationError } from "../src/ruleset";
import { LiveopsService } from "../src/service";
import type { RulesetRepository } from "../src/store";

class MemRepo implements RulesetRepository {
  rows: Ruleset[] = [];
  down = false;
  constructor(seed = true) {
    if (seed) this.rows.push({ version: 1, updated_at: "2026-11-03T00:00:00.000Z", updated_by: "seed", ...defaultValues() });
  }
  private g() {
    if (this.down) throw new Error("pg down");
  }
  async current() {
    this.g();
    return this.rows.at(-1) ?? null;
  }
  async insertNext(expected: number, v: RulesetValues, by: string) {
    this.g();
    if ((this.rows.at(-1)?.version ?? 0) !== expected) return null;
    const rs = { version: expected + 1, updated_at: new Date().toISOString(), updated_by: by, ...structuredClone(v) };
    this.rows.push(rs);
    return rs;
  }
  async history(limit: number) {
    return [...this.rows].reverse().slice(0, limit);
  }
  async ping() {
    this.g();
  }
  async close() {}
}

class MemCache implements RulesetCache {
  value: Ruleset | null = null;
  down = false;
  async get() {
    if (this.down) throw new Error("redis down");
    return this.value;
  }
  async set(rs: Ruleset) {
    if (this.down) throw new Error("redis down");
    this.value = rs;
  }
  async ping() {}
  async close() {}
}

class MemPublisher implements EventPublisher {
  sent: Array<{ key: string; payload: Ruleset }> = [];
  connected = true;
  async publish(key: string, payload: unknown) {
    if (!this.connected) return false;
    this.sent.push({ key, payload: payload as Ruleset });
    return true;
  }
}

const body = (expected: number, mutate: (v: RulesetValues) => void = () => undefined) => {
  const v = defaultValues();
  mutate(v);
  return { expected_version: expected, ...v };
};

describe("validateValues", () => {
  it("acepta los valores por defecto", () => {
    expect(validateValues(defaultValues())).toEqual(defaultValues());
  });
  it.each([
    (v: any) => (v.jokers.banner.chips_per_discard = 201),
    (v: any) => (v.jokers.classic.mult_bonus = -1),
    (v: any) => (v.jokers.duo.mult_factor = 0),
    (v: any) => (v.happy_hour.global_mult = 6),
    (v: any) => (v.jokers.banner.chips_per_discard = 2.5),
    (v: any) => (v.jokers.duo.enabled = "si"),
    (v: any) => delete v.happy_hour,
  ])("rechaza valores fuera de contrato (%#)", (mutate) => {
    const v = defaultValues();
    mutate(v);
    expect(() => validateValues(v)).toThrow(ValidationError);
  });
});

describe("API de liveops", () => {
  let repo: MemRepo;
  let cache: MemCache;
  let pub: MemPublisher;
  const health: HealthChecks = { postgres: async () => "ok", redis: async () => "ok", rabbitmq: async () => "ok" };
  const app = () => buildApp(new LiveopsService(repo, cache, pub, silentLogger), health);

  beforeEach(() => {
    repo = new MemRepo();
    cache = new MemCache();
    pub = new MemPublisher();
  });

  it("GET devuelve la v1 sembrada y la cachea", async () => {
    const res = await app().inject({ method: "GET", url: "/liveops/ruleset" });
    expect(res.json()).toMatchObject({ version: 1, jokers: { banner: { chips_per_discard: 30 } } });
    expect(cache.value?.version).toBe(1);
  });

  it("GET usa PostgreSQL si Redis está caído", async () => {
    cache.down = true;
    expect((await app().inject({ method: "GET", url: "/liveops/ruleset" })).json().version).toBe(1);
  });

  it("PUT con banner 45 crea la v2, actualiza la caché y publica exactamente un evento", async () => {
    const res = await app().inject({ method: "PUT", url: "/liveops/ruleset", payload: body(1, (v) => (v.jokers.banner.chips_per_discard = 45)) });
    expect(res.statusCode).toBe(200);
    expect(res.json()).toMatchObject({ version: 2, published: true, jokers: { banner: { chips_per_discard: 45 } } });
    expect(cache.value?.version).toBe(2);
    expect(pub.sent).toHaveLength(1);
    expect(pub.sent[0]).toMatchObject({ key: "liveops.ruleset.updated", payload: { version: 2 } });
  });

  it("PUT con una versión vieja responde 409 y no publica", async () => {
    await app().inject({ method: "PUT", url: "/liveops/ruleset", payload: body(1) });
    const res = await app().inject({ method: "PUT", url: "/liveops/ruleset", payload: body(1) });
    expect(res.statusCode).toBe(409);
    expect(res.json()).toMatchObject({ error: "version_conflict", current_version: 2 });
    expect(pub.sent).toHaveLength(1);
  });

  it("PUT con valores fuera de rango responde 400 con el detalle", async () => {
    const res = await app().inject({ method: "PUT", url: "/liveops/ruleset", payload: body(1, (v) => (v.jokers.duo.mult_factor = 11)) });
    expect(res.statusCode).toBe(400);
    expect(res.json().details[0]).toContain("jokers.duo.mult_factor");
    expect(repo.rows).toHaveLength(1);
  });

  it("toggle de un Joker y happy hour generan nuevas versiones", async () => {
    const t = await app().inject({ method: "POST", url: "/liveops/jokers/duo/toggle" });
    expect(t.json()).toMatchObject({ version: 2, jokers: { duo: { enabled: false } } });
    const h = await app().inject({ method: "POST", url: "/liveops/happy-hour", payload: { enabled: true, global_mult: 2 } });
    expect(h.json()).toMatchObject({ version: 3, happy_hour: { enabled: true, global_mult: 2 }, jokers: { duo: { enabled: false } } });
    expect((await app().inject({ method: "POST", url: "/liveops/jokers/nada/toggle" })).statusCode).toBe(404);
    expect((await app().inject({ method: "POST", url: "/liveops/happy-hour", payload: { global_mult: 9 } })).statusCode).toBe(400);
  });

  it("guarda aunque el broker esté caído y lo informa", async () => {
    pub.connected = false;
    const res = await app().inject({ method: "PUT", url: "/liveops/ruleset", payload: body(1) });
    expect(res.json()).toMatchObject({ version: 2, published: false });
  });

  it("registra quién hizo el cambio a partir del JWT reenviado", async () => {
    const token = `x.${Buffer.from(JSON.stringify({ alias: "Nicole" })).toString("base64url")}.y`;
    await app().inject({ method: "PUT", url: "/liveops/ruleset", payload: body(1), headers: { authorization: `Bearer ${token}` } });
    expect(repo.rows.at(-1)?.updated_by).toBe("Nicole");
    const hist = await app().inject({ method: "GET", url: "/liveops/ruleset/history" });
    expect(hist.json().versions.map((v: Ruleset) => v.version)).toEqual([2, 1]);
  });

  it("responde 503 si PostgreSQL está caído y /health lo refleja", async () => {
    repo.down = true;
    const res = await app().inject({ method: "PUT", url: "/liveops/ruleset", payload: body(1) });
    expect(res.statusCode).toBe(503);
    const h = await buildApp(new LiveopsService(repo, cache, pub, silentLogger), { ...health, postgres: async () => "down" }).inject({ method: "GET", url: "/health" });
    expect(h.json()).toMatchObject({ status: "degraded", service: "liveops-rules-ms", deps: { postgres: "down" } });
  });
});
