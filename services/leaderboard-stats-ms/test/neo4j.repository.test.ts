import { describe, expect, it } from "vitest";
import * as Q from "../src/graph/cypher";
import { Neo4jLeaderboardRepository, type GraphDriver } from "../src/graph/neo4j.repository";

type Call = { query: string; params: Record<string, unknown> };

/** Doble del driver: registra las consultas y devuelve registros preparados. */
function fakeDriver(responses: (query: string) => Array<Record<string, unknown>>, failOn?: (q: string) => Error | null) {
  const calls: Call[] = [];
  const run = async (query: string, params: Record<string, unknown> = {}) => {
    calls.push({ query, params });
    const err = failOn?.(query);
    if (err) throw err;
    return { records: responses(query).map((r) => ({ get: (k: string) => r[k] })) };
  };
  const tx = { run };
  const session = () => ({
    run,
    executeWrite: async <T>(fn: (t: typeof tx) => Promise<T>) => fn(tx),
    executeRead: async <T>(fn: (t: typeof tx) => Promise<T>) => fn(tx),
    close: async () => undefined,
  });
  const driver = { session, verifyConnectivity: async () => ({}), close: async () => undefined } as unknown as GraphDriver;
  return { driver, calls };
}

const match = {
  match_id: "m1",
  room_code: "JKR-AB12",
  is_bot_match: true,
  results: [{ player_id: "ana", score: 1248, won: true }, { player_id: "bot", score: 10, won: false, is_bot: true }],
};

describe("Neo4jLeaderboardRepository", () => {
  it("crea la partida, las relaciones PLAYED y FACED con los parámetros del evento", async () => {
    const { driver, calls } = fakeDriver((q) => (q === Q.MATCH_EXISTS ? [{ exists: false }] : []));
    const created = await new Neo4jLeaderboardRepository(driver).recordMatch(match, "2026-11-03T18:00:00Z");
    expect(created).toBe(true);
    expect(calls.map((c) => c.query)).toEqual([Q.MATCH_EXISTS, Q.CREATE_MATCH, Q.CREATE_FACED]);
    expect(calls[1].params).toEqual({
      match_id: "m1",
      room_code: "JKR-AB12",
      is_bot_match: true,
      ended_at: "2026-11-03T18:00:00Z",
      results: [
        { player_id: "ana", score: 1248, won: true, alias: null, is_bot: false },
        { player_id: "bot", score: 10, won: false, alias: null, is_bot: true },
      ],
    });
  });

  it("no vuelve a escribir una partida existente", async () => {
    const { driver, calls } = fakeDriver(() => [{ exists: true }]);
    expect(await new Neo4jLeaderboardRepository(driver).recordMatch(match, "2026-11-03T18:00:00Z")).toBe(false);
    expect(calls).toHaveLength(1);
  });

  it("trata la violación de unicidad (carrera entre réplicas) como duplicado", async () => {
    const { driver } = fakeDriver(
      () => [{ exists: false }],
      (q) => (q === Q.CREATE_MATCH ? Object.assign(new Error("dup"), { code: "Neo.ClientError.Schema.ConstraintValidationFailed" }) : null),
    );
    expect(await new Neo4jLeaderboardRepository(driver).recordMatch(match, "2026-11-03T18:00:00Z")).toBe(false);
  });

  it("propaga otros errores para que el evento se reintente", async () => {
    const { driver } = fakeDriver(() => [{ exists: false }], (q) => (q === Q.CREATE_MATCH ? new Error("boom") : null));
    await expect(new Neo4jLeaderboardRepository(driver).recordMatch(match, "x")).rejects.toThrow("boom");
  });

  it("mapea el ranking y calcula derrotas y win_rate del jugador", async () => {
    const { driver } = fakeDriver((q) => {
      if (q === Q.TOP) return [{ player_id: "ana", alias: "Ana", matches: 3, wins: 2, best_score: 1248, total_score: 2000 }];
      if (q === Q.PLAYER_STATS) {
        return [{ player_id: "ana", alias: "Ana", matches: 3, wins: 2, best_score: 1248, total_score: 2000, rivals: [{ player_id: "bot", alias: null, is_bot: true, matches: 2 }] }];
      }
      return [];
    });
    const repo = new Neo4jLeaderboardRepository(driver);
    expect(await repo.top(10)).toEqual([{ player_id: "ana", alias: "Ana", matches: 3, wins: 2, best_score: 1248, total_score: 2000 }]);
    expect(await repo.playerStats("ana")).toEqual({
      player_id: "ana", alias: "Ana", matches: 3, wins: 2, losses: 1, best_score: 1248, total_score: 2000, win_rate: 0.667,
      rivals: [{ player_id: "bot", alias: null, is_bot: true, matches: 2 }],
    });
  });

  it("devuelve null si el jugador no existe", async () => {
    const { driver } = fakeDriver(() => []);
    expect(await new Neo4jLeaderboardRepository(driver).playerStats("nadie")).toBeNull();
  });

  it("crea las restricciones definidas en db/neo4j/constraints.cypher", async () => {
    const { driver, calls } = fakeDriver(() => []);
    await new Neo4jLeaderboardRepository(driver).ensureSchema();
    expect(calls.map((c) => c.query)).toEqual(Q.CONSTRAINTS);
  });
});
