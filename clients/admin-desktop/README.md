# JokerArena Admin Desktop

> Panel de administración de escritorio para operadores del clúster JokerArena.

**Responsable:** Juan David Suárez Coronado  
**Tasks:** 23 (shell + login + Radar NOC) · 25 placeholder (Jaisson)

---

## Características

| Pestaña | Descripción |
|---------|-------------|
| 📡 **Radar NOC** | Cuadrícula de semáforos de contenedores en tiempo real vía WebSocket. Muestra CPU, RAM, health, alarmas, RabbitMQ, PostgreSQL y Redis. |
| ✏️ **Editor de Reglas** | Placeholder para Jaisson (Task 25) — edición en caliente del Ruleset activo. |

- 🟢 **Demo mode**: datos mock integrados, funciona sin el clúster corriendo.
- 🔄 **Reconexión automática**: back-off exponencial 1s → 30s.
- 🛡 **Token en memoria**: el JWT nunca se persiste en disco.

---

## Requisitos

- **Node.js** 18+ · **npm** 9+
- **Rust** 1.77+ · **Cargo** (solo para `tauri build`)
- Gateway corriendo en `http://localhost:8080` (o configurar `VITE_GATEWAY_URL`)

---

## Desarrollo rápido (solo navegador)

```bash
npm install
npm run dev
# → http://localhost:1420
```

Activa el toggle **Demo** en la barra de navegación para ver el Radar sin el clúster real.

## Aplicación de escritorio (Tauri)

```bash
npm install
npm run tauri dev     # con hot-reload
npm run tauri build   # genera instalador en src-tauri/target/release/bundle/
```

---

## Configuración

Crear `.env.local` (no commitear) para apuntar a otro gateway:

```env
VITE_GATEWAY_URL=http://192.168.1.100:8080
```

---

## Endpoints consumidos

Todos vía `http://localhost:8080` (API Gateway — nunca directo a microservicios):

| Método | Ruta | Uso |
|--------|------|-----|
| `POST` | `/api/auth/login` | Obtener JWT de administrador |
| `WS`   | `/admin/cluster/metrics?token=<JWT>` | Stream de snapshots del clúster |
| `GET`  | `/admin/liveops/ruleset` | Leer Ruleset activo (Task 25) |
| `PUT`  | `/admin/liveops/ruleset` | Publicar nuevo Ruleset (Task 25) |

---

## Estructura

```
clients/admin-desktop/
├── src/
│   ├── App.tsx                  ← Shell: auth + tabs
│   ├── config.ts                ← URLs del gateway
│   ├── components/
│   │   ├── Login.tsx            ← Pantalla de autenticación
│   │   ├── Radar.tsx            ← Vista NOC completa
│   │   ├── ContainerCard.tsx    ← Tarjeta de contenedor individual
│   │   └── LiveOpsEditor.tsx    ← Placeholder para Jaisson
│   ├── hooks/
│   │   ├── useGateway.ts        ← Login/logout, JWT en memoria
│   │   └── useClusterWS.ts      ← WebSocket con reconexión
│   └── types/
│       └── snapshot.ts          ← Tipos ↔ cluster-monitor-ms/model.go
└── src-tauri/
    ├── Cargo.toml
    ├── tauri.conf.json          ← CSP: solo localhost:8080
    └── src/main.rs
```

---

## Nota para Jaisson (Task 25)

Para integrar el Editor de Reglas real, reemplaza el contenido de  
`src/components/LiveOpsEditor.tsx`.  
El prop `token: string` ya llega listo para los headers `Authorization: Bearer <token>`.

