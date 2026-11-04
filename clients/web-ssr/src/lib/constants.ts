/** Única fuente de las metas y totales de partida en el frontend (espejo de room.go / match.go). */
export const DEFAULT_TARGET_SCORE = 22000;

export const TARGET_OPTIONS = [
  { value: DEFAULT_TARGET_SCORE, hint: 'Clásica' },
  { value: 26000, hint: 'Desafío' },
  { value: 33000, hint: 'Épica' },
] as const;

/** Respaldo del HUD cuando el servidor no envía hands_total / discards_total. */
export const DEFAULT_TOTAL_HANDS = 20;
export const DEFAULT_TOTAL_DISCARDS = 3;

/** Id del jugador bot en el protocolo (espejo de room.BotPlayerID en el backend). */
export const BOT_PLAYER_ID = 'bot';
