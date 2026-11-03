import type { Cache } from "../src/cache";
import type { MatchEnded, PlayerRegistered } from "../src/events";
import type { LeaderboardRepository, PlayerStats, Rival, TopEntry } from "../src/graph/repository";
import { MAX_RIVALS } from "../src/graph/repository";

interface P { player_id: string; alias: string | null; is_bot: boolean }
interface Played { player_id: string; match_id: string; score: number; won: boolean }

/** Implementación en memoria con la misma semántica que las consultas Cypher. */
export class InMemoryRepository implements LeaderboardRepository {
  players = new Map<string, P>();
  matches = new Set<string>();
  played: Played[] = [];
  failWith: Error | null = null;

  private guard(): void {
    if (this.failWith) throw this.failWith;
  }

  async ensureSchema(): Promise<void> {}

  async upsertPlayer(p: PlayerRegistered): Promise<void> {
    this.guard();
    const cur = this.players.get(p.player_id);
    this.players.set(p.player_id, { player_id: p.player_id, alias: p.alias, is_bot: cur?.is_bot ?? false });
  }

  async recordMatch(m: MatchEnded): Promise<boolean> {
    this.guard();
    if (this.matches.has(m.match_id)) return false;
    this.matches.add(m.match_id);
    for (const r of m.results) {
      const cur = this.players.get(r.player_id);
      this.players.set(r.player_id, {
        player_id: r.player_id,
        alias: cur?.alias ?? r.alias ?? null,
        is_bot: r.is_bot ?? false,
      });
      this.played.push({ player_id: r.player_id, match_id: m.match_id, score: r.score, won: r.won });
    }
    return true;
  }

  private statsOf(id: string) {
    const rows = this.played.filter((x) => x.player_id === id);
    return {
      matches: rows.length,
      wins: rows.filter((x) => x.won).length,
      best_score: rows.reduce((a, x) => Math.max(a, x.score), 0),
      total_score: rows.reduce((a, x) => a + x.score, 0),
    };
  }

  async top(limit: number): Promise<TopEntry[]> {
    this.guard();
    return [...this.players.values()]
      .filter((p) => !p.is_bot && this.played.some((x) => x.player_id === p.player_id))
      .map((p) => ({ player_id: p.player_id, alias: p.alias, ...this.statsOf(p.player_id) }))
      .sort((a, b) => b.wins - a.wins || b.best_score - a.best_score || a.matches - b.matches || a.player_id.localeCompare(b.player_id))
      .slice(0, limit);
  }

  async playerStats(id: string): Promise<PlayerStats | null> {
    this.guard();
    const p = this.players.get(id);
    if (!p) return null;
    const s = this.statsOf(id);
    const counts = new Map<string, number>();
    for (const mine of this.played.filter((x) => x.player_id === id)) {
      for (const other of this.played.filter((x) => x.match_id === mine.match_id && x.player_id !== id)) {
        counts.set(other.player_id, (counts.get(other.player_id) ?? 0) + 1);
      }
    }
    const rivals: Rival[] = [...counts.entries()]
      .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
      .slice(0, MAX_RIVALS)
      .map(([pid, n]) => ({ player_id: pid, alias: this.players.get(pid)?.alias ?? null, is_bot: this.players.get(pid)?.is_bot ?? false, matches: n }));
    return {
      player_id: id,
      alias: p.alias,
      ...s,
      losses: s.matches - s.wins,
      win_rate: s.matches === 0 ? 0 : Math.round((s.wins / s.matches) * 1000) / 1000,
      rivals,
    };
  }

  async ping(): Promise<void> {
    this.guard();
  }

  async close(): Promise<void> {}
}

export class FakeCache implements Cache {
  store = new Map<string, string>();
  ttls = new Map<string, number>();
  down = false;
  gets = 0;

  private guard(): void {
    if (this.down) throw new Error("redis down");
  }
  async get(k: string) {
    this.guard();
    this.gets += 1;
    return this.store.get(k) ?? null;
  }
  async set(k: string, v: string, ttl: number) {
    this.guard();
    this.store.set(k, v);
    this.ttls.set(k, ttl);
  }
  async del(k: string) {
    this.guard();
    this.store.delete(k);
  }
  async ping() {
    this.guard();
  }
  async close() {}
}

let seq = 0;
export function envelope(type: string, payload: unknown, overrides: Record<string, unknown> = {}) {
  seq += 1;
  return {
    event_id: `00000000-0000-4000-8000-${String(seq).padStart(12, "0")}`,
    type,
    version: 1,
    occurred_at: "2026-11-03T18:00:00Z",
    producer: "test",
    payload,
    ...overrides,
  };
}

export const matchEnded = (match_id: string, results: unknown[], extra: Record<string, unknown> = {}) =>
  envelope("game.match.ended", { match_id, room_code: "JKR-AB12", results, is_bot_match: false, ...extra });
