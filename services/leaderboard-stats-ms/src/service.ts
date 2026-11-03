import type { Cache } from "./cache";
import {
  InvalidEventError,
  ROUTING_KEYS,
  parseEnvelope,
  parseMatchEnded,
  parsePlayerRegistered,
} from "./events";
import type { LeaderboardRepository, PlayerStats, TopEntry } from "./graph/repository";
import type { Logger } from "./logger";

/** Clave Redis definida en la sección 0.6 de la guía. */
export const TOP_CACHE_KEY = "leaderboard:top";
/** Se cachea siempre el top completo y se recorta según el limit pedido. */
export const MAX_TOP_LIMIT = 100;

export type HandleResult = "processed" | "duplicate" | "ignored";

export interface RankedEntry extends TopEntry {
  rank: number;
}

export interface TopResponse {
  players: RankedEntry[];
  cached: boolean;
}

/**
 * Lógica del servicio: proyecta eventos del juego en el grafo (vista materializada)
 * y sirve el ranking con caché.
 */
export class LeaderboardService {
  constructor(
    private readonly repo: LeaderboardRepository,
    private readonly cache: Cache,
    private readonly logger: Logger,
    private readonly cacheTtlSeconds: number,
  ) {}

  /** Procesa un evento crudo del broker. Lanza InvalidEventError si está mal formado. */
  async handleEvent(raw: unknown): Promise<HandleResult> {
    const env = parseEnvelope(raw);
    if (env.version !== 1) {
      throw new InvalidEventError(`Versión de evento no soportada: ${env.type} v${env.version}`);
    }
    switch (env.type) {
      case ROUTING_KEYS.playerRegistered: {
        const player = parsePlayerRegistered(env.payload);
        await this.repo.upsertPlayer(player);
        await this.invalidateTop();
        this.logger.info({ event_id: env.event_id, player_id: player.player_id }, "jugador registrado en el grafo");
        return "processed";
      }
      case ROUTING_KEYS.matchEnded: {
        const match = parseMatchEnded(env.payload);
        const created = await this.repo.recordMatch(match, env.occurred_at);
        if (!created) {
          this.logger.info({ event_id: env.event_id, match_id: match.match_id }, "partida ya registrada: evento duplicado ignorado");
          return "duplicate";
        }
        await this.invalidateTop();
        this.logger.info({ event_id: env.event_id, match_id: match.match_id }, "partida registrada en el grafo");
        return "processed";
      }
      default:
        this.logger.warn({ event_id: env.event_id, type: env.type }, "tipo de evento no manejado: se ignora");
        return "ignored";
    }
  }

  async getTop(limit: number): Promise<TopResponse> {
    const cachedValue = await this.safeCache(() => this.cache.get(TOP_CACHE_KEY));
    if (cachedValue) {
      try {
        const all = JSON.parse(cachedValue) as RankedEntry[];
        return { players: all.slice(0, limit), cached: true };
      } catch {
        this.logger.warn({ key: TOP_CACHE_KEY }, "valor de caché corrupto: se recalcula");
      }
    }
    const rows = await this.repo.top(MAX_TOP_LIMIT);
    const ranked = rows.map((r, i) => ({ rank: i + 1, ...r }));
    await this.safeCache(() => this.cache.set(TOP_CACHE_KEY, JSON.stringify(ranked), this.cacheTtlSeconds));
    return { players: ranked.slice(0, limit), cached: false };
  }

  async getPlayer(playerId: string): Promise<PlayerStats | null> {
    return this.repo.playerStats(playerId);
  }

  private async invalidateTop(): Promise<void> {
    await this.safeCache(() => this.cache.del(TOP_CACHE_KEY));
  }

  /** Redis es una optimización: si falla, se registra y se sigue sin caché. */
  private async safeCache<T>(op: () => Promise<T>): Promise<T | null> {
    try {
      return await op();
    } catch (err) {
      this.logger.warn({ err: (err as Error).message }, "caché no disponible: se continúa sin caché");
      return null;
    }
  }
}
