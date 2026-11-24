# JokerArena: Multiverse Poker Engine 🃏

Videojuego web distribuido de póker con *Jokers* en tiempo real, inspirado en la mecánica de puntuación de *Balatro*. Arquitectura de Software 2026-II (Universidad Nacional de Colombia).

**Prototipo 3 (vigente):** 8 servicios de lógica en 5 lenguajes, API Gateway (nginx), **reverse proxy con TLS como única entrada**, 7 redes segmentadas, RabbitMQ y persistencia políglota (PostgreSQL + MongoDB + Redis + Neo4j). El **cliente web con SSR** (Next.js) vive en su propio repositorio: [joker-arena-frontend](https://github.com/AngelDavidRuizB/joker-arena-frontend).

> Documento de entrega: [`docs/p3/p3_2b.md`](docs/p3/p3_2b.md) · Escenarios y evidencias: [`docs/p3/secciones/`](docs/p3/secciones) · Contratos: [`contracts/`](contracts/)

---

## ⚡ Arranque rápido (proyecto completo)

Requisitos: **Docker Engine 24+ / Docker Desktop** con Compose v2, **bash** (Git Bash, WSL o Linux/macOS) y **openssl**. Opcional para desarrollo sin Docker: Node.js 20+.

```bash
# 1) Configuración (los valores por defecto ya funcionan; cámbialos si quieres)
cp .env.example .env

# 2) Certificados TLS del borde (se generan en edge/certs/, no están en git)
bash scripts/gen_certs.sh

# 3) Backend: 15 contenedores (8 servicios + gateway + reverse proxy + 4 datos + broker)
docker compose -f docker-compose.p2.yml up -d --build --wait

# 4) Cliente web (otro repositorio, se une a la red edge-net; no publica puertos)
cd ../joker-arena-frontend && docker compose up -d --build --wait

# 5) Comprobar
cd ../joker-arena-backend && bash scripts/verify.sh
```

| Qué | URL |
|---|---|
| **Aplicación web** (por el reverse proxy) | **https://localhost** |
| API / WebSocket (mismo origen, a través del proxy) | `https://localhost/api/*` · `wss://localhost/ws/{code}` |
| Redirección del puerto 80 | `http://localhost` → `301 https://localhost` |
| Administración (consola LiveOps + radar) | https://localhost/admin con el usuario `admin` y `ADMIN_PASSWORD` |

> El certificado lo firma una **CA local** generada por `gen_certs.sh`. El navegador avisará hasta que importes [`edge/certs/jokerarena-ca.crt`](edge/certs) como autoridad de confianza; con `curl` basta `--cacert edge/certs/jokerarena-ca.crt`.

---

## 🚀 Puesta en marcha detallada

### 1. Configuración (`.env`)

```bash
cp .env.example .env
```

Los valores del ejemplo ya permiten levantar el sistema; antes de publicarlo cambia las contraseñas. `.env` está en `.gitignore` y **nunca** se versiona.

### 2. Certificados del borde (Secure Channel, escenario 1)

```bash
bash scripts/gen_certs.sh            # CA local + certificado del reverse proxy en edge/certs/
bash scripts/gen_certs.sh --force    # regenerar
```

`edge/certs/` queda fuera del repositorio: **cada máquina debe generarlos** antes del primer `up`. Sin ellos, `jokerarena-reverse-proxy` no arranca (`nginx: [emerg] cannot load certificate .../server.crt`).

### 3. Backend (microservicios + gateway + reverse proxy)

```bash
docker compose -f docker-compose.p2.yml up -d --build --wait
docker compose -f docker-compose.p2.yml ps      # 15 contenedores "healthy"
```

- Solo el **reverse proxy** publica puertos (80 y 443). El gateway (`8080`), los servicios, el broker y las bases de datos no se exponen al host.
- Si algún contenedor no llega a `healthy`, `--wait` termina con error y el propio comando indica cuál: revisa `docker logs <contenedor>`.

### 4. Cliente web (repositorio aparte, red `edge-net`)

```bash
cd ../joker-arena-frontend
docker compose up -d --build --wait
```

El `compose.yml` del frontend se une a la red externa **`edge-net`** (la crea el backend) y **no publica puertos**: el reverse proxy lo alcanza como `jokerarena-web-ssr:3000`. Si el backend no está levantado, crea la red a mano: `docker network create edge-net`.

### 5. Acceso

1. Abre **https://localhost** (acepta/instala la CA local si el navegador avisa).
2. Regístrate con **usuario y contraseña** (mínimo 8 caracteres) o inicia sesión.
3. Para la consola de administración usa el usuario **`admin`** y la contraseña `ADMIN_PASSWORD` del `.env`.

### 6. Verificación

```bash
bash scripts/verify.sh          # P3 completo: TLS, proxy, rutas, WSS, segmentación, bloqueo, zero trust
bash scripts/verify.sh --up     # lo levanta primero (build + wait)
bash scripts/verify_p2.sh       # subconjunto: contenedores, rutas del gateway, bloqueo y broker
```

`verify.sh` comprueba, entre otras cosas: `301` del puerto 80, certificado válido para la CA local, HSTS y cabeceras de seguridad, que el `8080` **no** esté expuesto, las rutas a través del proxy (`/health`, `/api/auth/*`, `/api/rooms`, `/api/telemetry/*`, `/api/leaderboard/*`, `/admin/*`), el WebSocket `wss://localhost/ws/{code}` → `101`, el aislamiento entre redes, el bloqueo de cuenta (`423`) y que LiveOps rechace cambios sin token desde la red interna.

### Modo desarrollo / depuración

```bash
# Publica en localhost el gateway (8080), PostgreSQL (5432), MongoDB (27017), Redis (6379),
# RabbitMQ (15672) y Neo4j (7474/7687) para depurar. La web sigue en su propio repo.
docker compose -f docker-compose.p2.yml -f docker-compose.debug.yml up -d

# Cliente web en modo desarrollo (SSR con recarga) contra el gateway publicado
cd ../joker-arena-frontend
GATEWAY_INTERNAL_URL=http://localhost:8080 NEXT_PUBLIC_GATEWAY_WS_URL=ws://localhost:8080 npm run dev
# PowerShell: $env:GATEWAY_INTERNAL_URL='http://localhost:8080'; $env:NEXT_PUBLIC_GATEWAY_WS_URL='ws://localhost:8080'; npm run dev
# → http://localhost:3000 (las páginas SSR usan el gateway y el WebSocket va directo a :8080)
```

### Redes (segmentación, escenario 3)

| Red | Contiene | `internal: true` |
|---|---|---|
| **`edge-net`** | reverse proxy, API Gateway, cliente web SSR | no (borde) |
| `app-net` | comunicación entre microservicios y el gateway | sí |
| `broker-net` | RabbitMQ y sus clientes | sí |
| `pg-net` · `redis-net` · `mongo-net` · `neo4j-net` | cada base de datos, solo con los servicios que la usan | sí |

Regla práctica: el navegador solo habla con el reverse proxy; el resto del tráfico ocurre entre redes internas.

---

## 🧩 Servicios

| Servicio | Lenguaje | Carpeta | Responsabilidad |
|---|---|---|---|
| `auth-identity-ms` | Python 3.12 / FastAPI | `services/auth-identity-ms` | registro e inicio de sesión con usuario y contraseña (hash PBKDF2), JWT RS256, bloqueo por intentos fallidos, JWKS |
| `rooms-lobby-ms` | C# / .NET 8 | `services/rooms-lobby-ms` | salas del lobby (`JKR-XXXX`, modo bot/solitario, dificultad) |
| `game-engine-ms` | Go | `services/game-engine-ms` | motor autoritativo del juego (actor por sala), baraja, evaluador, puntuación, WebSocket |
| `bot-ai-ms` | Rust | `services/bot-ai-ms` | decisión del bot según dificultad |
| `telemetry-ms` | Python 3.12 / FastAPI | `services/telemetry-ms` | auditoría mano a mano en MongoDB (consumidor de eventos + REST) |
| `leaderboard-stats-ms` | TypeScript | `services/leaderboard-stats-ms` | ranking global (Neo4j + caché Redis) |
| `liveops-rules-ms` | TypeScript | `services/liveops-rules-ms` | ruleset versionado (balance de Jokers) y validación |
| `cluster-monitor-ms` | Go | `services/cluster-monitor-ms` | observabilidad: sondas del clúster, alarmas, radar por WebSocket |
| **API Gateway** | nginx | `gateway/` | enrutado, validación del JWT (`auth_request`), rate limit de `/api/auth/*` |
| **Reverse Proxy** | nginx + TLS | `edge/reverse-proxy/` | única entrada (80 → `301`, 443 TLS/WSS), oculta la topología, límites de tamaño/tiempo |
| **CLI** | Python | `clients/cli` | cliente de terminal (`jokerarena login …`, salas, ranking, auditoría) |

## 🔐 Seguridad en el P3 (escenarios y contramedidas)

| Escenario | Contramedida | Dónde |
|---|---|---|
| 1. Secure Channel | TLS 1.2/1.3 en el reverse proxy, HSTS, 80 → 301, WSS | `edge/reverse-proxy/conf.d/10-tls.conf` · [`escenario1_secure_channel.md`](docs/p3/secciones/escenario1_secure_channel.md) |
| 2. Reverse Proxy | única puerta de entrada; gateway y web sin puertos publicados; cabeceras saneadas | `edge/reverse-proxy/` · [`escenario2_reverse_proxy.md`](docs/p3/secciones/escenario2_reverse_proxy.md) |
| 3. Network Segmentation | 7 redes, solo `edge-net` no es interna | `docker-compose.p2.yml` · [`escenario3_segmentacion.md`](docs/p3/secciones/escenario3_segmentacion.md) |
| 4. Account Lockout + Rate Limiting | 5 fallos → bloqueo de 15 min (`423` + `Retry-After`), rate limit por IP, token de admin de 30 min | `services/auth-identity-ms` · [`escenario4_lockout.md`](docs/p3/secciones/escenario4_lockout.md) |
| 5. Zero Trust | el servicio valida el JWT aunque la llamada venga de la red interna | `services/liveops-rules-ms` · [`escenario5_zero_trust.md`](docs/p3/secciones/escenario5_zero_trust.md) |

## ⚙️ Variables de entorno (`.env`)

| Variable | Descripción |
|---|---|
| `POSTGRES_PASSWORD`, `MONGO_ROOT_*`, `NEO4J_PASSWORD`, `RABBITMQ_*` | credenciales de las bases de datos y del broker |
| `AUTH_DB_PASSWORD`, `LOBBY_DB_PASSWORD`, `GAME_DB_PASSWORD`, `LIVEOPS_DB_PASSWORD`, `MONITOR_DB_PASSWORD` | un usuario de PostgreSQL por servicio (esquema propio) |
| `ADMIN_PASSWORD` | contraseña del usuario `admin` (rol `admin`, consola `/admin`) |
| `AUTH_MAX_FAILED_ATTEMPTS` | intentos fallidos antes de bloquear la cuenta (5) |
| `AUTH_LOCKOUT_MINUTES` | **duración del bloqueo en minutos** (15) ← aquí se cambia el tiempo |
| `AUTH_ADMIN_TOKEN_TTL_SECONDS` | vigencia del token de `admin` (1800 = 30 min); los jugadores tienen 12 h |
| `DOCKER_GID` | grupo dueño de `/var/run/docker.sock` para `cluster-monitor-ms` (Linux: `stat -c %g`; Docker Desktop: `0`) |
| `ROOM_IDLE_TIMEOUT` | tiempo sin clientes para cerrar una partida abandonada (`15m`) |
| `GATEWAY_PORT` | puerto del gateway en el host **solo** con `docker-compose.debug.yml` (8080) |
| `PUBLIC_GATEWAY_WS_URL` | (opcional) URL pública del WebSocket si se despliega fuera de `localhost` |

La configuración del **cliente web** (puerto, `GATEWAY_INTERNAL_URL`, etc.) vive en `../joker-arena-frontend/.env` — ver su README.

## 🎮 Cómo se juega

1. Regístrate con **usuario y contraseña** (mínimo 8 caracteres) o inicia sesión. Tras **5 intentos fallidos** la cuenta se bloquea 15 minutos (configurable).
2. Crea una sala (meta 22 000 / 26 000 / 33 000, con o sin Bot) o únete con un código. Quien no es dueño de la sala entra como espectador.
3. Selecciona hasta 5 cartas (clic o teclas `1–8`) y **Jugar mano** (`P`) o **Descartar** (`D`). Tienes 20 manos y 3 descartes; la baraja se recicla.
4. Puntaje = `(fichas de la mano + fichas de las cartas que puntúan + 30 × descartes restantes) × ((mult de la mano + 4) × 2 si hay pareja)`. Ejemplo: Full House K♠K♥K♣4♦4♠ sin descartes = 78 × 16 = **1248**.
5. Ganas al alcanzar la meta; pierdes si agotas las 20 manos.

Desde la consola de administración (`/admin`, usuario `admin`) se ajusta el ruleset de LiveOps en caliente y se ve el radar del clúster.


---

## 🧪 Pruebas

```bash
# 1) Verificación del despliegue completo (recomendado; requiere certificados y el front levantado)
bash scripts/verify.sh            # P3: TLS, proxy, rutas, WSS, segmentación, bloqueo, zero trust
bash scripts/verify_p2.sh         # contenedores, rutas del gateway, bloqueo y broker

# 2) Servicio de identidad (pytest dentro de un contenedor, sin instalar nada)
docker run --rm -v "$PWD/services/auth-identity-ms:/src" -w /src python:3.12-slim \
  sh -c "pip install -q -r requirements-dev.txt && python -m pytest -q"

# 3) CLI
docker run --rm -v "$PWD/clients/cli:/src" -w /src python:3.12-slim \
  sh -c "pip install -q -e '.[dev]' && python -m pytest -q"

# 4) Cliente web (repositorio hermano)
cd ../joker-arena-frontend && npm ci && npm run typecheck && npm test && npm run build

# 5) Integración / E2E en Python (requiere el stack levantado y los certificados)
python -m venv .venv && .venv/Scripts/pip install -r tests/requirements.txt
.venv/Scripts/python -m pytest tests -q
```

Cobertura destacada: bloqueo de cuenta y política de contraseñas (`services/auth-identity-ms/tests`), protocolo y actor de sala del motor (Go, Prototipo 1), contratos HTTP y eventos, flujo completo de partida por WebSocket y pruebas de componentes/validaciones del cliente web.

## 📁 Estructura

```text
.
├── services/                  # 8 microservicios (cada uno con su Dockerfile y compose.yml)
├── gateway/                   # API Gateway (nginx): rutas, JWT, rate limit
├── edge/reverse-proxy/        # Reverse Proxy (nginx + TLS): única entrada 80/443
│   └── (edge/certs)           # CA y certificado generados con scripts/gen_certs.sh (no versionados)
├── clients/cli/               # CLI en Python (el cliente web está en joker-arena-frontend)
├── contracts/                 # contratos HTTP, eventos, ruleset y protocolo WebSocket
├── db/                        # init de PostgreSQL (esquemas por servicio), MongoDB y Neo4j
├── docs/p2/ · docs/p3/        # documentación de los prototipos 2 y 3 (escenarios y evidencias)
├── perf/ · results/           # pruebas de carga (k6) y resultados del P3
├── scripts/                   # gen_certs.sh · verify.sh (P3) · verify_p2.sh
├── tests/                     # suites de integración/E2E en Python
├── docker-compose.p2.yml      # orquestación vigente (P3, 15 contenedores)
└── docker-compose.debug.yml   # puertos de depuración en localhost (opcional)
```

**Prototipo 1 (histórico, sin cambios):** `docker-compose.yml`, `jokerarena-game-ms/`, `jokerarena-telemetry-ms/`, `db/postgres/create_db.sql`, `db/mongo/mongo-init.js`, `p1_X.md` y `GUIA_SUSTENTACION.md` documentan la primera entrega (5 contenedores en una red plana y cliente web SPA en Nginx). Se conservan como referencia; para el P3 se usan `docker-compose.p2.yml` y `services/`.

## 🩺 Problemas comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `jokerarena-reverse-proxy` reiniciándose y `nginx: [emerg] cannot load certificate .../server.crt` | Faltan los certificados (no están en git) | `bash scripts/gen_certs.sh` y luego `docker compose -f docker-compose.p2.yml up -d --force-recreate jokerarena-reverse-proxy` |
| El navegador avisa del certificado en `https://localhost` | La CA local no está instalada | Importa `edge/certs/jokerarena-ca.crt` como autoridad de confianza, o usa `curl --cacert edge/certs/jokerarena-ca.crt` |
| `network edge-net declared as external, but could not be found` (en el frontend) | El backend no está levantado (él crea la red) | Levanta primero el backend o `docker network create edge-net` |
| `https://localhost` responde pero la web da `502` | El cliente web no está levantado (repo aparte) | `cd ../joker-arena-frontend && docker compose up -d --build --wait` |
| `Bind for 0.0.0.0:80 failed: port is already allocated` | Otro servicio ocupa 80/443 (IIS, Skype, otro nginx) | Detén ese servicio o usa `docker-compose.debug.yml` para trabajar con el gateway en 8080 |
| `423 account_locked` al iniciar sesión | Bloqueo por 5 intentos fallidos (`AUTH_LOCKOUT_MINUTES`, 15 min por defecto) | Espera el tiempo indicado o `docker exec jokerarena-redis redis-cli del auth:lock:<usuario> auth:fail:<usuario>` |
| Un contenedor queda `unhealthy` y `--wait` falla | El servicio no arrancó o no responde a su healthcheck | `docker compose -f docker-compose.p2.yml ps` y `docker logs <contenedor>` |
| `docker: invalid reference format` al pegar un comando | En PowerShell `\` **no** continúa líneas (es de bash) | Usa el comando en una línea o el backtick `` ` `` al final de cada línea |

Para empezar de cero: `docker compose -f docker-compose.p2.yml down -v` (borra volúmenes y datos) y repite el arranque rápido.

Cliente web (SSR, Next.js): <https://github.com/AngelDavidRuizB/joker-arena-frontend>

Desarrollado con fines académicos para la asignatura **Arquitectura de Software** (2026-II).

