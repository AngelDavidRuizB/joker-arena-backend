UNIVERSIDAD NACIONAL DE COLOMBIA — SEDE BOGOTÁ

Arquitectura de Software (2026-II)

Documento de Arquitectura de Software

Prototipo 1 — Prototipo Vertical

JokerArena

Multiverse Poker Engine

1. Identificación del Sistema y Equipo

1.1. Información del sistema de software

2. Descripción del Sistema y Alcance (Scope)

2.1. Dominio y enfoque funcional

JokerArena es un videojuego web distribuido y plataforma de telemetría competitiva inspirado en las mecánicas centrales del roguelike de construcción de barajas Balatro.

El sistema traslada la evaluación de manos tradicionales de póker, combinadas con modificadores pasivos y acumulativos (Jokers, multiplicadores aditivos y exponenciales), a un entorno multijugador sincronizado en red. Los jugadores compiten en salas simultáneas por superar metas de puntaje crecientes (Blinds / Ciegas), en rondas con restricciones de descartes y manos disponibles.

Adicionalmente, el sistema integra una consola de auditoría y telemetría en tiempo real que permite a espectadores y rivales inspeccionar el desglose algebraico exacto de cada jugada: fichas base, multiplicadores planos y escalamiento multiplicativo.

2.2. Alcance funcional del Prototipo 1 (Prototipo Vertical)

El Prototipo 1 abarca un flujo funcional completo de extremo a extremo (End-to-End) que atraviesa todas las capas lógicas y físicas de la arquitectura:

Gestión de salas y jugadores. Creación de salas con un código alfanumérico único para 2 a 4 jugadores, con sincronización de estado entre ellos. Incluye un modo Simulador de Oponente / Bot para pruebas y demostraciones locales sin dependencias de red externa.

Core Game Loop (motor de juego). Administra la baraja francesa de 52 cartas; permite seleccionar o descartar hasta 5 cartas; reconoce de forma determinista las manos de póker (desde Par hasta Escalera Real, incluyendo Carta Alta, Doble Par, Trío, Escalera, Color, Full House y Póker); y calcula la puntuación con la fórmula fundamental:

Puntaje Obtenido = Fichas Totales (Chips) × Multiplicador Total (Mult)

Mecánica de modificadores (Jokers implementados). Tres Jokers cubren los tres tipos de efecto del sistema (ver tabla 2.3).

Consola de telemetría y auditoría en vivo. Muestra a espectadores y rivales el desglose algebraico paso a paso de cada jugada en tiempo real y guarda un historial inmutable de cada mano en la base de datos para su análisis estadístico posterior.

2.3. Jokers implementados

La cascada de evaluación que aplica el Game Engine combina los tres tipos de modificador en este orden:

Puntaje = (Chips Base + Chips Jokers) × (Mult Base + Mult Jokers) × Π (Mult Exponenciales)

3. Matriz de Requisitos No Funcionales (RNF)

La siguiente matriz relaciona cada requisito exigido con la solución arquitectural adoptada y la táctica de calidad que la justifica.

4. Estructura y Arquitectura del Programa

4.1. Visión general y estilos arquitectónicos

El sistema se diseña como una arquitectura distribuida de microservicios organizada en tres capas (presentación, lógica de negocio y persistencia) bajo un estilo Cliente-Servidor con servidor autoritativo (Server-Authoritative). Se combinan tres estilos:

Cliente-Servidor. El cliente es liviano: delega toda validación de reglas de póker al backend, lo que evita trampas desde el navegador.

Microservicios / servicios distribuidos. La lógica de negocio se descompone en dos servicios desacoplados (Game Engine y Telemetry Service), cada uno con su propio almacenamiento bajo el patrón Database-per-Service.

Publicación-Suscripción y streaming de eventos. Las acciones de juego viajan por un canal bidireccional persistente (WebSockets) con difusión de estado a la sala, y la telemetría se publica por un canal SSE hacia los clientes suscritos.

4.2. Capas y componentes

4.3. Conectores y protocolos de comunicación

4.4. Estructura interna de los componentes

Organización interna propuesta para el prototipo; los nombres de módulos pueden ajustarse durante la implementación.

Web Frontend Client — `frontend/src/`

Game Engine & Session Manager — `game-engine/` (Go)

Telemetry & Analytics Service — `telemetry-service/app/` (Python)

4.5. Flujo de una jugada (End-to-End)

5. Vistas Arquitectónicas: C&C y Despliegue

5.1. Vista de Componentes y Conectores (C&C) — microservicios

Figura 1. Vista C&C de JokerArena: dos microservicios con base de datos propia (Database-per-Service) y seis conectores.

5.2. Vista de despliegue (Docker Compose)

Figura 2. Cada componente se empaqueta en un contenedor independiente dentro de la red virtual jokerarena-net. El Web Frontend Client se ejecuta en el navegador; su contenedor solo sirve la SPA.

5.3. Elementos de la vista C&C (componentes y puertos)

5.4. Conectores: roles y semántica

5.5. Decisiones de microservicios y tácticas de calidad

Evolución posible (fuera del alcance del Prototipo 1): incorporar un API Gateway como punto de entrada único y un message broker que sustituya al conector C4 para garantizar la entrega de eventos con reintentos. La descomposición actual en servicios independientes permite añadirlos sin rediseñar los componentes existentes.

5.6. Estructura de despliegue (Docker)

El proyecto se empaqueta en contenedores independientes orquestados con Docker Compose, con la siguiente estructura de repositorio:

Las dependencias de arranque indicadas son la propuesta para `docker-compose.yml`.

6. Modelo de Datos del Prototipo

6.1. Esquema relacional (PostgreSQL)

Pertenece al Game Engine y garantiza integridad referencial para el catálogo, las cuentas y el registro de partidas.

6.2. Esquema documental (MongoDB — colección hand_telemetry)

Pertenece al Telemetry Service. Cada mano jugada genera un documento inmutable (Audit Trail Ledger) con la descomposición interna de la fórmula aplicada.

6.3. Verificación del ejemplo

7. Instrucciones de Ejecución del Prototipo

El prototipo está 100 % contenerizado. Para desplegarlo en cualquier máquina solo se requiere tener instalados Docker y Docker Compose.

7.1. Descarga y ejecución paso a paso

Clonar el repositorio:

Construir y levantar todos los contenedores:

Verificar el estado de los servicios:

Consultar los logs en tiempo real (opcional):

7.2. Puntos de acceso (URLs de validación)

7.3. Guía rápida de prueba vertical (live demo)

Abrir http://localhost:3000 en el navegador.

Hacer clic en «Crear Sala» y activar la opción «Simular Oponente (Bot)».

Seleccionar hasta 5 cartas de la mano repartida y presionar «Jugar Mano».

Observar cómo el tapete actualiza el puntaje con animaciones, mientras el panel lateral derecho recibe por SSE y muestra en vivo el desglose de Fichas × Multiplicador almacenado en MongoDB.

Para detener y limpiar el entorno:


--- TABLE ---
Equipo | Grupo [Asignar identificador: ej. 1a, 2b]
Integrantes | [Nombre completo integrante 1 — Código] [Nombre completo integrante 2 — Código] [Nombre completo integrante 3 — Código] [Nombre completo integrante 4 — Código]
Docente | Jeisson Andrés Vergara Vargas
Fecha | Octubre de 2026


--- TABLE ---
Nombre del equipo | Grupo [Asignar identificador: ej. 1a, 2b]
Integrantes | [Nombre completo integrante 1 — Código] [Nombre completo integrante 2 — Código] [Nombre completo integrante 3 — Código] [Nombre completo integrante 4 — Código]
Asignatura | Arquitectura de Software (2026-II)
Docente | Jeisson Andrés Vergara Vargas
Institución | Universidad Nacional de Colombia — Sede Bogotá


--- TABLE ---
Nombre del sistema | JokerArena: Multiverse Poker Engine
Concepto visual | Un comodín estilizado en arte pixel-art/retro que sostiene una baraja holográfica, rodeado por un nodo de interconexión de red distribuida y un display digital luminoso con multiplicadores exponenciales (×Mult). El logo (logo.png) se incluye en el encabezado del informe exportado a PDF.
Naturaleza | Videojuego web distribuido y plataforma de telemetría competitiva, inspirado en las mecánicas de construcción de barajas (deck-builder) del juego Balatro.


--- TABLE ---
Joker | Efecto | Tipo de modificador | Condición de activación
Joker Clásico | +4 Mult (adición plana) | FLAT_MULT | Siempre que se juegue una mano.
Banner | +40 Chips por cada descarte restante | FLAT_CHIPS | Proporcional a los descartes que le queden al jugador.
The Duo | ×2 Mult (multiplicador exponencial) | EXP_MULT | Solo si la mano evaluada contiene un Par.


--- TABLE ---
Requisito exigido | Solución en JokerArena | Justificación técnica y tácticas de calidad
Arquitectura distribuida | Despliegue modular en microservicios independientes que se comunican mediante protocolos de red sobre TCP/IP. | Táctica de desacoplamiento: aislamiento de fallos entre el ciclo de juego en tiempo real (baja latencia) y el pipeline analítico/histórico de auditoría.
≥ 1 componente de presentación | Frontend Web SPA en TypeScript / React + Tailwind CSS. | Usabilidad y rendimiento: renderizado fluido del tapete interactivo de cartas, animaciones de multiplicadores y visualización dual de telemetría.
≥ 2 componentes de lógica | 1. Game Engine & Session Manager (Go). 2. Telemetry & Analytics Service (Python / FastAPI). | Especialización tecnológica: Go gestiona concurrencia masiva y sockets con poca sobrecarga de memoria; Python ofrece un entorno idóneo para transformaciones analíticas e ingestión documental.
≥ 2 componentes de datos | 1. PostgreSQL (relacional ACID). 2. MongoDB (NoSQL documental). | Persistencia políglota: PostgreSQL garantiza integridad referencial en cuentas y catálogos estáticos; MongoDB aporta esquema flexible y alto rendimiento de escritura para eventos de auditoría (Audit Trail).
≥ 2 tipos de conectores basados en HTTP | 1. HTTP REST (JSON sobre HTTP/1.1). 2. Server-Sent Events (text/event-stream). Complementario: WebSockets (HTTP Upgrade, RFC 6455). | Compatibilidad y rigor: REST para operaciones síncronas (salas, catálogos); SSE para transmisión unidireccional continua de telemetría hacia dashboards; WebSockets para comunicación bidireccional sub-50 ms en las jugadas.
≥ 3 lenguajes de programación | 1. TypeScript (presentación). 2. Go (lógica 1: Game Engine). 3. Python (lógica 2: Telemetry). | Idoneidad de propósito: tipado estático para el cliente web, concurrencia compilada con goroutines para el motor de juego e interpretación dinámica para analítica.
Despliegue orientado a contenedores | Entorno contenerizado unificado con Docker y Docker Compose. | Portabilidad y reproducibilidad: aislamiento de dependencias por servicio, redes virtuales (bridge) y volúmenes de datos declarados.


--- TABLE ---
Capa | Componente | Tecnología | Responsabilidad
A. Presentación (Client Tier) | Web Frontend Client | TypeScript / React + Tailwind CSS | Aplicación SPA que renderiza el tapete interactivo de cartas, las animaciones de multiplicadores y el panel dual de telemetría. Captura las acciones del jugador (seleccionar, descartar, jugar mano).
B. Lógica de negocio (Application Services Tier) | Game Engine & Session Manager | Go / Golang | Autoridad central del juego. Administra las partidas en memoria mediante goroutines, procesa las reglas del póker, aplica la cascada de Jokers y mantiene la sincronización de la sala.
 | Telemetry & Analytics Service | Python / FastAPI | Ingiere las jugadas enviadas por el Game Engine, calcula estadísticas agregadas (eficiencia de descartes, multiplicadores pico) y transmite la telemetría en tiempo real.
C. Persistencia políglota (Data Tier) | PostgreSQL | SQL relacional | Integridad ACID para datos estructurados: catálogo de Jokers (jokers), cuentas de usuario (players) y registro maestro de partidas (matches).
 | MongoDB | NoSQL documental | Eventos de auditoría inmutables en la colección hand_telemetry, con esquema flexible para detallar la descomposición interna de cada mano.


--- TABLE ---
Conector | Protocolo | Enlace | Uso
C1 | HTTP REST (JSON) | Frontend → Game Engine | Operaciones síncronas: autenticación y creación/consulta de salas.
C2 | WebSockets (RFC 6455) | Frontend ↔ Game Engine | Acciones de juego en tiempo real con latencia sub-50 ms.
C3 | Server-Sent Events | Telemetry Service → Frontend | Actualización continua de la telemetría en vivo.
C4 | HTTP REST asíncrono | Game Engine → Telemetry Service | Envío no bloqueante de cada jugada (goroutine), sin añadir latencia al jugador si la analítica se degrada.
C5 | TCP nativo PostgreSQL | Game Engine → PostgreSQL | Persistencia relacional mediante pool de conexiones (pgx).
C6 | MongoDB Wire Protocol (BSON) | Telemetry Service → MongoDB | Inserción y consulta de documentos de auditoría.


--- TABLE ---
Módulo | Responsabilidad
components/ (Tapete, Mano, Cartas, Lobby) | Renderizado del tapete interactivo, selección de hasta 5 cartas, botones «Jugar Mano» / «Descartar» y creación/unión a salas.
components/TelemetryPanel | Panel dual de telemetría: desglose paso a paso de Fichas × Multiplicador y animaciones de multiplicadores.
hooks/ (conexión WebSocket y SSE) | Gestión del ciclo de vida de los puertos p_ws_io (duplex) y p_sse_in (receptor de eventos).
services/api | Cliente HTTP REST (p_rest_out) hacia el Game Engine.
state/ y types/ | Estado local de la interfaz y tipos TypeScript de los mensajes (PLAY_HAND, DISCARD_CARDS, ROOM_STATE_UPDATE).


--- TABLE ---
Paquete | Responsabilidad
cmd/server/main.go | Punto de entrada: configura el servidor HTTP, el pool pgx y el despachador de telemetría.
internal/api | Handlers REST (p_rest_srv): salas, metadatos y /health.
internal/ws | Servidor WebSocket (p_ws_srv): conexiones de los jugadores, recepción de acciones y difusión de ROOM_STATE_UPDATE.
internal/room | Session Manager: una sala por goroutine con canales, 2 a 4 jugadores, código alfanumérico único y modo Bot.
internal/deck | Generación, barajado y reparto de la baraja francesa de 52 cartas.
internal/evaluator | Reconocimiento determinista de manos de póker.
internal/scoring + internal/jokers | Pipeline de puntuación Chips × Mult y cascada de Jokers (Joker Clásico, Banner, The Duo).
internal/telemetry | Cliente HTTP asíncrono (p_async_dispatch) que despacha cada jugada al Telemetry Service.
internal/store | Acceso a PostgreSQL (p_sql_out): jokers, players, matches.


--- TABLE ---
Módulo | Responsabilidad
main.py | Aplicación FastAPI y registro de rutas; documentación Swagger en /docs.
Ruta de ingesta | Recibe por HTTP POST JSON las jugadas del Game Engine (p_ingest_rest).
Ruta de streaming | Publica eventos text/event-stream en /api/telemetry/stream (p_sse_out).
Servicio de analítica | Calcula métricas agregadas: eficiencia de descartes, manos más recurrentes y multiplicador pico.
Repositorio Mongo | Persistencia asíncrona (cliente motor, p_mongo_out) en la colección hand_telemetry.


--- TABLE ---
# | Conector | Descripción
1 | C1 | El jugador crea una sala (opción «Simular Oponente (Bot)») y recibe su código alfanumérico.
2 | C2 | El cliente abre el WebSocket; el Game Engine reparte cartas de la baraja de 52 y difunde el estado de la sala.
3 | C2 | El jugador selecciona hasta 5 cartas y envía PLAY_HAND (o DISCARD_CARDS).
4 | — | El Game Engine valida la acción (servidor autoritativo), reconoce la mano, aplica la cascada de Jokers y calcula el puntaje.
5 | C2 | El Game Engine difunde ROOM_STATE_UPDATE a todos los jugadores de la sala.
6 | C4 | En una goroutine, despacha la jugada al Telemetry Service sin bloquear el ciclo de juego.
7 | C6 | El Telemetry Service persiste el documento inmutable en hand_telemetry y actualiza las métricas agregadas.
8 | C3 | El Telemetry Service emite el evento por SSE; el panel lateral muestra en vivo el desglose Chips × Mult.
9 | C5 | Al concluir la partida, el Game Engine registra el resultado en matches y actualiza total_victories del ganador.


--- TABLE ---
Componente | Puertos | Descripción del puerto
Web Frontend Client | p_rest_out p_ws_io p_sse_in | Emite peticiones HTTP síncronas al Game Engine (crear, listar, unirse a sala). Canal bidireccional permanente para recibir el estado de la partida y enviar jugadas. Escucha continua del canal SSE del Telemetry Service para el desglose en vivo.
Game Engine & Session Manager | p_rest_srv p_ws_srv p_async_dispatch p_sql_out | Expone endpoints REST de sesiones y metadatos. Administra las conexiones WebSocket de los jugadores de cada sala. Emite de forma asíncrona cada mano jugada hacia la telemetría. Lectura/escritura hacia PostgreSQL mediante pool pgx.
Telemetry & Analytics Service | p_ingest_rest p_sse_out p_mongo_out | Recibe las jugadas procesadas (HTTP POST JSON). Publica eventos text/event-stream hacia los clientes suscritos. Persiste los documentos analíticos en MongoDB con el cliente asíncrono motor.
PostgreSQL | p_sql_in | Puerto 5432/TCP.
MongoDB | p_mongo_in | Puerto 27017/TCP.


--- TABLE ---
Conector | Roles | Semántica
C1 HTTP REST (JSON / HTTP 1.1) | Caller (Frontend) → Callee (Game Engine) | Síncrono, petición-respuesta. Consultas idempotentes, autenticación básica y creación de salas.
C2 WebSocket (RFC 6455) | Peer (Frontend) ↔ Peer (Game Engine) | Full-duplex persistente sobre TCP, iniciado con handshake HTTP Upgrade. Mensajes JSON: PLAY_HAND, DISCARD_CARDS, ROOM_STATE_UPDATE.
C3 SSE (text/event-stream) | Publisher (Telemetry) → Subscriber (Frontend) | Unidireccional continuo sobre HTTP estándar; evita el polling cíclico.
C4 HTTP REST asíncrono | Emitter (Game Engine) → Collector (Telemetry) | Despacho no bloqueante en una goroutine. Si la telemetría cae o se degrada, el juego nunca se interrumpe ni sufre latencia.
C5 Protocolo nativo PostgreSQL | Client (Game Engine) → Server (PostgreSQL :5432) | Conexión persistente mediante pool TCP binario optimizado con pgx.
C6 MongoDB Wire Protocol | Client (Telemetry) → Server (MongoDB :27017) | Operaciones asíncronas de inserción y consulta de documentos BSON sobre TCP.


--- TABLE ---
Decisión | Aplicación en JokerArena | Beneficio
Database-per-Service | PostgreSQL es propiedad exclusiva del Game Engine; MongoDB, del Telemetry Service. Ningún servicio accede a la base de datos del otro. | Bajo acoplamiento; cada servicio elige el motor adecuado a su carga (ACID vs. alto volumen de escritura).
Comunicación asíncrona entre servicios | El conector C4 se ejecuta en goroutines, fuera de la ruta crítica de la jugada. | Disponibilidad y tolerancia a fallos: la analítica no degrada el juego.
Servidor autoritativo | Barajado, evaluación de manos y cálculo de puntaje ocurren solo en el Game Engine. | Integridad: el cliente no puede falsificar resultados.
Streaming desacoplado | La telemetría llega al cliente por SSE directamente desde el Telemetry Service, no a través del Game Engine. | El canal de juego (C2) queda libre para la baja latencia.
Contenerización por servicio | Un contenedor por componente, orquestado con Docker Compose. | Despliegue reproducible y escalado independiente de cada servicio.


--- TABLE ---
joker-arena/ ├── docker-compose.yml          # Orquestador multi-contenedor ├── README.md                   # Instrucciones generales ├── frontend/                   # Presentación (TypeScript / React) │   ├── Dockerfile │   ├── package.json │   ├── tsconfig.json │   └── src/ ├── game-engine/                # Lógica 1 (Go): Game Engine │   ├── Dockerfile │   ├── go.mod │   ├── go.sum │   └── cmd/server/main.go ├── telemetry-service/          # Lógica 2 (Python): Telemetry │   ├── Dockerfile │   ├── requirements.txt │   └── app/main.py └── data/     └── init-postgres.sql       # DDL de inicialización de PostgreSQL


--- TABLE ---
Servicio | Puerto | Tecnología | Notas
frontend | 3000 | TypeScript / React | Sirve la SPA; la lógica de cliente corre en el navegador.
game-engine | 8080 | Go | REST, WebSocket (/ws) y verificación de estado (/health). Depende de PostgreSQL.
telemetry-service | 8000 | Python / FastAPI | Ingesta, SSE y Swagger (/docs). Depende de MongoDB.
postgres | 5432 | PostgreSQL | Se inicializa con data/init-postgres.sql; volumen de datos persistente.
mongodb | 27017 | MongoDB | Volumen de datos persistente; colección hand_telemetry.


--- TABLE ---
-- Catálogo estático de tipos de Jokers CREATE TABLE jokers (     id VARCHAR(50) PRIMARY KEY,     name VARCHAR(100) NOT NULL,     description TEXT NOT NULL,     modifier_type VARCHAR(20) NOT NULL, -- 'FLAT_CHIPS', 'FLAT_MULT', 'EXP_MULT'     default_value NUMERIC(10,2) NOT NULL );   -- Registro de usuarios y estadísticas consolidadas CREATE TABLE players (     id VARCHAR(50) PRIMARY KEY,     username VARCHAR(100) UNIQUE NOT NULL,     total_victories INT DEFAULT 0,     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP );   -- Registro maestro de partidas CREATE TABLE matches (     id VARCHAR(50) PRIMARY KEY,     room_code VARCHAR(10) NOT NULL,     winner_id VARCHAR(50) REFERENCES players(id),     target_score INT NOT NULL,     status VARCHAR(20) DEFAULT 'FINISHED',     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP );


--- TABLE ---
{   "_id": "6701f4c281e892d7b1a2001",   "match_id": "sala-a104",   "player_id": "usr-joker-01",   "round_number": 2,   "poker_hand": "Full House",   "cards_played": [     {"suit": "SPADES",   "value": "K", "chips": 10},     {"suit": "HEARTS",   "value": "K", "chips": 10},     {"suit": "CLUBS",    "value": "K", "chips": 10},     {"suit": "DIAMONDS", "value": "4", "chips": 4},     {"suit": "SPADES",   "value": "4", "chips": 4}   ],   "scoring_pipeline": {     "base_chips": 40,     "base_mult": 4,     "jokers_evaluated": [       {"joker": "Joker Clásico", "effect": "+4 Mult", "mult_added": 4},       {"joker": "The Duo", "effect": "x2 Mult (Par detectado)", "mult_multiplier": 2.0}     ],     "final_chips": 78,     "final_mult": 16.0,     "total_hand_score": 1248   },   "timestamp": "2026-10-12T19:40:00Z" }


--- TABLE ---
Paso | Cálculo | Resultado
Chips finales | Chips base (40) + chips de las cartas (10+10+10+4+4 = 38) | 78
Mult tras Joker Clásico | Mult base (4) + 4 | 8
Mult tras The Duo | 8 × 2 (la mano contiene un Par) | 16
Puntaje de la mano | 78 × 16 | 1248


--- TABLE ---
git clone https://github.com/usuario-org/joker-arena.git cd joker-arena


--- TABLE ---
docker compose up --build -d


--- TABLE ---
docker compose ps


--- TABLE ---
docker compose logs -f


--- TABLE ---
Componente | Dirección local | Propósito de verificación
Frontend Web | http://localhost:3000 | Interfaz interactiva de juego y consola de telemetría.
Game Engine (Go) | http://localhost:8080/health | Estado del motor de juego; WebSocket en ws://localhost:8080/ws.
Telemetry Service (Python) | http://localhost:8000/docs | Swagger UI y endpoint SSE http://localhost:8000/api/telemetry/stream.
PostgreSQL | localhost:5432 | Inspección con DBeaver/pgAdmin (user: postgres, db: jokerarena_db).
MongoDB | localhost:27017 | Inspección con MongoDB Compass (mongodb://localhost:27017).


--- TABLE ---
docker compose down -v

