/**
 * Prueba de integración contra Neo4j, Redis y RabbitMQ reales.
 * Solo corre con INTEGRATION=1 (ver README: "Prueba de integración").
 * Publica eventos en el exchange como lo harían auth-identity y game-engine,
 * y verifica el resultado a través de la API HTTP del servicio en ejecución.
 */
import amqp from "amqplib";
import { describe, expect, it } from "vitest";

const RUN = process.env.INTEGRATION === "1";
const BASE = process.env.LEADERBOARD_URL ?? "http://localhost:8086";
const AMQP = process.env.RABBITMQ_URL ?? "amqp://guest:guest@localhost:5672/";
const EXCHANGE = "jokerarena.events";
const runId = Date.now().toString(36);

async function publish(routingKey: string, payload: unknown, eventId = `${runId}-${Math.random()}`) {
  const conn = await amqp.connect(AMQP);
  const ch = await conn.createConfirmChannel();
  await ch.assertExchange(EXCHANGE, "topic", { durable: true });
  const env = { event_id: eventId, type: routingKey, version: 1, occurred_at: new Date().toISOString(), producer: "integration-test", payload };
  ch.publish(EXCHANGE, routingKey, Buffer.from(JSON.stringify(env)), { contentType: "application/json", persistent: true });
  await ch.waitForConfirms();
  await conn.close();
}

async function eventually<T>(fn: () => Promise<T>, ok: (v: T) => boolean, timeoutMs = 15_000): Promise<T> {
  const start = Date.now();
  let last: T | undefined;
  while (Date.now() - start < timeoutMs) {
    last = await fn();
    if (ok(last)) return last;
    await new Promise((r) => setTimeout(r, 300));
  }
  throw new Error(`No se cumplió la condición a tiempo. Último valor: ${JSON.stringify(last)}`);
}

const getJson = async (path: string) => {
  const res = await fetch(`${BASE}${path}`);
  return { status: res.status, body: (await res.json()) as any };
};

describe.runIf(RUN)("integración con Neo4j + Redis + RabbitMQ", () => {
  const ana = `ana-${runId}`;
  const beto = `beto-${runId}`;

  it("el servicio reporta todas sus dependencias en ok", async () => {
    const h = await eventually(() => getJson("/health"), (r) => r.body.status === "ok");
    expect(h.body.deps).toEqual({ neo4j: "ok", redis: "ok", rabbitmq: "ok" });
  });

  it("proyecta eventos del broker en el ranking y en las estadísticas", async () => {
    await publish("auth.player.registered", { player_id: ana, alias: "Ana Integración" });
    await publish("auth.player.registered", { player_id: beto, alias: "Beto Integración" });
    const m1 = { match_id: `m1-${runId}`, room_code: "JKR-TEST", is_bot_match: false, results: [
      { player_id: ana, score: 99999, won: true }, { player_id: beto, score: 500, won: false } ] };
    await publish("game.match.ended", m1);
    await publish("game.match.ended", m1, `dup-${runId}`); // duplicado: no debe contar dos veces
    await publish("game.match.ended", { match_id: `m2-${runId}`, room_code: "JKR-BOT1", is_bot_match: true, results: [
      { player_id: ana, score: 30000, won: true }, { player_id: "bot", score: 100, won: false } ] });

    const stats = await eventually(() => getJson(`/leaderboard/players/${ana}`), (r) => r.status === 200 && r.body.matches === 2);
    expect(stats.body).toMatchObject({ alias: "Ana Integración", matches: 2, wins: 2, losses: 0, best_score: 99999, total_score: 129999, win_rate: 1 });
    expect(stats.body.rivals.map((r: any) => r.player_id).sort()).toEqual(["bot", beto].sort());

    const top = await getJson("/leaderboard/top?limit=100");
    const ids = top.body.players.map((p: any) => p.player_id);
    expect(ids).toContain(ana);
    expect(ids).not.toContain("bot");
    expect(top.body.players[ids.indexOf(ana)].rank).toBeLessThan(top.body.players[ids.indexOf(beto)].rank);

    const again = await getJson("/leaderboard/top?limit=100");
    expect(again.body.cached).toBe(true);
  });

  it("un mensaje inválido no bloquea la cola", async () => {
    await publish("game.match.ended", { match_id: `bad-${runId}`, results: [] });
    const m3 = { match_id: `m3-${runId}`, room_code: "JKR-TEST", is_bot_match: false, results: [{ player_id: beto, score: 1, won: true }] };
    await publish("game.match.ended", m3);
    const s = await eventually(() => getJson(`/leaderboard/players/${beto}`), (r) => r.body.matches === 2);
    expect(s.body.wins).toBe(1);
  });
});
