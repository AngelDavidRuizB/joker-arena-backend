# Contrato de eventos — RabbitMQ

- **Exchange:** `jokerarena.events`, tipo `topic`, durable. Cada productor lo declara al arrancar (idempotente).
- **Mensajes:** JSON UTF-8, `content_type: application/json`, `delivery_mode: 2` (persistentes).
- **Colas:** una por consumidor y propósito, durable, nombre `<servicio>.<propósito>`.
- **Entrega:** al menos una vez. **Todos los consumidores deben ser idempotentes** (usar `event_id` o la clave natural del evento).

## Sobre común

```json
{
  "event_id": "uuid v4",
  "type": "game.hand.scored",
  "version": 1,
  "occurred_at": "2026-11-03T18:00:00Z",
  "producer": "game-engine-ms",
  "payload": { }
}
```

`type` es igual a la routing key. Un cambio incompatible en un payload sube `version`.

## Eventos

| Routing key | Productor | Consumidores (cola) |
|---|---|---|
| `auth.player.registered` | auth-identity-ms | leaderboard-stats-ms (`leaderboard-stats-ms.players`) |
| `lobby.room.created` | rooms-lobby-ms | telemetry-ms (`telemetry-ms.events`) |
| `game.match.started` | game-engine-ms | telemetry-ms (`telemetry-ms.events`), rooms-lobby-ms (`rooms-lobby-ms.game`) |
| `game.hand.scored` | game-engine-ms | telemetry-ms (`telemetry-ms.events`) |
| `game.match.ended` | game-engine-ms | telemetry-ms (`telemetry-ms.events`), leaderboard-stats-ms (`leaderboard-stats-ms.matches`), rooms-lobby-ms (`rooms-lobby-ms.game`) |
| `liveops.ruleset.updated` | liveops-rules-ms | game-engine-ms (`game-engine-ms.ruleset`) |

### `auth.player.registered`

```json
{ "player_id": "uuid", "alias": "Ana" }
```

### `lobby.room.created`

```json
{ "room_code": "JKR-AB12", "mode": "bot", "difficulty": "normal", "target_score": 22000, "created_by": "uuid" }
```

### `game.match.started`

```json
{ "match_id": "uuid", "room_code": "JKR-AB12", "player_ids": ["uuid", "bot"], "with_bot": true, "target_score": 22000, "ruleset_version": 1 }
```

### `game.hand.scored`

```json
{
  "match_id": "uuid", "room_code": "JKR-AB12", "player_id": "uuid", "is_bot": false,
  "hand_number": 3, "hand_type": "PAIR",
  "cards": [{ "rank": "K", "suit": "hearts", "chips": 10, "scored": true }],
  "base_chips": 10, "base_mult": 2, "card_chips": 20, "discards_left": 2,
  "chips": 90, "mult": 12, "score": 1080, "accumulated_score": 2328,
  "jokers": [{ "id": "banner", "name": "Banner", "effect": "...", "triggered": true, "chips_added": 60, "mult_added": 0, "mult_factor": 1 }],
  "ruleset_version": 1
}
```

### `game.match.ended`

```json
{
  "match_id": "uuid", "room_code": "JKR-AB12", "is_bot_match": true,
  "reason": "TARGET_SCORE_REACHED", "target_score": 22000, "hands_played": 12,
  "results": [
    { "player_id": "uuid", "alias": "Ana", "score": 23100, "won": true, "is_bot": false },
    { "player_id": "bot", "alias": "Bot", "score": 18000, "won": false, "is_bot": true }
  ]
}
```

El bot siempre usa `player_id: "bot"` y `is_bot: true`.

### `liveops.ruleset.updated`

Payload: el ruleset completo (ver `ruleset.md`).
