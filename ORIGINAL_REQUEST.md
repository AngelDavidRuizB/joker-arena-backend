# Original User Request

## 2026-10-06T05:29:30Z

Construir e implementar de forma demostrable el 40% del Prototipo 1 de JokerArena (Multiverse Poker Engine), abarcando la infraestructura multi-contenedor completa en Docker Compose, los Slices Verticales V-1 y V-2 de extremo a extremo (Frontend Web SPA en React/TypeScript, Game Engine en Go con HTTP REST y WebSockets, y persistencia relacional en PostgreSQL), y el esqueleto de salud y contratos gRPC del servicio de telemetría en Python.

Working directory: d:/Usuarios/Documents/Universidad/2026-2/Arquitectura Software/Prototipo 1

Integrity mode: development

## Requirements

### R1. Infraestructura Multi-Contenedor y Persistencia Políglota
Configurar el entorno distribuido en Docker Compose con 5 servicios: `frontend` (React/Vite/Nginx en puerto 3000), `game-engine` (Go en puerto 8080), `telemetry-service` (Python/FastAPI en puerto 8000 y 50051), `postgres` (PostgreSQL 16 en puerto 5432) y `mongodb` (MongoDB 7 en puerto 27017). Incluir scripts de inicialización DDL y seed en PostgreSQL para catálogos y usuarios (`db/postgres/init.sql`), volúmenes de datos y healthchecks automáticos.

### R2. Flujo Vertical 1: Registro de Jugadores y Creación de Salas (REST C1 + PostgreSQL C6)
Implementar en el Frontend la vista de inicio y lobby para capturar el alias del jugador y permitir la creación de salas (con código alfanumérico de 6 caracteres y selector de modo Bot). El Game Engine en Go debe procesar los endpoints REST (`POST /api/players`, `POST /api/rooms`, `GET /api/rooms/{code}`), validar datos y persistir jugadores y partidas en PostgreSQL mediante pool de conexiones `pgx`.

### R3. Flujo Vertical 2: Sincronización en Tiempo Real y Reparto de Cartas (WebSocket C2)
Implementar el conector WebSocket bidireccional permanente entre el Frontend y el Game Engine. El Game Engine debe gestionar el ciclo de vida de la sala mediante goroutines, inicializar la baraja francesa de 52 cartas, barajar deterministamente y emitir el evento `ROOM_STATE_UPDATE` repartiendo 8 cartas a la mano del jugador. El Frontend debe renderizar el tapete interactivo con las cartas y permitir la selección/deselección de hasta 5 cartas.

### R4. Esqueleto de Telemetría y Contrato gRPC
Configurar el microservicio de telemetría en Python con FastAPI y el cliente MongoDB (`motor`). Proveer la definición formal del archivo de contrato Protocol Buffers (`proto/telemetry.proto`) para el conector gRPC (C5) entre Go y Python, con endpoints `/health` y `/docs` operativos.

## Acceptance Criteria

### Infraestructura y Despliegue
- [ ] El comando `docker compose up --build -d` levanta los 5 contenedores sin errores y todos pasan a estado `healthy` o `running`.
- [ ] PostgreSQL inicializa automáticamente las tablas `players`, `jokers`, `matches` y `match_players` con el catálogo semilla de Jokers.
- [ ] MongoDB responde en el puerto 27017 y el Telemetry Service se conecta exitosamente.

### Funcionalidad Vertical 40% (E2E)
- [ ] Un usuario puede ingresar a `http://localhost:3000`, registrar un alias y recibir confirmación persistida en PostgreSQL (`SELECT * FROM players`).
- [ ] El usuario puede crear una sala con código único y el botón "Jugar contra Bot" crea la sesión de juego en PostgreSQL.
- [ ] El Frontend establece la conexión WebSocket en `ws://localhost:8080/ws/{room_code}` sin errores de handshake.
- [ ] Al conectarse a la sala, el jugador recibe en tiempo real su mano inicial de 8 cartas de la baraja de 52 y el tapete web las renderiza interactivamente.
- [ ] El jugador puede hacer clic sobre las cartas para seleccionar hasta 5 cartas con retroalimentación visual reactiva.
- [ ] El endpoint de salud del Telemetry Service `http://localhost:8000/health` responde HTTP 200 OK y `/docs` expone la documentación OpenAPI.
