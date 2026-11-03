# game-engine-ms

Motor autoritativo del juego. **Responsable:** Santiago Zamora Garzon · **Go 1.22** · puerto interno **8081**.

Mantiene cada sala en su propia goroutine (patrón actor): baraja, evaluador de manos, puntuación con Jokers y turno del bot. Es la evolución del motor del Prototipo 1 (`jokerarena-game-ms/`, que no se modificó).

## Qué cambió frente al Prototipo 1

| Antes (P1) | Ahora (P2) |
|---|---|
| Registraba jugadores (`POST /api/players`) | Lo hace **auth-identity-ms**; el motor valida el **JWT** (RS256, JWKS) |
| Creaba salas (`POST /api/rooms`) | Lo hace **rooms-lobby-ms**; el motor consulta `GET /rooms/{code}` al abrir el WebSocket |
| Bot dentro del motor | Le pide la jugada a **bot-ai-ms**; si falla o tarda > 800 ms usa la estrategia local |
| Valores de Jokers fijos | **Ruleset de LiveOps** recargado en caliente (`liveops.ruleset.updated`) |
| Telemetría sin conectar | Publica `game.match.started`, `game.hand.scored`, `game.match.ended` en RabbitMQ |
| Tablas `players/rooms/matches` | Esquema propio `game.matches` (`db/postgres/init/30_game.sql`) |

## Interfaces

- `GET /ws/{code}?token=<jwt>` — WebSocket del juego (`contracts/ws-protocol.md`).
- `GET /ruleset` — ruleset que se está aplicando y su origen (`default`, `liveops-http`, `liveops-event`).
- `GET /health` — `{"status","service","deps":{"postgres","rabbitmq"},"ruleset_version"}`.

## Eventos

Publica en `jokerarena.events`: `game.match.started`, `game.hand.scored`, `game.match.ended`.
Consume `liveops.ruleset.#` (cola `game-engine-ms.ruleset`).
Sin broker el juego sigue funcionando: los eventos se guardan en una cola en memoria (hasta 1024) y se publican al reconectar.

## Variables de entorno

`PORT`, `POSTGRES_URL`, `RABBITMQ_URL`, `AUTH_JWKS_URL`, `ROOMS_LOBBY_URL`, `LIVEOPS_URL`, `BOT_AI_URL`, `ALLOWED_ORIGINS`, `STORAGE_MODE` (`postgres` | `memory`).

## Pruebas

```bash
go test ./...
```

Incluye la mano canónica (Full House K-K-K-4-4 = **1248**) con el ruleset por defecto, variantes del ruleset (Banner 45, The Duo desactivado, Happy Hour ×2/×3), ciclo de eventos de una partida, bot remoto con respaldo, validación del JWT y una partida completa por WebSocket.

## Patrones

Actor (goroutine por sala) · Publisher (eventos) · Strategy + Hot reload (ruleset) · Fallback (bot local, ruleset por defecto) · Token-based auth (JWKS).
