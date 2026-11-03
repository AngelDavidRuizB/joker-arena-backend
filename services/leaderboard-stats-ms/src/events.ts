/**
 * Contrato de eventos consumidos (sección 0.7 de la guía, contracts/events.md).
 * La validación es estricta en los campos que este servicio usa y tolerante
 * con campos extra, para no romperse si el productor agrega información.
 */

export const ROUTING_KEYS = {
  playerRegistered: "auth.player.registered",
  matchEnded: "game.match.ended",
} as const;

export interface Envelope {
  event_id: string;
  type: string;
  version: number;
  occurred_at: string;
  producer: string;
  payload: unknown;
}

export interface PlayerRegistered {
  player_id: string;
  alias: string;
}

export interface MatchResult {
  player_id: string;
  score: number;
  won: boolean;
  /** Opcional: si el productor lo envía, se guarda como alias de respaldo. */
  alias?: string;
  /** Opcional: marca al oponente bot para excluirlo del ranking. */
  is_bot?: boolean;
}

export interface MatchEnded {
  match_id: string;
  room_code: string;
  results: MatchResult[];
  is_bot_match: boolean;
}

/** Evento mal formado: no tiene sentido reintentarlo. */
export class InvalidEventError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "InvalidEventError";
  }
}

/** Identificador reservado para el oponente bot cuando el productor no envía is_bot. */
export const BOT_PLAYER_ID = "bot";

type Obj = Record<string, unknown>;

function isObj(v: unknown): v is Obj {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function str(o: Obj, key: string, ctx: string): string {
  const v = o[key];
  if (typeof v !== "string" || v.trim() === "") {
    throw new InvalidEventError(`${ctx}: el campo "${key}" debe ser un texto no vacío`);
  }
  return v;
}

export function parseEnvelope(raw: unknown): Envelope {
  if (!isObj(raw)) throw new InvalidEventError("El mensaje no es un objeto JSON");
  const version = raw.version;
  if (typeof version !== "number" || !Number.isInteger(version) || version < 1) {
    throw new InvalidEventError('envelope: el campo "version" debe ser un entero >= 1');
  }
  if (!("payload" in raw)) throw new InvalidEventError('envelope: falta el campo "payload"');
  const occurredAt = str(raw, "occurred_at", "envelope");
  if (Number.isNaN(Date.parse(occurredAt))) {
    throw new InvalidEventError('envelope: "occurred_at" no es una fecha ISO-8601 válida');
  }
  return {
    event_id: str(raw, "event_id", "envelope"),
    type: str(raw, "type", "envelope"),
    version,
    occurred_at: occurredAt,
    producer: str(raw, "producer", "envelope"),
    payload: raw.payload,
  };
}

export function parsePlayerRegistered(payload: unknown): PlayerRegistered {
  const ctx = ROUTING_KEYS.playerRegistered;
  if (!isObj(payload)) throw new InvalidEventError(`${ctx}: payload inválido`);
  return { player_id: str(payload, "player_id", ctx), alias: str(payload, "alias", ctx) };
}

export function parseMatchEnded(payload: unknown): MatchEnded {
  const ctx = ROUTING_KEYS.matchEnded;
  if (!isObj(payload)) throw new InvalidEventError(`${ctx}: payload inválido`);
  const rawResults = payload.results;
  if (!Array.isArray(rawResults) || rawResults.length === 0) {
    throw new InvalidEventError(`${ctx}: "results" debe ser una lista con al menos un jugador`);
  }
  const seen = new Set<string>();
  const results = rawResults.map((r, i): MatchResult => {
    const rctx = `${ctx} results[${i}]`;
    if (!isObj(r)) throw new InvalidEventError(`${rctx}: debe ser un objeto`);
    const player_id = str(r, "player_id", rctx);
    if (seen.has(player_id)) throw new InvalidEventError(`${rctx}: jugador repetido "${player_id}"`);
    seen.add(player_id);
    if (typeof r.score !== "number" || !Number.isFinite(r.score) || r.score < 0) {
      throw new InvalidEventError(`${rctx}: "score" debe ser un número >= 0`);
    }
    if (typeof r.won !== "boolean") throw new InvalidEventError(`${rctx}: "won" debe ser booleano`);
    const result: MatchResult = { player_id, score: r.score, won: r.won };
    if (typeof r.alias === "string" && r.alias.trim() !== "") result.alias = r.alias;
    const isBot = r.is_bot === true || player_id === BOT_PLAYER_ID;
    if (isBot) result.is_bot = true;
    return result;
  });
  return {
    match_id: str(payload, "match_id", ctx),
    room_code: str(payload, "room_code", ctx),
    results,
    is_bot_match: payload.is_bot_match === true,
  };
}
