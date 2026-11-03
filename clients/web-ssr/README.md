# web-ssr

Cliente web con **Server-Side Rendering**. **Responsable:** Jose Arturo Herrera Rivera · **Next.js 14 (App Router), React 18, TypeScript, Tailwind** · puerto **3000**.

Es la migración del cliente React del Prototipo 1 (repo `joker-arena-frontend`, sin cambios) a Next.js: se reutilizaron los componentes de la mesa, la cascada de puntuación, el store y el cliente WebSocket.

| Página | Renderizado | Datos |
|---|---|---|
| `/` | cliente | entrada por alias (`/api/auth/*`) |
| `/lobby` | **servidor** (lista de salas) + formulario interactivo | `/api/rooms` |
| `/room/{code}` | cliente | WebSocket `/ws/{code}?token=` |
| `/leaderboard` | **servidor** | `/api/leaderboard/top` |
| `/matches/{match_id}` | **servidor** | `/api/telemetry/matches/{id}` |
| `/admin` | cliente (rol `admin`) | editor de LiveOps (`/admin/liveops/*`) y radar (WebSocket `/admin/cluster/metrics`) |

Cómo se ve el SSR: "ver código fuente" en `/leaderboard` o `/lobby` muestra el HTML ya con los datos.

- El **servidor de Next** llama al gateway por la red interna (`GATEWAY_INTERNAL_URL=http://jokerarena-gateway:8080`) con el JWT de la cookie `ja_token`.
- El **navegador** llama a `/api/*` en el mismo origen y Next lo reenvía al gateway (rewrite), así que no hay CORS ni acceso directo a microservicios. El WebSocket va directo al gateway (`ws://<host>:8080`, o `NEXT_PUBLIC_GATEWAY_WS_URL`).

## Desarrollo y pruebas

```bash
npm install
```

```bash
npm test
```

```bash
GATEWAY_INTERNAL_URL=http://localhost:8080 npm run dev
```
