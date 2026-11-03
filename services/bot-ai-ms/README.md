# bot-ai-ms

Decisión del oponente bot. **Responsable:** Juan David Suárez Coronado · **Rust (axum)** · puerto interno **8084** · sin estado.

Port fiel del evaluador y la puntuación del motor en Go (la mano canónica da **1248** también aquí), más tres dificultades. Usa el ruleset de LiveOps que llega en cada petición, así que el bot "entiende" los cambios de balance.

## Endpoint

`POST /bot/decide`

```json
{ "hand": [{"rank":"K","suit":"SPADES"}], "hands_left": 10, "discards_left": 2,
  "target_score": 22000, "current_score": 0, "difficulty": "normal", "ruleset": { } }
```

→ `{"action": "PLAY"|"DISCARD", "card_indices": [0,1,2], "difficulty": "normal", "expected_hand": "FULL_HOUSE", "expected_score": 1248}`

| Dificultad | Estrategia |
|---|---|
| `easy` | Siempre juega su mejor mano. |
| `normal` | Igual al bot local del motor: descarta si su mejor mano es más débil que un trío. |
| `hard` | Descarta si su mejor mano es más débil que un color (salvo en su última mano). |

Entradas inválidas → `400` con `{"error","message"}`. `GET /health`.
Solo se llama dentro de la red (no se expone por el gateway).

## Pruebas

```bash
cargo test
```
