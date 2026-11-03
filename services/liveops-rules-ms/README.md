# liveops-rules-ms

Balance del juego en caliente. **Responsable:** Jaisson Andrés Machado Bautista · **TypeScript / Node 22 (Fastify)** · puerto interno **8087**.

Guarda el ruleset de los Jokers como **versiones append-only** en PostgreSQL, lo cachea en Redis y publica cada cambio (`liveops.ruleset.updated`). game-engine-ms lo recarga sin reiniciarse: las salas nuevas lo usan de inmediato y las activas desde su siguiente mano. Contrato: `contracts/ruleset.md`.

## Endpoints (por el gateway con prefijo `/admin`, rol admin)

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/liveops/ruleset` | Vigente (Redis → PostgreSQL). |
| PUT | `/liveops/ruleset` | `{expected_version, jokers, happy_hour}`. `400` fuera de rango, `409` si otro guardó primero. |
| POST | `/liveops/jokers/{banner\|classic\|duo}/toggle` | Activa/desactiva un Joker. |
| POST | `/liveops/happy-hour` | `{enabled, global_mult}`. |
| GET | `/liveops/ruleset/history` | Últimas versiones (quién y cuándo). |
| GET | `/health` | PostgreSQL, Redis, RabbitMQ. |

## Demo

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' -d '{"alias":"admin","password":"<ADMIN_PASSWORD>"}' | jq -r .token)
```

```bash
curl -s -X POST localhost:8080/admin/liveops/happy-hour -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"enabled":true,"global_mult":2}'
```

```bash
curl -s localhost:8080/api/game/ruleset
```

## Pruebas

```bash
npm install && npm test
```
