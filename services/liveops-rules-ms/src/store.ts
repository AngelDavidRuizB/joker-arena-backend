import { Pool } from "pg";
import type { Ruleset, RulesetValues } from "./ruleset";

/** Persistencia append-only de versiones (PostgreSQL, esquema liveops). */
export interface RulesetRepository {
  current(): Promise<Ruleset | null>;
  /** Inserta la versión expectedVersion+1. Devuelve null si otra escritura ganó (conflicto). */
  insertNext(expectedVersion: number, values: RulesetValues, updatedBy: string): Promise<Ruleset | null>;
  history(limit: number): Promise<Ruleset[]>;
  ping(): Promise<void>;
  close(): Promise<void>;
}

interface Row {
  version: number;
  payload: RulesetValues;
  updated_by: string;
  updated_at: Date;
}

const toRuleset = (r: Row): Ruleset => ({
  version: r.version,
  updated_at: r.updated_at.toISOString(),
  updated_by: r.updated_by,
  jokers: r.payload.jokers,
  happy_hour: r.payload.happy_hour,
});

export class PostgresRulesetRepository implements RulesetRepository {
  private readonly pool: Pool;

  constructor(connectionString: string) {
    this.pool = new Pool({ connectionString, max: 5, connectionTimeoutMillis: 3_000 });
    this.pool.on("error", () => undefined); // conexiones caídas: se reabren en la siguiente consulta
  }

  async current(): Promise<Ruleset | null> {
    const res = await this.pool.query<Row>(
      "SELECT version, payload, updated_by, updated_at FROM liveops.liveops_rulesets ORDER BY version DESC LIMIT 1",
    );
    return res.rows[0] ? toRuleset(res.rows[0]) : null;
  }

  async insertNext(expectedVersion: number, values: RulesetValues, updatedBy: string): Promise<Ruleset | null> {
    // La PK sobre version hace que dos ediciones concurrentes de la misma versión no puedan ganar ambas.
    const res = await this.pool.query<Row>(
      `INSERT INTO liveops.liveops_rulesets (version, payload, updated_by)
       SELECT $1 + 1, $2::jsonb, $3
       WHERE (SELECT COALESCE(MAX(version), 0) FROM liveops.liveops_rulesets) = $1
       ON CONFLICT (version) DO NOTHING
       RETURNING version, payload, updated_by, updated_at`,
      [expectedVersion, JSON.stringify(values), updatedBy],
    );
    return res.rows[0] ? toRuleset(res.rows[0]) : null;
  }

  async history(limit: number): Promise<Ruleset[]> {
    const res = await this.pool.query<Row>(
      "SELECT version, payload, updated_by, updated_at FROM liveops.liveops_rulesets ORDER BY version DESC LIMIT $1",
      [limit],
    );
    return res.rows.map(toRuleset);
  }

  async ping(): Promise<void> {
    await this.pool.query("SELECT 1");
  }

  async close(): Promise<void> {
    await this.pool.end();
  }
}
