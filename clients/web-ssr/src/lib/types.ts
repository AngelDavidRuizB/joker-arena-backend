// Contratos del sistema consumidos por el cliente web (ver contracts/ del backend).
// - C2 WebSocket del juego: game-engine-ms  -> protocolo de sala y puntuación.
// - REST a través del API Gateway: auth-identity-ms, rooms-lobby-ms, leaderboard-stats-ms.

export type Suit = 'SPADES' | 'HEARTS' | 'CLUBS' | 'DIAMONDS';
export type RoomStatus = 'WAITING' | 'IN_PROGRESS' | 'FINISHED';

export interface Card {
  rank: string;
  suit: Suit;
  chips: number;
}

export interface PlayerInfo {
  id: string;
  alias: string;
  is_bot: boolean;
}

/** Payload of ROOM_STATE_UPDATE. */
export interface RoomState {
  room_code: string;
  match_id: string;
  status: RoomStatus;
  target_score: number;
  hands_left: number;
  /** Totales de la partida (aditivos; el FE usa 20/3 si faltan). */
  hands_total?: number;
  discards_total?: number;
  discards_left: number;
  current_score: number;
  deck_count: number;
  active_jokers: string[];
  cards_in_hand: Card[];
  hand_number: number;
  with_bot: boolean;
  host_id: string;
  players: PlayerInfo[];
  /** Estado del bot (presente solo en partidas con bot; ausente/cero si no hay bot). */
  bot_score?: number;
  bot_hands_left?: number;
  bot_discards_left?: number;
  bot_hand_number?: number;
  /** Mano completa de 8 cartas del bot (presente solo en partidas con bot). */
  bot_hand?: Card[];
}

/** Payload of BOT_ACTION: the bot's chosen move, announced before it mutates its state. */
export interface BotAction {
  room_code: string;
  action: 'PLAY' | 'DISCARD';
  /** 1..5 zero-based indices (ascending) into bot_hand as last broadcast in ROOM_STATE_UPDATE. */
  card_indices: number[];
  hand_number: number;
}

export interface ScoredCard {
  rank: string;
  suit: Suit;
  chips: number;
  scored: boolean;
}

export interface JokerEffect {
  id: string;
  name: string;
  effect: string;
  triggered: boolean;
  chips_added: number;
  mult_added: number;
  mult_factor: number;
}

/** Payload of HAND_SCORED: the full algebraic audit of one played hand. */
export interface HandScored {
  room_code: string;
  match_id: string;
  player_id: string;
  hand_number: number;
  accumulated_score: number;
  timestamp: string;
  poker_hand: string;
  base_chips: number;
  base_mult: number;
  card_chips: number;
  cards_played: ScoredCard[];
  jokers: JokerEffect[];
  discards_left: number;
  final_chips: number;
  final_mult: number;
  score: number;
}

/** Payload of MATCH_FINISHED. */
export interface MatchFinished {
  room_code: string;
  match_id: string;
  winner_id: string;
  result: 'WIN' | 'LOSE';
  final_score: number;
  target_score: number;
  target_reached: boolean;
  reason: 'TARGET_SCORE_REACHED' | 'HANDS_EXHAUSTED';
  hands_played: number;
}

export interface ServerError {
  code: string;
  message: string;
}

export type ServerMessage =
  | { type: 'ROOM_STATE_UPDATE'; payload: RoomState }
  | { type: 'HAND_SCORED'; payload: HandScored }
  | { type: 'MATCH_FINISHED'; payload: MatchFinished }
  | { type: 'BOT_ACTION'; payload: BotAction }
  | { type: 'ERROR'; payload: ServerError };

export type ConnectionStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed';

/* ------------------------------------------------------------------ REST P2 */

/** Respuesta de POST /api/auth/register y /api/auth/login (auth-identity-ms). */
export interface AuthResponse {
  token: string;
  token_type: string;
  expires_in: number;
  player: {
    player_id: string;
    alias: string;
    role: 'player' | 'admin';
  };
}

export type LobbyMode = 'bot' | 'pvp';
export type LobbyDifficulty = 'easy' | 'normal' | 'hard';
export type LobbyStatus = 'open' | 'in_progress' | 'closed';

/** Sala tal como la devuelve rooms-lobby-ms (GET/POST /api/rooms). */
export interface LobbyRoom {
  room_code: string;
  mode: LobbyMode;
  difficulty: LobbyDifficulty;
  target_score: number;
  status: LobbyStatus;
  created_by: string;
  created_at: string;
  players: { player_id: string; alias: string }[];
}

/** Entrada del ranking global (leaderboard-stats-ms, GET /api/leaderboard/top). */
export interface RankedPlayer {
  rank: number;
  player_id: string;
  alias: string | null;
  matches: number;
  wins: number;
  best_score: number;
  total_score: number;
}
