# Ficha Técnica — `clients/admin-desktop`

| Campo              | Valor                                                                 |
|--------------------|-----------------------------------------------------------------------|
| **Componente**     | `admin-desktop`                                                       |
| **Tipo**           | Cliente de Escritorio (Presentación)                                  |
| **Responsable**    | Juan David Suárez Coronado — Task 23 (shell + login + Radar NOC)      |
| **Co-responsable** | Jaisson Fabian Patiño Orjuela — Task 25 (Editor de Reglas LiveOps)   |
| **Lenguajes**      | Rust 1.77+ (shell Tauri) · TypeScript 5 / React 18 (frontend)         |
| **Framework**      | Tauri 2 · Vite 5 · React 18                                          |
| **Puerto dev**     | `http://localhost:1420` (Vite dev server)                             |
| **Gateway**        | `http://localhost:8080` (único punto de entrada — nunca directo a MS) |
| **Rol en C&C**     | Capa de Presentación — Cliente Dual (junto a `web-ssr`)               |

---

## Responsabilidad

Interfaz de administración de escritorio para operadores del clúster JokerArena.  
Proporciona dos capacidades principales:

1. **Radar NOC** (Tasks 23 + 24): cuadrícula de semáforos en tiempo real del estado de cada contenedor, métricas de RabbitMQ / PostgreSQL / Redis y alarmas activas. Se conecta vía WebSocket al Gateway.
2. **Editor de Reglas LiveOps** (Task 25 — Jaisson): formulario para leer y publicar el `Ruleset` activo (jokers habilitados, multiplicadores, etc.) sin reiniciar el servidor.

---

## Arquitectura interna

```
src/
├── config.ts              ← GATEWAY_URL, WS_METRICS_URL, LOGIN_URL, LIVEOPS_URL
├── types/
│   └── snapshot.ts        ← Tipos TypeScript ↔ cluster-monitor-ms/model.go
├── hooks/
│   ├── useGateway.ts      ← Login JWT, logout (token en memoria, no localStorage)
│   └── useClusterWS.ts    ← WebSocket con reconexión exponencial (1s→30s)
├── components/
│   ├── Login.tsx          ← Formulario de autenticación
│   ├── Radar.tsx          ← Vista NOC: grid de contenedores + infra row
│   ├── ContainerCard.tsx  ← Tarjeta individual: semáforo + CPU/RAM/state
│   └── LiveOpsEditor.tsx  ← Placeholder para Jaisson (Task 25)
└── App.tsx                ← Shell: nav tabs (Radar | Editor) + toggle Demo
src-tauri/
├── Cargo.toml             ← Tauri 2 + serde + tauri-plugin-opener
├── build.rs               ← tauri_build::build()
├── tauri.conf.json        ← CSP: solo http/ws localhost:8080
└── src/
    ├── lib.rs             ← tauri::Builder + plugin opener
    └── main.rs            ← Punto de entrada binario
```

---

## Patrones de Diseño

| Patrón                | Aplicación                                                                              |
|-----------------------|-----------------------------------------------------------------------------------------|
| **Observer**          | `useClusterWS` suscribe al stream WS; Radar re-renderiza en cada snapshot recibido      |
| **API Gateway Client**| Toda comunicación pasa por `localhost:8080`; los MS son invisibles al cliente            |
| **Single Entry Point**| El Shell (`App.tsx`) controla autenticación y navegación; los tabs son componentes puros |
| **Façade**            | `useGateway` encapsula fetch + decodificación JWT exponiéndola como estado React simple  |

---

## Contratos de Comunicación

### Login
```
POST http://localhost:8080/api/auth/login
Body: { "alias": "admin", "password": "..." }
Response 200: { "token": "<JWT>" }
JWT claims: sub (player_id), alias, role ("admin"|"player"), jti, exp
```

### Radar NOC — WebSocket
```
ws://localhost:8080/admin/cluster/metrics?token=<JWT>
← { generated_at, interval_seconds, containers[], rabbitmq{}, postgres{}, redis{}, sources{}, alarms[] }
```
Snapshot shape definida en `services/cluster-monitor-ms/internal/monitor/model.go`.

### LiveOps Editor — REST (Task 25)
```
GET http://localhost:8080/admin/liveops/ruleset          → Ruleset activo
PUT http://localhost:8080/admin/liveops/ruleset          → Publicar nuevo Ruleset
Authorization: Bearer <JWT>
```

---

## Instrucciones de Desarrollo

### Requisitos
- Node.js 18+ y npm 9+
- Rust 1.77+ y Cargo (para `tauri build`)
- Sistema operativo: Windows / macOS / Linux

### Solo frontend (sin Tauri — para demo rápida)
```bash
cd clients/admin-desktop
npm install
npm run dev
# Abre http://localhost:1420 en el navegador
# Usar toggle "Demo" para ver el Radar sin el clúster corriendo
```

### Aplicación de escritorio completa
```bash
cd clients/admin-desktop
npm install
npm run tauri dev    # Desarrollo (hot-reload)
npm run tauri build  # Compilar instalador
```

### Variables de entorno opcionales
```bash
# .env.local (no commitear)
VITE_GATEWAY_URL=http://mi-servidor:8080
```

---

## Decisiones de Diseño

- **Token en memoria, no `localStorage`**: evita que el token JWT persista en disco en la caja del operador. Se limpia al cerrar la app.
- **Demo mode**: muestra datos mock completos sin necesidad del clúster. Crítico para la demo de la materia.
- **Reconexión exponencial**: back-off de 1s → 2s → 4s → … → 30s máximo. No satura el Gateway si está caído.
- **CSP en `tauri.conf.json`**: el webview solo puede conectar a `localhost:8080`; cualquier otra URL es bloqueada por el browser engine nativo de Tauri.
- **Sin Docker**: este cliente NO va en `docker-compose.p2.yml`. Es un binario de escritorio que el operador instala y ejecuta directamente.

---

## Integración con el Equipo

| Quién     | Qué consume / provee                                               |
|-----------|--------------------------------------------------------------------|
| **Ángel** | Provee `cluster-monitor-ms` → WS `/admin/cluster/metrics`         |
| **Nicole**| Provee `jokerarena-gateway` → punto de entrada único              |
| **Jaisson**| Integra `LiveOpsEditor.tsx` real en `clients/admin-desktop/src/`  |
| **Santiago**| `game-engine-ms` consume `bot-ai-ms` (no afecta directamente)   |
