# Contrato de rutas del API Gateway

Único punto de entrada: **`http://localhost:8080`** (contenedor `jokerarena-gateway`, nginx).
Configuración: `gateway/nginx.conf`.

- **Autenticación:** las rutas marcadas con JWT hacen una subpetición a `auth-identity-ms` (`GET /auth/verify`) antes de enrutar. Sin token → `401`; rol insuficiente → `403`.
- **Token:** `Authorization: Bearer <jwt>`. En WebSockets los navegadores no envían cabeceras, así que el token va en `?token=<jwt>`.
- **Errores del gateway:** misma forma que los servicios: `{"error": "<codigo>", "message": "<texto>"}` (`401 unauthorized`, `403 forbidden`, `503 service_unavailable`, `404 not_found`).
- **CORS:** orígenes `http://localhost:*` y `tauri://localhost`.
- **Límite:** `/api/auth/*` admite 10 peticiones por segundo por IP (ráfaga de 20).

| Ruta pública | Método | Servicio → ruta interna | Auth |
|---|---|---|---|
| `/health` | GET | gateway | pública |
| `/api/auth/register` | POST | auth-identity-ms `/auth/register` | pública |
| `/api/auth/login` | POST | auth-identity-ms `/auth/login` | pública |
| `/api/auth/me` | GET | auth-identity-ms `/auth/me` | JWT (lo valida auth) |
| `/api/auth/logout` | POST | auth-identity-ms `/auth/logout` | JWT (lo valida auth) |
| `/api/rooms` | GET, POST | rooms-lobby-ms `/rooms` | JWT |
| `/api/rooms/{code}` | GET | rooms-lobby-ms `/rooms/{code}` | JWT |
| `/api/rooms/{code}/join` | POST | rooms-lobby-ms `/rooms/{code}/join` | JWT |
| `/ws/{code}?token=<jwt>` | WebSocket | game-engine-ms `/ws/{code}` | JWT (lo valida el motor con el JWKS) |
| `/api/game/ruleset` | GET | game-engine-ms `/ruleset` | pública |
| `/api/telemetry/matches` | GET | telemetry-ms `/matches` | JWT |
| `/api/telemetry/matches/{match_id}` | GET | telemetry-ms `/matches/{match_id}` | JWT |
| `/api/telemetry/hands` | GET | telemetry-ms `/hands` | JWT |
| `/api/telemetry/stats` | GET | telemetry-ms `/stats` | JWT |
| `/api/leaderboard/top` | GET | leaderboard-stats-ms `/leaderboard/top` | pública |
| `/api/leaderboard/players/{player_id}` | GET | leaderboard-stats-ms `/leaderboard/players/{id}` | pública |
| `/admin/liveops/ruleset` | GET, PUT | liveops-rules-ms `/liveops/ruleset` | JWT rol `admin` |
| `/admin/liveops/ruleset/history` | GET | liveops-rules-ms `/liveops/ruleset/history` | JWT rol `admin` |
| `/admin/liveops/jokers/{id}/toggle` | POST | liveops-rules-ms | JWT rol `admin` |
| `/admin/liveops/happy-hour` | POST | liveops-rules-ms | JWT rol `admin` |
| `/admin/cluster/snapshot` | GET | cluster-monitor-ms | JWT rol `admin` |
| `/admin/cluster/metrics?token=<jwt>` | WebSocket | cluster-monitor-ms | JWT rol `admin` |

`bot-ai-ms` no se publica: solo lo llama `game-engine-ms` dentro de la red.

## Llamadas internas (dentro de `jokerarena-net`)

| Origen → destino | Petición |
|---|---|
| game-engine-ms → rooms-lobby-ms | `GET /rooms/{code}` (validar la sala al abrir el WebSocket) |
| game-engine-ms → bot-ai-ms | `POST /bot/decide` (turno del bot; respaldo local si falla o tarda > 800 ms) |
| game-engine-ms → liveops-rules-ms | `GET /liveops/ruleset` (ruleset vigente al arrancar) |
| game-engine-ms → auth-identity-ms | `GET /.well-known/jwks.json` (llaves para validar el JWT) |
| gateway → auth-identity-ms | `GET /auth/verify` (validación de rutas protegidas) |
| web-ssr (servidor) → gateway | las mismas rutas públicas, con el JWT de la cookie `ja_token` |
