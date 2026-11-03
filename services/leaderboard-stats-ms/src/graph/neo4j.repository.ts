import neo4j, { type Driver } from "neo4j-driver";
import type { MatchEnded, PlayerRegistered } from "../events";
import * as Q from "./cypher";
import { MAX_RIVALS, type LeaderboardRepository, type PlayerStats, type Rival, type TopEntry } from "./repository";

/** Subconjunto del driver que usamos; permite inyectar un doble de prueba. */
export type GraphDriver = Pick<Driver, "session" | "verifyConnectivity" | "close">;

export function createNeo4jDriver(url: string, user: string, password: string): Driver {
  return neo4j.driver(url, neo4j.auth.basic(user, password), {
    // Devuelve enteros de JS en lugar de objetos neo4j.Integer (los valores aquí son pequeños).
    disableLosslessIntegers: true,
    connectionAcquisitionTimeout: 5_000,
    connectionTimeout: 5_000,
  });
}

function isConstraintViolation(err: unknown): boolean {
  const code = (err as { code?: string } | null)?.code ?? "";
  return code === "Neo.ClientError.Schema.ConstraintValidationFailed";
}

const num = (v: unknown): number => (typeof v === "number" ? v : Number(v ?? 0));

export class Neo4jLeaderboardRepository implements LeaderboardRepository {
  constructor(private readonly driver: GraphDriver) {}

  async ensureSchema(): Promise<void> {
    const session = this.driver.session({ defaultAccessMode: neo4j.session.WRITE });
    try {
      for (const stmt of Q.CONSTRAINTS) await session.run(stmt);
    } finally {
      await session.close();
    }
  }

  async upsertPlayer(player: PlayerRegistered): Promise<void> {
    const session = this.driver.session({ defaultAccessMode: neo4j.session.WRITE });
    try {
      await session.executeWrite((tx) => tx.run(Q.UPSERT_PLAYER, { ...player }));
    } finally {
      await session.close();
    }
  }

  async recordMatch(match: MatchEnded, endedAt: string): Promise<boolean> {
    const session = this.driver.session({ defaultAccessMode: neo4j.session.WRITE });
    try {
      return await session.executeWrite(async (tx) => {
        const existing = await tx.run(Q.MATCH_EXISTS, { match_id: match.match_id });
        if (existing.records[0]?.get("exists") === true) return false;
        await tx.run(Q.CREATE_MATCH, {
          match_id: match.match_id,
          room_code: match.room_code,
          is_bot_match: match.is_bot_match,
          ended_at: endedAt,
          results: match.results.map((r) => ({
            player_id: r.player_id,
            score: r.score,
            won: r.won,
            alias: r.alias ?? null,
            is_bot: r.is_bot ?? false,
          })),
        });
        await tx.run(Q.CREATE_FACED, { match_id: match.match_id });
        return true;
      });
    } catch (err) {
      // Dos réplicas procesando el mismo evento a la vez: la restricción de unicidad gana.
      if (isConstraintViolation(err)) return false;
      throw err;
    } finally {
      await session.close();
    }
  }

  async top(limit: number): Promise<TopEntry[]> {
    const session = this.driver.session({ defaultAccessMode: neo4j.session.READ });
    try {
      const res = await session.executeRead((tx) => tx.run(Q.TOP, { limit: neo4j.int(limit) }));
      return res.records.map((rec) => ({
        player_id: rec.get("player_id") as string,
        alias: (rec.get("alias") as string | null) ?? null,
        matches: num(rec.get("matches")),
        wins: num(rec.get("wins")),
        best_score: num(rec.get("best_score")),
        total_score: num(rec.get("total_score")),
      }));
    } finally {
      await session.close();
    }
  }

  async playerStats(playerId: string): Promise<PlayerStats | null> {
    const session = this.driver.session({ defaultAccessMode: neo4j.session.READ });
    try {
      const res = await session.executeRead((tx) =>
        tx.run(Q.PLAYER_STATS, { player_id: playerId, max_rivals: neo4j.int(MAX_RIVALS) }),
      );
      const rec = res.records[0];
      if (!rec) return null;
      const matches = num(rec.get("matches"));
      const wins = num(rec.get("wins"));
      const rivals = ((rec.get("rivals") as Record<string, unknown>[] | null) ?? []).map(
        (r): Rival => ({
          player_id: r.player_id as string,
          alias: (r.alias as string | null) ?? null,
          is_bot: r.is_bot === true,
          matches: num(r.matches),
        }),
      );
      return {
        player_id: rec.get("player_id") as string,
        alias: (rec.get("alias") as string | null) ?? null,
        matches,
        wins,
        losses: matches - wins,
        best_score: num(rec.get("best_score")),
        total_score: num(rec.get("total_score")),
        win_rate: matches === 0 ? 0 : Math.round((wins / matches) * 1000) / 1000,
        rivals,
      };
    } finally {
      await session.close();
    }
  }

  async ping(): Promise<void> {
    await this.driver.verifyConnectivity();
  }

  async close(): Promise<void> {
    await this.driver.close();
  }
}
