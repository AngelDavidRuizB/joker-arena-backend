# Documento de Arquitectura de Software — Prototipo 1
## Sistema: JokerArena — Multiverse Poker Engine

---

## 1. Identificación del Sistema y Equipo

* **Nombre del Equipo:** Grupo `[Asignar Identificador: ej. 1a, 2b]`
* **Integrantes del Equipo:**
  * `[Nombre Completo Integrante 1 - Código]`
  * `[Nombre Completo Integrante 2 - Código]`
  * `[Nombre Completo Integrante 3 - Código]`
  * `[Nombre Completo Integrante 4 - Código]`
* **Asignatura:** Arquitectura de Software (2026-II)
* **Docente:** Jeisson Andrés Vergara Vargas
* **Institución:** Universidad Nacional de Colombia — Sede Bogotá

### 1.1. Información del Sistema de Software
* **Nombre del Sistema:** **JokerArena: Multiverse Poker Engine**
* **Identidad Visual / Logo:**
  
  ```
  +-------------------------------------------------------------+
  |                        [ JOKER ARENA ]                      |
  |                                                             |
  |         .---.      .-------------------.     /\             |
  |        / ... \     | A               ♠ |    /  \            |
  |       | () () |    |    .--------.     |   / /\ \   xMult   |
  |        \  -  /     |    |   🃏   |     |  / /  \ \ [x3.5]   |
  |       /`-----'\    |    '--------'     | / / /\ \ \         |
  |      / /|   |\ \   |                 A | \/_/  \_\/         |
  |     (_/ |_|_| \_)  '-------------------'                    |
  |                                                             |
  |   [Pixel-Art Joker]      [Holo Ace Card]    [Telemetry Bus] |
  +-------------------------------------------------------------+
  ```
  *(Nota: Se incluye imagen formal `logo.png` en el encabezado del informe exportado a PDF)*.
  
* **Concepto Visual:** Un comodín estilizado en arte pixel-art/retro sosteniendo una baraja holográfica, rodeado por un nodo de interconexión de red distribuida y un display digital luminoso con multiplicadores exponenciales ($\times\text{Mult}$).

---

## 2. Descripción del Sistema y Alcance Funcional

### 2.1. Dominio y Enfoque Funcional
**JokerArena** es un videojuego web distribuido y plataforma de telemetría competitiva inspirado en las mecánicas centrales del exitoso roguelike de construcción de barajas *Balatro*. 

El sistema traslada la experiencia de evaluación de manos tradicionales de póker combinadas con modificadores pasivos y acumulativos (*Jokers*, multiplicadores exponenciales y aditivos) hacia un entorno multijugador sincronizado en red. Los jugadores compiten en salas simultáneas por superar metas de puntaje crecientes (*Blinds* / Ciegas) en rondas con restricciones de descartes y manos disponibles. 

Adicionalmente, el sistema integra una **consola de auditoría y telemetría en tiempo real**, permitiendo a espectadores y contrincantes inspeccionar el desglose algebraico exacto de cada jugada (fichas base, multiplicadores planos y escalamiento multiplicativo).

---

### 2.2. Alcance Funcional del Prototipo 1 (Prototipo Vertical)
Para cumplir con la entrega del primer prototipo vertical, se delimita un flujo de ejecución completo de extremo a extremo (*End-to-End*), que atraviesa todas las capas lógicas y físicas de la arquitectura:

1. **Gestión de Sesiones y Salas Multijugador:**
   * Creación de salas de juego con identificador alfanumérico único.
   * Conexión y sincronización de estado de 2 a 4 jugadores por sala.
   * Modo *"Simulador de Oponente / Bot"* para demostración fluida en un único entorno sin dependencias de red externa.

2. **Motor de Juego Balatro (Core Game Loop):**
   * Generación y reparto de baraja francesa estándar de 52 cartas.
   * Selección de hasta 5 cartas para jugar o descartar.
   * Algoritmo de reconocimiento determinista de manos de póker (High Card, Pair, Two Pair, Three of a Kind, Straight, Flush, Full House, Four of a Kind, Straight Flush).
   * Sistema de puntuación fundamental:
     $$\text{Puntaje Obtenido} = \text{Fichas Totales (Chips)} \times \text{Multiplicador Total (Mult)}$$

3. **Mecánica de Modificadores (Jokers Implementados):**
   * **Joker Clásico:** Otorga $+4 \text{ Mult}$ plano de adición simple a la jugada.
   * **Banner:** Otorga $+40 \text{ Chips}$ planos por cada descarte restante en la mano.
   * **The Duo:** Aplica $\times 2 \text{ Mult}$ (multiplicador exponencial) si la mano evaluada contiene un Par.

4. **Consola de Telemetría, Monitoreo y Auditoría en Vivo:**
   * Transmisión continua de cada evento de juego hacia una consola de auditoría.
   * Desglose paso a paso de la fórmula matemática en la UI del cliente espectador/oponente.
   * Persistencia inmutable del historial de eventos de cada mano jugada para análisis estadístico posterior.

---

## 3. Matriz de Requisitos No Funcionales (RNF)

| Requisito Exigido (PDF) | Solución Arquitectural en JokerArena | Justificación Técnica y Tácticas de Calidad |
| :--- | :--- | :--- |
| **Arquitectura Distribuida** | Despliegue modular en microservicios independientes comunicados mediante protocolos de red sobre TCP/IP. | **Táctica de Desacoplamiento:** Aislamiento de fallos entre el ciclo de juego en tiempo real (baja latencia) y el pipeline analítico/histórico de auditoría. |
| **$\ge 1$ Componente de Presentación** | **Frontend Web SPA:** Aplicación de página única desarrollada en **TypeScript / React + Tailwind CSS**. | **Usabilidad y Rendimiento:** Renderizado fluido del tapete interactivo de cartas, animaciones visuales de multiplicadores y visualización dual de telemetría. |
| **$\ge 2$ Componentes de Lógica** | 1. **Game Engine & Session Manager** (Go / Golang).<br>2. **Telemetry & Analytics Service** (Python / FastAPI). | **Especialización Tecnológica:** Go gestiona concurrencia masiva y sockets sin sobrecarga de memoria; Python provee un entorno idóneo para transformaciones analíticas e ingestión documental. |
| **$\ge 2$ Componentes de Datos** | 1. **PostgreSQL** (Motor Relacional ACID).<br>2. **MongoDB** (Motor NoSQL Documental). | **Persistencia Políglota:** PostgreSQL garantiza integridad referencial en cuentas y catálogos estáticos; MongoDB provee un esquema flexible y de alto rendimiento de escritura para eventos de auditoría (*Audit Trail*). |
| **$\ge 2$ Tipos de Conectores basados en HTTP** | 1. **HTTP REST** (JSON sobre HTTP/1.1).<br>2. **Server-Sent Events (SSE)** (`text/event-stream` sobre HTTP).<br>*(Conector Complementario):* **WebSockets** (vía HTTP Upgrade RFC 6455). | **Compatibilidad y Rigor:** HTTP REST para operaciones unarias síncronas (gestión de salas, catálogos); HTTP SSE para transmisión unidireccional continua de telemetría hacia dashboards; WebSockets para comunicación bidireccional sub-50ms en jugadas. |
| **$\ge 3$ Lenguajes de Programación** | 1. **TypeScript** (Capa de Presentación).<br>2. **Go** (Capa Lógica 1: Game Engine).<br>3. **Python** (Capa Lógica 2: Telemetry Service). | **Idoneidad de Propósito General:** Tres lenguajes fuertemente diferenciados por paradigma (tipado estático para cliente web, concurrencia compilada con goroutines para el motor de juego, e interpretación dinámica para analítica). |
| **Despliegue Orientado a Contenedores** | Entorno contenerizado unificado mediante **Docker** y **Docker Compose**. | **Portabilidad y Reproducibilidad:** Aislamiento de dependencias por servicio, configuración declarativa de redes virtuales (`bridge`) y volúmenes de datos. |

---

## 4. Estructuras Arquitectónicas: Component-and-Connector (C&C)

### 4.1. Estilos Arquitectónicos Utilizados

1. **Cliente-Servidor (Client-Server Style):**
   * Separa estrictamente la interfaz de usuario en el navegador de la lógica de procesamiento en el servidor. El cliente es liviano y delega toda validación de reglas de póker al backend para evitar trampas en cliente (*Server-Authoritative Game Loop*).

2. **Microservicios / Servicios Distribuidos (Distributed Services Style):**
   * Descomposición de la lógica de negocio en dos servicios desacoplados: `Game Engine` y `Telemetry Service`. Cada servicio mantiene su propio almacenamiento de datos siguiendo el patrón **Database-per-Service**.

3. **Publicación-Suscripción / Concurrencia Basada en Eventos (Publish-Subscribe & Streaming):**
   * El servicio de telemetría publica eventos de jugada a través de un canal SSE hacia clientes y observadores suscritos.
   * La comunicación de acciones de juego utiliza un canal bidireccional persistente sobre WebSockets con difusión de estados a los jugadores de la sala.

---

### 4.2. Vista de Componentes y Conectores (C&C Diagram)

```mermaid
flowchart TD
    subgraph PRESENTATION["CAPA DE PRESENTACIÓN (Client Tier)"]
        UI["Web Frontend Client<br><i>(TypeScript / React)</i><br><b>Puertos:</b><br>• p_rest_out (HTTP REST Client)<br>• p_ws_io (WebSocket Duplex)<br>• p_sse_in (SSE Receiver)"]
    end

    subgraph LOGIC["CAPA DE LÓGICA (Application Services Tier)"]
        GE["Game Engine & Session Manager<br><i>(Go / Golang)</i><br><b>Puertos:</b><br>• p_rest_srv (HTTP REST Server)<br>• p_ws_srv (WebSocket Server)<br>• p_async_dispatch (HTTP Async Out)<br>• p_sql_out (Postgres Client)"]

        TE["Telemetry & Analytics Service<br><i>(Python / FastAPI)</i><br><b>Puertos:</b><br>• p_ingest_rest (HTTP Ingest Server)<br>• p_sse_out (SSE Broadcast Server)<br>• p_mongo_out (Mongo Driver Client)"]
    end

    subgraph DATA["CAPA DE DATOS (Data Persistence Tier)"]
        PG[("PostgreSQL Database<br><i>(Relational / SQL)</i><br><b>Puertos:</b><br>• p_sql_in (Port 5432 TCP)")]

        MG[("MongoDB Database<br><i>(NoSQL Document Store)</i><br><b>Puertos:</b><br>• p_mongo_in (Port 27017 TCP)")]
    end

    %% Conectores de Presentación a Lógica
    UI -->|"Conector C1: HTTP REST [JSON]<br>(Salas, Auth, Config)"| GE
    UI <-->|"Conector C2: WebSocket [RFC 6455]<br>(Acciones de Juego en Vivo)"| GE
    UI <--|"Conector C3: HTTP SSE [text/event-stream]<br>(Telemetría y Auditoría)"| TE

    %% Conector Inter-Servicios
    GE -.->|"Conector C4: HTTP REST [POST Async]<br>(Despacho no bloqueante de jugadas)"| TE

    %% Conectores de Persistencia
    GE -->|"Conector C5: TCP Nativo PostgreSQL<br>(Driver pgx / SQL)"| PG
    TE -->|"Conector C6: TCP Wire Protocol BSON<br>(Driver Motor / PyMongo)"| MG
```

---

### 4.3. Descripción Detallada de Elementos Arquitectónicos (Componentes)

#### A. Componente de Presentación
* **`Web Frontend Client` (TypeScript / React):**
  * **Responsabilidad:** Renderizar la interfaz de usuario, capturar la interacción del jugador (seleccionar cartas, descartar, pulsar "Jugar Mano"), gestionar el estado local de la interfaz y visualizar el tablero multijugador y el panel de telemetría analítica.
  * **Puertos:**
    * `p_rest_out`: Emite peticiones HTTP síncronas al Game Engine (crear sala, listar salas, unirse).
    * `p_ws_io`: Canal bidireccional permanente con el Game Engine para recibir el estado de la partida y enviar jugadas con latencia sub-50ms.
    * `p_sse_in`: Escucha continua del canal de eventos SSE del Telemetry Service para actualizar en vivo el gráfico del desglose matemático de las jugadas.

#### B. Componentes de Lógica
* **`Game Engine & Session Manager` (Go / Golang):**
  * **Responsabilidad:** Actuar como autoridad central de juego (*Server-Authoritative*). Administra las salas multijugador en memoria mediante goroutines y canales, baraja y reparte cartas de forma segura, valida la jerarquía de las manos de póker y calcula el puntaje aplicando la cascada de Jokers:
    $$\text{Puntaje} = (\text{Chips Base} + \text{Chips Jokers}) \times (\text{Mult Base} + \text{Mult Jokers}) \times \prod (\text{Mult Exponenciales})$$
  * **Puertos:**
    * `p_rest_srv`: Expone endpoints REST para inicialización de sesiones y metadatos.
    * `p_ws_srv`: Administra las conexiones WebSocket entrantes de los jugadores de cada sala.
    * `p_async_dispatch`: Emite eventos de manos jugadas de forma asíncrona (no bloqueante) hacia el servicio de telemetría.
    * `p_sql_out`: Conexión de lectura/escritura hacia PostgreSQL mediante pool de conexiones `pgx`.

* **`Telemetry & Analytics Service` (Python / FastAPI):**
  * **Responsabilidad:** Ingestar las cargas útiles de auditoría de cada mano, calcular métricas analíticas agregadas (eficiencia de descartes, manos más recurrentes, multiplicador pico) y retransmitir los desgloses en tiempo real a los tableros de observación.
  * **Puertos:**
    * `p_ingest_rest`: Recibe las jugadas procesadas desde el Game Engine mediante peticiones HTTP POST JSON.
    * `p_sse_out`: Publica eventos continuos en formato `text/event-stream` hacia los clientes web suscritos.
    * `p_mongo_out`: Persiste los documentos analíticos en MongoDB utilizando el cliente asíncrono `motor`.

#### C. Componentes de Datos
* **`PostgreSQL Database` (Motor Relacional):**
  * **Responsabilidad:** Almacenar datos transaccionales fuertemente estructurados que requieren integridad referencial y garantías ACID: cuentas de usuario, catálogo de tipos de cartas y Jokers, y registro maestro de partidas concluidas.
* **`MongoDB Database` (Motor Documental NoSQL):**
  * **Responsabilidad:** Almacenar documentos de eventos de auditoría y telemetría bajo el patrón *Audit Trail Ledger*. Cada mano jugada genera un documento inmutable con esquema flexible que detalla la descomposición interna de la fórmula aplicada.

---

### 4.4. Descripción Detallada de Relaciones (Conectores)

1. **Conector C1: HTTP REST (JSON sobre HTTP/1.1):**
   * *Roles:* `Caller` (Frontend) $\to$ `Callee` (Game Engine).
   * *Semántica:* Síncrono, petición-respuesta estándar. Utilizado para consultas idempotentes, autenticación básica y creación de salas.

2. **Conector C2: WebSocket (RFC 6455 sobre Handshake HTTP Upgrade):**
   * *Roles:* `Peer` (Frontend) $\longleftrightarrow$ `Peer` (Game Engine).
   * *Semántica:* Full-duplex bidireccional persistente sobre TCP iniciado por negociación HTTP. Transmite mensajes JSON estructurados (`PLAY_HAND`, `DISCARD_CARDS`, `ROOM_STATE_UPDATE`).

3. **Conector C3: Server-Sent Events (SSE / HTTP Streaming):**
   * *Roles:* `Publisher` (Telemetry Service) $\to$ `Subscriber` (Frontend).
   * *Semántica:* Unidireccional continuo sobre HTTP estándar (`Content-Type: text/event-stream`). Permite al dashboard recibir telemetría en tiempo real sin sobrecargar el servidor con peticiones cíclicas (*polling*).

4. **Conector C4: HTTP REST Asíncrono Inter-Servicios:**
   * *Roles:* `Emitter` (Game Engine) $\to$ `Collector` (Telemetry Service).
   * *Semántica:* Despacho no bloqueante mediante goroutine de Go. Si el servicio de telemetría sufre degradación o caída temporal, el ciclo de juego del jugador nunca se interrumpe ni sufre latencia (Táctica de Disponibilidad y Tolerancia a Fallos).

5. **Conector C5: Protocolo Nativo PostgreSQL:**
   * *Roles:* `Client` (Game Engine) $\to$ `Server` (PostgreSQL Port 5432).
   * *Semántica:* Conexión persistente mediante pool TCP binario optimizado con `pgx`.

6. **Conector C6: MongoDB Wire Protocol:**
   * *Roles:* `Client` (Telemetry Service) $\to$ `Server` (MongoDB Port 27017).
   * *Semántica:* Operaciones asíncronas de inserción y consulta de documentos BSON sobre TCP.

---

### 4.5. Modelo de Datos del Prototipo

#### A. Esquema Relacional (PostgreSQL)
```sql
-- Catálogo estático de tipos de Jokers
CREATE TABLE jokers (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    modifier_type VARCHAR(20) NOT NULL, -- 'FLAT_CHIPS', 'FLAT_MULT', 'EXP_MULT'
    default_value NUMERIC(10,2) NOT NULL
);

-- Registro de usuarios y estadísticas consolidadas
CREATE TABLE players (
    id VARCHAR(50) PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    total_victories INT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Registro maestro de partidas
CREATE TABLE matches (
    id VARCHAR(50) PRIMARY KEY,
    room_code VARCHAR(10) NOT NULL,
    winner_id VARCHAR(50) REFERENCES players(id),
    target_score INT NOT NULL,
    status VARCHAR(20) DEFAULT 'FINISHED',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

#### B. Esquema Documental NoSQL (MongoDB - Colección `hand_telemetry`)
```json
{
  "_id": "6701f4c281e892d7b1a2001",
  "match_id": "sala-a104",
  "player_id": "usr-joker-01",
  "round_number": 2,
  "poker_hand": "Full House",
  "cards_played": [
    {"suit": "SPADES", "value": "K", "chips": 10},
    {"suit": "HEARTS", "value": "K", "chips": 10},
    {"suit": "CLUBS", "value": "K", "chips": 10},
    {"suit": "DIAMONDS", "value": "4", "chips": 4},
    {"suit": "SPADES", "value": "4", "chips": 4}
  ],
  "scoring_pipeline": {
    "base_chips": 40,
    "base_mult": 4,
    "jokers_evaluated": [
      {"joker": "Joker Clásico", "effect": "+4 Mult", "mult_added": 4},
      {"joker": "The Duo", "effect": "x2 Mult (Par detectado)", "mult_multiplier": 2.0}
    ],
    "final_chips": 78,
    "final_mult": 16.0,
    "total_hand_score": 1248
  },
  "timestamp": "2026-10-12T19:40:00Z"
}
```

---

## 5. Instrucciones de Ejecución del Prototipo

El prototipo está 100% contenerizado. Para desplegar y ejecutar el sistema en cualquier máquina host, solo se requiere tener instalado **Docker** y **Docker Compose**.

### 5.1. Estructura de Directorios del Repositorio
```plaintext
joker-arena/
├── docker-compose.yml          # Orquestador multi-contenedor del sistema
├── README.md                   # Instrucciones generales
├── frontend/                   # Componente de Presentación (TypeScript / React)
│   ├── Dockerfile
│   ├── package.json
│   ├── tsconfig.json
│   └── src/
├── game-engine/                # Componente de Lógica 1 (Go)
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   └── cmd/
│       └── server/main.go
├── telemetry-service/          # Componente de Lógica 2 (Python)
│   ├── Dockerfile
│   ├── requirements.txt
│   └── app/
│       └── main.py
└── data/
    └── init-postgres.sql       # Script DDL de inicialización automática de PostgreSQL
```

### 5.2. Comandos de Descarga y Ejecución Paso a Paso

1. **Clonar el repositorio:**
   ```bash
   git clone https://github.com/usuario-org/joker-arena.git
   cd joker-arena
   ```

2. **Construir y levantar todos los contenedores:**
   ```bash
   docker compose up --build -d
   ```

3. **Verificar el estado de salud de los servicios:**
   ```bash
   docker compose ps
   ```

4. **Comprobar logs en tiempo real (opcional):**
   ```bash
   docker compose logs -f
   ```

### 5.3. Puntos de Acceso del Sistema (URLs de Validación)

| Componente | Dirección Local | Propósito de Verificación |
| :--- | :--- | :--- |
| **Frontend Web** | `http://localhost:3000` | Interfaz interactiva de juego y consola de telemetría. |
| **Game Engine (Go)** | `http://localhost:8080/health` | Verificación de estado del motor de juego y WebSockets (`ws://localhost:8080/ws`). |
| **Telemetry Service (Python)** | `http://localhost:8000/docs` | Documentación interactiva Swagger UI y endpoints SSE (`http://localhost:8000/api/telemetry/stream`). |
| **PostgreSQL** | `localhost:5432` | Puerto expuesto para inspección con DBeaver/pgAdmin (`user: postgres`, `db: jokerarena_db`). |
| **MongoDB** | `localhost:27017` | Puerto expuesto para inspección con MongoDB Compass (`mongodb://localhost:27017`). |

### 5.4. Guía Rápida de Prueba Vertical (Live Demo)
1. Abrir `http://localhost:3000` en el navegador.
2. Hacer clic en **"Crear Sala"** y activar la opción **"Simular Oponente (Bot)"**.
3. Seleccionar hasta 5 cartas de la mano repartida en pantalla y presionar **"Jugar Mano"**.
4. Observar cómo el tapete de juego actualiza el puntaje con animaciones, mientras el panel lateral derecho recibe por SSE y renderiza en vivo el desglose de Fichas $\times$ Multiplicador almacenado en MongoDB.
5. Para detener y limpiar el entorno:
   ```bash
   docker compose down -v
   ```
