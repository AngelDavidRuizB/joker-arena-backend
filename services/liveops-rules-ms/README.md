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

## Seguridad (P3 · escenario 5, Zero Trust)

El servicio no confía en la red interna: todo `PUT`/`POST` exige un JWT admin que **liveops verifica por su cuenta** (firma RS256 con el JWKS de `auth-identity-ms`, `iss`, `exp`, sesión no cerrada y `role = admin`). `401` sin token o inválido, `403` sin rol admin, `503` si auth no responde (fail closed). Los `GET` siguen abiertos (el motor lee el ruleset sin token). Las llaves se piden a `http://jokerarena-auth-identity-ms:8082/.well-known/jwks.json` con reintentos, sin depender del orden de arranque. Código: `src/auth.ts`.

```bash
# Desde otro contenedor de la red interna, sin token → 401
docker run --rm --network app-net curlimages/curl -s -o /dev/null -w "%{http_code}\n" -X PUT http://jokerarena-liveops-rules-ms:8087/liveops/ruleset
```

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
