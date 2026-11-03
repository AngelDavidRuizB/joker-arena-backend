# leaderboard-stats-ms

Servicio de **ranking y estadísticas** de JokerArena (Prototipo 2).
**Responsable:** Nicole Ariadna Celemin Triana · **Lenguaje:** TypeScript (Node 22, Fastify) · **Puerto interno:** 8086

Escucha los eventos del juego en RabbitMQ, los guarda como un **grafo** en Neo4j (jugadores, partidas y quién se enfrentó con quién) y expone el ranking por HTTP. El top se cachea en Redis.

```
auth-identity-ms ──auth.player.registered──┐
                                           ├──► RabbitMQ (jokerarena.events) ──► leaderboard-stats-ms ──► Neo4j
game-engine-ms ────game.match.ended────────┘                                          │      ▲
                                                                                      ▼      │
                         gateway  GET /api/leaderboard/*  ───────────────────────►  HTTP   Redis (caché del top)
```

## Endpoints

Todos devuelven JSON en `snake_case`. Errores con la forma `{"error": "<codigo>", "message": "<texto>"}`.
A través del gateway se publican como `/api/leaderboard/top` y `/api/leaderboard/players/{player_id}` (rutas públicas, sin JWT).

| Método | Ruta | Respuesta |
|---|---|---|
| GET | `/leaderboard/top?limit=10` | Top de jugadores (sin el bot). `limit` entre 1 y 100 (por defecto 10); fuera de rango → `400 invalid_limit`. |
| GET | `/leaderboard/players/{player_id}` | Estadísticas del jugador y sus 5 rivales más frecuentes. `404 player_not_found` si no existe. |
| GET | `/health` | `{"status":"ok"\|"degraded","service":"leaderboard-stats-ms","deps":{"neo4j","redis","rabbitmq"}}`. Siempre 200: el servicio sigue arriba aunque una dependencia caiga. |

Si Neo4j no responde, las rutas de datos devuelven `503 store_unavailable`.

**Ejemplo `GET /leaderboard/top?limit=2`:**

```json
{
  "generated_at": "2026-11-03T18:00:00.000Z",
  "cached": false,
  "players": [
    { "rank": 1, "player_id": "6f1c…", "alias": "Ana", "matches": 3, "wins": 2, "best_score": 1248, "total_score": 2900 },
    { "rank": 2, "player_id": "a93b…", "alias": "Beto", "matches": 2, "wins": 1, "best_score": 2000, "total_score": 2800 }
  ]
}
```

Orden del ranking: más victorias → mejor puntaje → menos partidas jugadas.

**Ejemplo `GET /leaderboard/players/{id}`:**

```json
{
  "player_id": "6f1c…", "alias": "Ana",
  "matches": 3, "wins": 2, "losses": 1, "win_rate": 0.667,
  "best_score": 1248, "total_score": 2900,
  "rivals": [ { "player_id": "bot", "alias": null, "is_bot": true, "matches": 2 } ]
}
```

## Eventos que consume

Exchange `jokerarena.events` (topic, durable). Contrato completo en `contracts/events.md`.

| Routing key | Cola | Qué hace |
|---|---|---|
| `auth.player.registered` | `leaderboard-stats-ms.players` | Crea o actualiza `(:Player {player_id, alias})`. |
| `game.match.ended` | `leaderboard-stats-ms.matches` | Crea `(:Match)`, una relación `PLAYED {score, won}` por jugador y `FACED` entre cada par de jugadores. |

- **Idempotente:** si llega dos veces la misma partida (`match_id`), la segunda se ignora.
- **El bot no entra al ranking:** se reconoce por `is_bot: true` en `results[]` o por `player_id: "bot"`.
- **Mensajes inválidos** (JSON roto o fuera de contrato) se descartan y se registran en el log; no bloquean la cola.
- **Fallos transitorios** (Neo4j caído) se reintentan: el mensaje vuelve a la cola y se procesa cuando Neo4j vuelve.
- Si RabbitMQ se reinicia, el consumidor se reconecta solo.

## Datos

- **Neo4j** (dueño): modelo y restricciones en `db/neo4j/constraints.cypher`. El servicio crea las restricciones al arrancar.
- **Redis:** clave `leaderboard:top` (top 100 ya calculado, TTL 30 s por defecto). Se borra cada vez que llega una partida o un jugador nuevo. Si Redis falla, el servicio responde igual desde Neo4j.

## Variables de entorno

| Variable | Por defecto | Descripción |
|---|---|---|
| `PORT` | `8086` | Puerto HTTP. |
| `NEO4J_URL` | `bolt://jokerarena-neo4j:7687` | Conexión a Neo4j. |
| `NEO4J_USER` | `neo4j` | Usuario de Neo4j. |
| `NEO4J_PASSWORD` | — (obligatoria) | Contraseña de Neo4j. |
| `REDIS_URL` | `redis://jokerarena-redis:6379/0` | Conexión a Redis. |
| `RABBITMQ_URL` | — (obligatoria) | `amqp://<usuario>:<clave>@jokerarena-rabbitmq:5672/` |
| `RABBITMQ_EXCHANGE` | `jokerarena.events` | Exchange de eventos. |
| `LEADERBOARD_CACHE_TTL_SECONDS` | `30` | Vida de la caché del top. |
| `LOG_LEVEL` | `info` | Nivel de logs (pino). |

## Cómo correrlo

Con todo el sistema (desde la raíz del repo):

```bash
docker compose -f docker-compose.p2.yml up -d --build jokerarena-leaderboard-stats-ms
```

Eso levanta también Neo4j, Redis y RabbitMQ (`depends_on`). El servicio no publica puertos al host: se consume a través del gateway (`http://localhost:8080/api/leaderboard/top`).

Para ver el grafo en el navegador de Neo4j (`http://localhost:7474`):

```cypher
MATCH (p:Player)-[r:PLAYED]->(m:Match) RETURN p, r, m LIMIT 50
```

## Pruebas

```bash
npm install
```

```bash
npm test
```

45 pruebas unitarias: validación del contrato de eventos, lógica del ranking, caché (aciertos, invalidación, Redis caído), idempotencia, manejo de mensajes del broker, API HTTP y consultas Cypher (con un doble del driver).

**Prueba de integración** (contra Neo4j, Redis y RabbitMQ reales, con el sistema levantado). Publica eventos en el exchange como lo harían auth-identity y game-engine y verifica el resultado por HTTP:

```bash
LEADERBOARD_URL=http://<ip-del-contenedor>:8086 RABBITMQ_URL=amqp://<usuario>:<clave>@localhost:5672/ npm run test:integration
```

## Patrones aplicados

- **Event-driven consumer / Publish-subscribe:** no llama a ningún otro servicio; se entera de todo por eventos.
- **Materialized view:** el grafo es una proyección de lectura construida a partir de eventos.
- **Idempotent receiver:** procesar el mismo evento dos veces no cambia el resultado.
- **Cache-aside:** el top se lee de Redis y, si no está, de Neo4j (y se guarda en Redis).
- **Health check y degradación controlada:** `/health` reporta cada dependencia y el servicio no se cae si una falla.
