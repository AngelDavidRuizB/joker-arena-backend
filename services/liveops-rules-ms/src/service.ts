import type { EventPublisher, RulesetCache } from "./infra";
import { ROUTING_KEY } from "./infra";
import type { Logger } from "./logger";
import { defaultValues, type JokerId, type Ruleset, type RulesetValues, validateValues, ValidationError } from "./ruleset";
import type { RulesetRepository } from "./store";

export class VersionConflictError extends Error {
  constructor(public readonly current: number) {
    super(`La versión vigente es ${current}`);
    this.name = "VersionConflictError";
  }
}

export interface SaveResult {
  ruleset: Ruleset;
  published: boolean;
}

export class LiveopsService {
  constructor(
    private readonly repo: RulesetRepository,
    private readonly cache: RulesetCache,
    private readonly publisher: EventPublisher,
    private readonly logger: Logger,
  ) {}

  /** Cache-aside: Redis primero; si no está (o Redis falla), PostgreSQL y se vuelve a sembrar. */
  async current(): Promise<Ruleset> {
    try {
      const cached = await this.cache.get();
      if (cached) return cached;
    } catch (err) {
      this.logger.warn({ err: (err as Error).message }, "Redis no disponible: se lee de PostgreSQL");
    }
    let rs = await this.repo.current();
    if (!rs) {
      // Base vacía (no debería pasar: 40_liveops.sql siembra la v1). Se siembra aquí por robustez.
      rs = (await this.repo.insertNext(0, defaultValues(), "seed")) ?? (await this.repo.current());
      if (!rs) throw new Error("no se pudo sembrar el ruleset");
    }
    await this.safeCache(rs);
    return rs;
  }

  /** Guarda una nueva versión con control optimista, actualiza la caché y publica el cambio. */
  async save(expectedVersion: unknown, body: unknown, updatedBy: string): Promise<SaveResult> {
    if (typeof expectedVersion !== "number" || !Number.isInteger(expectedVersion) || expectedVersion < 1) {
      throw new ValidationError(["expected_version debe ser un entero >= 1 (la versión que estás editando)"]);
    }
    const values = validateValues(body);
    const saved = await this.repo.insertNext(expectedVersion, values, updatedBy);
    if (!saved) {
      const now = await this.repo.current();
      throw new VersionConflictError(now?.version ?? 0);
    }
    await this.safeCache(saved);
    const published = await this.publisher.publish(ROUTING_KEY, saved);
    if (!published) {
      this.logger.warn({ version: saved.version }, "versión guardada pero no publicada: el motor la tomará al reiniciar o en el siguiente cambio");
    }
    this.logger.info({ version: saved.version, by: updatedBy }, "nuevo ruleset");
    return { ruleset: saved, published };
  }

  async toggleJoker(id: JokerId, updatedBy: string): Promise<SaveResult> {
    const cur = await this.repo.current();
    if (!cur) throw new Error("no hay ruleset vigente");
    const values: RulesetValues = structuredClone({ jokers: cur.jokers, happy_hour: cur.happy_hour });
    values.jokers[id].enabled = !values.jokers[id].enabled;
    return this.save(cur.version, values, updatedBy);
  }

  async setHappyHour(body: unknown, updatedBy: string): Promise<SaveResult> {
    const cur = await this.repo.current();
    if (!cur) throw new Error("no hay ruleset vigente");
    const b = (typeof body === "object" && body !== null ? body : {}) as Record<string, unknown>;
    const values: RulesetValues = structuredClone({ jokers: cur.jokers, happy_hour: cur.happy_hour });
    if ("enabled" in b) values.happy_hour.enabled = b.enabled as boolean;
    if ("global_mult" in b) values.happy_hour.global_mult = b.global_mult as number;
    return this.save(cur.version, values, updatedBy);
  }

  history(limit: number): Promise<Ruleset[]> {
    return this.repo.history(limit);
  }

  private async safeCache(rs: Ruleset): Promise<void> {
    try {
      await this.cache.set(rs);
    } catch (err) {
      this.logger.warn({ err: (err as Error).message }, "no se pudo actualizar la caché");
    }
  }
}
