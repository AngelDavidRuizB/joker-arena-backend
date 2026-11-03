import { describe, expect, it } from "vitest";
import { silentLogger } from "../src/logger";
import { handleDelivery } from "../src/messaging/consumer";
import { LeaderboardService } from "../src/service";
import { FakeCache, InMemoryRepository, envelope, matchEnded } from "./fakes";

const setup = () => {
  const repo = new InMemoryRepository();
  const service = new LeaderboardService(repo, new FakeCache(), silentLogger, 30);
  return { repo, service };
};
const buf = (v: unknown) => Buffer.from(typeof v === "string" ? v : JSON.stringify(v));

describe("handleDelivery", () => {
  it("confirma (ack) un evento válido", async () => {
    const { service, repo } = setup();
    expect(await handleDelivery(buf(matchEnded("m1", [{ player_id: "a", score: 1, won: true }])), service, silentLogger)).toBe("ack");
    expect(repo.matches.has("m1")).toBe(true);
  });

  it("confirma también duplicados y tipos ajenos (no deben quedar en la cola)", async () => {
    const { service } = setup();
    const evt = matchEnded("m1", [{ player_id: "a", score: 1, won: true }]);
    await handleDelivery(buf(evt), service, silentLogger);
    expect(await handleDelivery(buf(evt), service, silentLogger)).toBe("ack");
    expect(await handleDelivery(buf(envelope("lobby.room.created", {})), service, silentLogger)).toBe("ack");
  });

  it("descarta (reject) JSON inválido o eventos fuera de contrato", async () => {
    const { service } = setup();
    expect(await handleDelivery(buf("{roto"), service, silentLogger)).toBe("reject");
    expect(await handleDelivery(buf(matchEnded("m1", [])), service, silentLogger)).toBe("reject");
  });

  it("reintenta (retry) cuando Neo4j falla", async () => {
    const { service, repo } = setup();
    repo.failWith = Object.assign(new Error("Neo4j caído"), { code: "ServiceUnavailable" });
    expect(await handleDelivery(buf(matchEnded("m1", [{ player_id: "a", score: 1, won: true }])), service, silentLogger)).toBe("retry");
  });
});
