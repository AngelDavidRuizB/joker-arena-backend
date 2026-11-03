import type { MatchEnded, PlayerRegistered } from "../events";

/** Fila del ranking global. */
export interface TopEntry {
  player_id: string;
  alias: string | null;
  matches: number;
  wins: number;
  best_score: number;
  total_score: number;
}

export interface Rival {
  player_id: string;
  alias: string | null;
  is_bot: boolean;
  matches: number;
}

export interface PlayerStats {
  player_id: string;
  alias: string | null;
  matches: number;
  wins: number;
  losses: number;
  best_score: number;
  total_score: number;
  win_rate: number;
  rivals: Rival[];
}

/**
 * Puerto de persistencia del ranking. La implementación real usa Neo4j
 * (graph/neo4j.repository.ts); los tests usan una versión en memoria.
 */
export interface LeaderboardRepository {
  /** Crea las restricciones de unicidad (idempotente). */
  ensureSchema(): Promise<void>;
  upsertPlayer(player: PlayerRegistered): Promise<void>;
  /** Devuelve false si la partida ya estaba registrada (evento duplicado). */
  recordMatch(match: MatchEnded, endedAt: string): Promise<boolean>;
  top(limit: number): Promise<TopEntry[]>;
  playerStats(playerId: string): Promise<PlayerStats | null>;
  ping(): Promise<void>;
  close(): Promise<void>;
}

/** Máximo de rivales frecuentes devueltos por jugador. */
export const MAX_RIVALS = 5;
