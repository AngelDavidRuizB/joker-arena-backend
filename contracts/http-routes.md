# Contrato de rutas del API Gateway

Único punto de entrada (P3): **`https://localhost`** (contenedor `jokerarena-reverse-proxy`, nginx).
Es el único contenedor con puertos publicados en el host:

- `443`: HTTPS y WSS (TLS 1.2/1.3, HSTS). Certificados locales: `bash scripts/gen_certs.sh`
  (CA en `edge/certs/jokerarena-ca.crt`, fuera de git).
- `80`: solo redirige con `301` a `https://`.
- `/api/*`, `/ws/*`, `/admin/*` (salvo la página `/admin` de la web) y `/health` van al API
  Gateway por `edge-net`; el resto (`/`) va al cliente web (`jokerarena-web-ssr:3000`, repo
  `joker-arena-frontend`).
- El proxy no revela la versión de nginx (`server_tokens off`), agrega cabeceras de seguridad y
  corta las peticiones demasiado grandes (`413`, cuerpo > 64 KB) o lentas (`408`).
- Envía `X-Forwarded-For`, `X-Real-IP` y `X-Forwarded-Proto`; el gateway recupera la IP real
  (`real_ip_header`), así que el límite de `/api/auth/*` sigue siendo por cliente.

El API Gateway (`jokerarena-gateway`, `gateway/nginx.conf`) ya no publica el `8080` en el host:
solo se llega a él a través del reverse proxy (para depurar: `docker-compose.debug.yml`).
Las rutas de las tablas son las mismas que en el P2; solo cambia el origen
(`http://localhost:8080` → `https://localhost`).
Configuración del proxy: `edge/reverse-proxy/` (`nginx.conf`, `conf.d/10-tls.conf`, `locations.conf`).

- **Autenticación:** las rutas marcadas con JWT hacen una subpetición a `auth-identity-ms` (`GET /auth/verify`) antes de enrutar. Sin token → `401`; rol insuficiente → `403`.
- **Token:** `Authorization: Bearer <jwt>`. En WebSockets los navegadores no envían cabeceras, así que el token va en `?token=<jwt>`.
- **Errores del gateway:** misma forma que los servicios: `{"error": "<codigo>", "message": "<texto>"}` (`401 unauthorized`, `403 forbidden`, `503 service_unavailable`, `404 not_found`).
- **CORS:** orígenes `http(s)://localhost:*` y `tauri://localhost`.
- **Límite:** `/api/auth/*` admite 10 peticiones por segundo por IP (ráfaga de 20).

### Cuerpos de autenticación (`/api/auth/*`)

```json
POST /api/auth/register   {"user": "JokerPro99", "password": "secreta123", "repeat_password": "secreta123"}
POST /api/auth/login      {"user": "JokerPro99", "password": "secreta123"}
```

- `user`: nombre de usuario, 2–30 caracteres (`letras`, números, `_`, `-`). `alias` se acepta como sinónimo.
- `password`: mínimo 8 y máximo 72 caracteres. Se guarda como hash `pbkdf2_sha256` en `auth.players.password_hash`; nunca se devuelve ni se registra.
- `repeat_password` (solo registro): si llega, debe coincidir con `password` (`400 password_mismatch`).
- Registro: `201 {token, player}` · `409 alias_taken` (ya existe con contraseña) · `400 invalid_password` · `503 store_unavailable`.
- Login: `200 {token, player}` · `401 invalid_credentials` (usuario o contraseña incorrectos) · `404 player_not_found` · `401 password_not_set` (cuenta anterior sin contraseña: vuelve a registrarte con el mismo usuario para crearla).
- El usuario reservado `admin` exige además `ADMIN_PASSWORD` (registro y login).
- Bloqueo: tras `AUTH_MAX_FAILED_ATTEMPTS` (5) fallos de credenciales en login **o** registro, la cuenta queda bloqueada `AUTH_LOCKOUT_MINUTES` (15) minutos → `423 account_locked` + cabecera `Retry-After` (segundos) en ambos endpoints. Un login correcto reinicia el contador.
- Vigencia del token: **12 h** para `player` y **30 min** para `admin` (`AUTH_ADMIN_TOKEN_TTL_SECONDS`); `expires_in` y el claim `exp` reflejan el valor aplicado.

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

## Llamadas internas (dentro de `app-net`)

| Origen → destino | Petición |
|---|---|
| game-engine-ms → rooms-lobby-ms | `GET /rooms/{code}` (validar la sala al abrir el WebSocket) |
| game-engine-ms → bot-ai-ms | `POST /bot/decide` (turno del bot; respaldo local si falla o tarda > 800 ms) |
| game-engine-ms → liveops-rules-ms | `GET /liveops/ruleset` (ruleset vigente al arrancar) |
| game-engine-ms → auth-identity-ms | `GET /.well-known/jwks.json` (llaves para validar el JWT) |
| gateway → auth-identity-ms | `GET /auth/verify` (validación de rutas protegidas) |
| cliente web SSR (repo `joker-arena-frontend`) → gateway | las mismas rutas públicas, con el JWT de la cookie `ja_token` |
