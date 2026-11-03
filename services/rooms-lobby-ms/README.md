# rooms-lobby-ms

Lobby de salas. **Responsable:** Jose Arturo Herrera Rivera · **C# / .NET 8 (Minimal API)** · puerto interno **8083**.

Crea salas con código `JKR-XXXX` (sin caracteres ambiguos 0/O, 1/I/L), las lista y lleva su estado (`open` → `in_progress` → `closed`) a partir de los eventos del motor.

## Endpoints

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/rooms` | `{"mode": "bot"\|"pvp", "difficulty": "easy"\|"normal"\|"hard", "target_score"}` → `201`. Publica `lobby.room.created`. |
| GET | `/rooms?status=open\|in_progress\|closed\|all&limit=` | Lista de salas. |
| GET | `/rooms/{code}` | Detalle (lo usa game-engine-ms al abrir el WebSocket). |
| POST | `/rooms/{code}/join` | Unirse a una sala `pvp` abierta (máx. 4). |
| GET | `/health` | Estado de PostgreSQL y RabbitMQ. |

La identidad se toma del JWT que el gateway ya validó y reenvía en `Authorization`.

## Eventos

Publica `lobby.room.created`. Consume `game.match.started` y `game.match.ended` (cola `rooms-lobby-ms.game`): la sala solo avanza de estado, nunca retrocede.

## Datos

PostgreSQL, esquema `lobby`: `lobby.rooms` y `lobby.room_players` — `db/postgres/init/20_lobby.sql`.

## Pruebas

```bash
dotnet test tests/RoomsLobby.Tests
```

Sin .NET instalado:

```bash
docker run --rm -v "$PWD":/src -w /src mcr.microsoft.com/dotnet/sdk:8.0 dotnet test tests/RoomsLobby.Tests
```
