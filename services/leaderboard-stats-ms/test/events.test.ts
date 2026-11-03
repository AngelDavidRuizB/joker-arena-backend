import { describe, expect, it } from "vitest";
import { InvalidEventError, parseEnvelope, parseMatchEnded, parsePlayerRegistered } from "../src/events";
import { envelope } from "./fakes";

describe("parseEnvelope", () => {
  it("acepta un sobre válido y conserva el payload", () => {
    const env = parseEnvelope(envelope("auth.player.registered", { player_id: "p1", alias: "Ana" }));
    expect(env.type).toBe("auth.player.registered");
    expect(env.payload).toEqual({ player_id: "p1", alias: "Ana" });
  });

  it.each([
    ["no es objeto", "texto"],
    ["sin event_id", { ...envelope("x", {}), event_id: undefined }],
    ["version no entera", { ...envelope("x", {}), version: "1" }],
    ["fecha inválida", { ...envelope("x", {}), occurred_at: "ayer" }],
    ["sin payload", (() => { const e: Record<string, unknown> = envelope("x", {}); delete e.payload; return e; })()],
  ])("rechaza un sobre %s", (_name, raw) => {
    expect(() => parseEnvelope(raw)).toThrow(InvalidEventError);
  });
});

describe("parsePlayerRegistered", () => {
  it("exige player_id y alias", () => {
    expect(parsePlayerRegistered({ player_id: "p1", alias: "Ana" })).toEqual({ player_id: "p1", alias: "Ana" });
    expect(() => parsePlayerRegistered({ player_id: "p1" })).toThrow(InvalidEventError);
    expect(() => parsePlayerRegistered({ player_id: "", alias: "Ana" })).toThrow(InvalidEventError);
  });
});

describe("parseMatchEnded", () => {
  const ok = {
    match_id: "m1",
    room_code: "JKR-AB12",
    is_bot_match: true,
    results: [
      { player_id: "p1", score: 1248, won: true, extra: "se ignora" },
      { player_id: "bot", score: 900, won: false },
    ],
  };

  it("acepta el payload del contrato y marca al bot", () => {
    const m = parseMatchEnded(ok);
    expect(m.results[0]).toEqual({ player_id: "p1", score: 1248, won: true });
    expect(m.results[1]).toEqual({ player_id: "bot", score: 900, won: false, is_bot: true });
    expect(m.is_bot_match).toBe(true);
  });

  it("respeta is_bot explícito y el alias opcional", () => {
    const m = parseMatchEnded({ ...ok, results: [{ player_id: "x", score: 1, won: false, is_bot: true, alias: "Robo" }] });
    expect(m.results[0]).toEqual({ player_id: "x", score: 1, won: false, is_bot: true, alias: "Robo" });
  });

  it.each([
    ["results vacío", { ...ok, results: [] }],
    ["score negativo", { ...ok, results: [{ player_id: "p1", score: -1, won: true }] }],
    ["score no numérico", { ...ok, results: [{ player_id: "p1", score: "10", won: true }] }],
    ["won no booleano", { ...ok, results: [{ player_id: "p1", score: 1, won: "si" }] }],
    ["jugador repetido", { ...ok, results: [{ player_id: "p1", score: 1, won: true }, { player_id: "p1", score: 2, won: false }] }],
    ["sin match_id", { ...ok, match_id: undefined }],
  ])("rechaza %s", (_n, payload) => {
    expect(() => parseMatchEnded(payload)).toThrow(InvalidEventError);
  });
});
