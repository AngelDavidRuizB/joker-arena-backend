import { beforeEach, describe, expect, it } from "vitest";
import { InvalidEventError } from "../src/events";
import { silentLogger } from "../src/logger";
import { LeaderboardService, TOP_CACHE_KEY } from "../src/service";
import { FakeCache, InMemoryRepository, envelope, matchEnded } from "./fakes";

describe("LeaderboardService", () => {
  let repo: InMemoryRepository;
  let cache: FakeCache;
  let service: LeaderboardService;

  beforeEach(() => {
    repo = new InMemoryRepository();
    cache = new FakeCache();
    service = new LeaderboardService(repo, cache, silentLogger, 30);
  });

  it("proyecta jugadores y partidas y arma el ranking (el bot no compite)", async () => {
    await service.handleEvent(envelope("auth.player.registered", { player_id: "ana", alias: "Ana" }));
    await service.handleEvent(envelope("auth.player.registered", { player_id: "beto", alias: "Beto" }));
    await service.handleEvent(matchEnded("m1", [
      { player_id: "ana", score: 1248, won: true },
      { player_id: "beto", score: 800, won: false },
    ]));
    await service.handleEvent(matchEnded("m2", [
      { player_id: "beto", score: 2000, won: true },
      { player_id: "bot", score: 1500, won: false },
    ], { is_bot_match: true }));
    await service.handleEvent(matchEnded("m3", [
      { player_id: "ana", score: 500, won: true },
      { player_id: "bot", score: 900, won: false },
    ], { is_bot_match: true }));

    const top = await service.getTop(10);
    expect(top.cached).toBe(false);
    expect(top.players.map((p) => [p.rank, p.player_id, p.wins])).toEqual([
      [1, "ana", 2],
      [2, "beto", 1],
    ]);
    expect(top.players[0]).toMatchObject({ alias: "Ana", matches: 2, best_score: 1248, total_score: 1748 });
  });

  it("es idempotente: un game.match.ended repetido no suma dos veces", async () => {
    const evt = matchEnded("m1", [{ player_id: "ana", score: 100, won: true }]);
    expect(await service.handleEvent(evt)).toBe("processed");
    expect(await service.handleEvent({ ...evt, event_id: "otro-id" })).toBe("duplicate");
    expect((await service.getTop(10)).players[0].matches).toBe(1);
  });

  it("cache-aside: la segunda lectura sale de Redis con el TTL configurado", async () => {
    await service.handleEvent(matchEnded("m1", [{ player_id: "ana", score: 100, won: true }]));
    expect((await service.getTop(5)).cached).toBe(false);
    expect(cache.ttls.get(TOP_CACHE_KEY)).toBe(30);
    const second = await service.getTop(5);
    expect(second.cached).toBe(true);
    expect(second.players[0].player_id).toBe("ana");
  });

  it("recorta la respuesta al limit pedido aunque la caché tenga más", async () => {
    for (let i = 0; i < 5; i++) {
      await service.handleEvent(matchEnded(`m${i}`, [{ player_id: `p${i}`, score: i * 10, won: true }]));
    }
    await service.getTop(100);
    expect((await service.getTop(2)).players).toHaveLength(2);
  });

  it("invalida la caché cuando llega una partida nueva o un jugador nuevo", async () => {
    await service.handleEvent(matchEnded("m1", [{ player_id: "ana", score: 100, won: true }]));
    await service.getTop(10);
    expect(cache.store.has(TOP_CACHE_KEY)).toBe(true);
    await service.handleEvent(matchEnded("m2", [{ player_id: "beto", score: 300, won: true }]));
    expect(cache.store.has(TOP_CACHE_KEY)).toBe(false);
    await service.getTop(10);
    await service.handleEvent(envelope("auth.player.registered", { player_id: "ana", alias: "Ana María" }));
    expect(cache.store.has(TOP_CACHE_KEY)).toBe(false);
    expect((await service.getTop(10)).players.find((p) => p.player_id === "ana")?.alias).toBe("Ana María");
  });

  it("si Redis está caído sigue respondiendo desde Neo4j", async () => {
    await service.handleEvent(matchEnded("m1", [{ player_id: "ana", score: 100, won: true }]));
    cache.down = true;
    const top = await service.getTop(10);
    expect(top.cached).toBe(false);
    expect(top.players).toHaveLength(1);
    // Procesar eventos tampoco falla por Redis.
    expect(await service.handleEvent(matchEnded("m2", [{ player_id: "ana", score: 1, won: false }]))).toBe("processed");
  });

  it("recalcula si el valor en caché está corrupto", async () => {
    await service.handleEvent(matchEnded("m1", [{ player_id: "ana", score: 100, won: true }]));
    cache.store.set(TOP_CACHE_KEY, "{no es json");
    expect((await service.getTop(10)).cached).toBe(false);
  });

  it("ignora tipos de evento que no le corresponden", async () => {
    expect(await service.handleEvent(envelope("game.hand.scored", {}))).toBe("ignored");
  });

  it("rechaza versiones de evento desconocidas", async () => {
    await expect(service.handleEvent(envelope("game.match.ended", {}, { version: 2 }))).rejects.toThrow(InvalidEventError);
  });

  it("devuelve estadísticas y rivales frecuentes de un jugador", async () => {
    await service.handleEvent(envelope("auth.player.registered", { player_id: "ana", alias: "Ana" }));
    await service.handleEvent(matchEnded("m1", [{ player_id: "ana", score: 100, won: true }, { player_id: "bot", score: 50, won: false }]));
    await service.handleEvent(matchEnded("m2", [{ player_id: "ana", score: 300, won: false }, { player_id: "bot", score: 400, won: true }]));
    await service.handleEvent(matchEnded("m3", [{ player_id: "ana", score: 200, won: true }, { player_id: "beto", score: 10, won: false }]));
    const stats = await service.getPlayer("ana");
    expect(stats).toMatchObject({ matches: 3, wins: 2, losses: 1, best_score: 300, total_score: 600, win_rate: 0.667 });
    expect(stats?.rivals[0]).toEqual({ player_id: "bot", alias: null, is_bot: true, matches: 2 });
    expect(await service.getPlayer("nadie")).toBeNull();
  });
});
