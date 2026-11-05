# Protocolo WebSocket del juego

`ws://localhost:8080/ws/{code}?token=<jwt>` → gateway → `game-engine-ms`.
Se conserva el protocolo del Prototipo 1; lo nuevo es la autenticación por JWT y el campo `ruleset_version`.

Todos los mensajes: `{"type": "<TIPO>", "payload": { ... }}`.

## Conexión

1. El gateway rechaza con HTTP 401 un token inválido, vencido o de una sesión cerrada (subpetición a auth-identity-ms); después el motor valida el JWT (firma RS256 contra el JWKS). El jugador es `sub`/`alias` del token.
2. Si la sala no está activa en memoria, la consulta a rooms-lobby-ms (`GET /rooms/{code}`) y crea la sesión. El anfitrión es quien creó la sala; el resto entra como espectador.
3. El servidor envía un `ROOM_STATE_UPDATE` inicial.

Si el motor se reinicia a media partida, la sesión continúa con el mismo `match_id`, puntajes y contadores guardados en `game.matches.progress`; las cartas no se guardan, así que se reparte una mano nueva. Una partida sin clientes durante `ROOM_IDLE_TIMEOUT` (15 min por defecto) termina con `reason: ABANDONED`.

## Cliente → servidor

| type | payload | Efecto |
|---|---|---|
| `JOIN_ROOM` | `{ "room_code": "JKR-AB23" }` | Reenvía el estado actual (opcional; el estado ya llega al conectar). |
| `PLAY_HAND` | `{ "card_indices": [0, 2, 5] }` | Juega 1 a 5 cartas (índices de la mano, base 0). Solo el anfitrión. |
| `DISCARD_CARDS` | `{ "card_indices": [1, 3] }` | Descarta 1 a 5 cartas. Solo el anfitrión. |

## Servidor → cliente

| type | Cuándo | payload (campos principales) |
|---|---|---|
| `ROOM_STATE_UPDATE` | al conectar y tras cada acción | `room_code, match_id, status (WAITING\|IN_PROGRESS\|FINISHED), target_score, hands_left, hands_total, discards_left, discards_total, current_score, deck_count, active_jokers, cards_in_hand[], hand_number, with_bot, host_id, players[], bot_score, bot_hands_left, bot_hand_number, bot_discards_left, bot_hand[]` |
| `HAND_SCORED` | tras una jugada (anfitrión o bot) | `room_code, match_id, player_id ("bot" para el bot), hand_number, accumulated_score, timestamp, poker_hand, base_chips, base_mult, card_chips, cards_played[], jokers[] {id, name, effect, enabled, triggered, chips_added, mult_added, mult_factor}, discards_left, final_chips, final_mult, score, ruleset_version` |
| `BOT_ACTION` | antes de que el bot modifique su mano | `room_code, action (PLAY\|DISCARD), card_indices[], hand_number` |
| `MATCH_FINISHED` | fin de la partida | `room_code, match_id, winner_id, result (WIN\|LOSE, del anfitrión), final_score, target_score, target_reached, reason (TARGET_SCORE_REACHED\|HANDS_EXHAUSTED\|ABANDONED), hands_played` |
| `ERROR` | rechazo | `code, message` |

Orden por ronda: `HAND_SCORED` (anfitrión) → `ROOM_STATE_UPDATE` → `BOT_ACTION` → (`HAND_SCORED` del bot si jugó) → `ROOM_STATE_UPDATE` → `MATCH_FINISHED` si terminó.

## Códigos de error

`UNAUTHORIZED` (token ausente o inválido), `ROOM_NOT_FOUND`, `MATCH_FINISHED`, `STORAGE_UNAVAILABLE` (el lobby no responde), `NOT_ALLOWED` (un espectador intentó jugar), `INVALID_SELECTION`, `INVALID_MESSAGE`, `NO_HANDS_LEFT`, `NO_DISCARDS_LEFT`, `ROOM_FULL`.
Tras `UNAUTHORIZED`, `ROOM_NOT_FOUND` o `MATCH_FINISHED` el cliente no debe reconectar.
