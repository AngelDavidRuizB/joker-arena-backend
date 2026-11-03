import { beforeEach, describe, expect, it } from "vitest";
import { buildApp, type HealthChecks } from "../src/http/app";
import { silentLogger } from "../src/logger";
import { LeaderboardService } from "../src/service";
import { FakeCache, InMemoryRepository, matchEnded } from "./fakes";

describe("API HTTP", () => {
  let repo: InMemoryRepository;
  let service: LeaderboardService;
  let health: HealthChecks;

  beforeEach(async () => {
    repo = new InMemoryRepository();
    service = new LeaderboardService(repo, new FakeCache(), silentLogger, 30);
    health = { neo4j: async () => "ok", redis: async () => "ok", rabbitmq: async () => "ok" };
    await service.handleEvent(matchEnded("m1", [{ player_id: "ana", score: 1248, won: true }, { player_id: "beto", score: 10, won: false }]));
  });

  const app = () => buildApp({ service, health });

  it("GET /leaderboard/top devuelve el ranking con posición (snake_case)", async () => {
    const res = await app().inject({ method: "GET", url: "/leaderboard/top?limit=1" });
    expect(res.statusCode).toBe(200);
    const body = res.json();
    expect(body.players).toEqual([{ rank: 1, player_id: "ana", alias: null, matches: 1, wins: 1, best_score: 1248, total_score: 1248 }]);
    expect(typeof body.generated_at).toBe("string");
    expect(body.cached).toBe(false);
  });

  it.each(["0", "101", "abc", "2.5"])("GET /leaderboard/top?limit=%s responde 400", async (limit) => {
    const res = await app().inject({ method: "GET", url: `/leaderboard/top?limit=${limit}` });
    expect(res.statusCode).toBe(400);
    expect(res.json().error).toBe("invalid_limit");
  });

  it("GET /leaderboard/players/:id devuelve estadísticas o 404", async () => {
    const ok = await app().inject({ method: "GET", url: "/leaderboard/players/ana" });
    expect(ok.statusCode).toBe(200);
    expect(ok.json()).toMatchObject({ player_id: "ana", wins: 1, rivals: [{ player_id: "beto", matches: 1 }] });
    const missing = await app().inject({ method: "GET", url: "/leaderboard/players/nadie" });
    expect(missing.statusCode).toBe(404);
    expect(missing.json()).toMatchObject({ error: "player_not_found" });
  });

  it("responde 503 con el formato de error común si Neo4j no está disponible", async () => {
    repo.failWith = Object.assign(new Error("no hay conexión"), { code: "ServiceUnavailable" });
    const res = await app().inject({ method: "GET", url: "/leaderboard/players/ana" });
    expect(res.statusCode).toBe(503);
    expect(res.json()).toEqual({ error: "store_unavailable", message: expect.any(String) });
  });

  it("GET /health reporta ok o degraded según las dependencias", async () => {
    const ok = await app().inject({ method: "GET", url: "/health" });
    expect(ok.json()).toEqual({ status: "ok", service: "leaderboard-stats-ms", deps: { neo4j: "ok", redis: "ok", rabbitmq: "ok" } });
    health.redis = async () => { throw new Error("x"); };
    health.rabbitmq = async () => "down";
    const degraded = await app().inject({ method: "GET", url: "/health" });
    expect(degraded.statusCode).toBe(200);
    expect(degraded.json()).toMatchObject({ status: "degraded", deps: { neo4j: "ok", redis: "down", rabbitmq: "down" } });
  });

  it("rutas desconocidas responden 404 con el formato de error común", async () => {
    const res = await app().inject({ method: "GET", url: "/nada" });
    expect(res.statusCode).toBe(404);
    expect(res.json().error).toBe("not_found");
  });
});
