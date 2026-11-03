# telemetry-ms

Auditoría y estadísticas. **Responsable:** José Alejandro Solano Garcés · **Python 3.12 / FastAPI + Motor + aio-pika** · puerto interno **8085**.

Consume los eventos del juego desde RabbitMQ (cola `telemetry-ms.events`, bindings `game.#` y `lobby.room.created`) y los proyecta en MongoDB. Es idempotente: cada `event_id` se guarda una sola vez.

## Endpoints

| Ruta | Descripción |
|---|---|
| `GET /matches?limit=` | Últimas partidas (resumen y resultado). |
| `GET /matches/{match_id}` | Resumen y auditoría mano a mano (`404` si no hay datos). |
| `GET /hands?match_id=&player_id=&limit=` | Manos registradas. |
| `GET /stats` | Totales y distribución de tipos de mano. |
| `GET /health` | Estado de MongoDB y del consumidor. |

Por el gateway: `/api/telemetry/*` (JWT).

## Datos (MongoDB, base `telemetry`)

`events` (copia cruda, `event_id` único) · `hand_events` (única por `match_id, player_id, hand_number`) · `match_summaries`. Índices en `db/mongo/init.js`.

## Pruebas

```bash
pip install -r requirements-dev.txt && pytest -q
```

El servicio gRPC del Prototipo 1 no se usa en el Prototipo 2 (la ingesta ahora es asíncrona por RabbitMQ), por eso no se incluyó.
