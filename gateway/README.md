# API Gateway

Punto único de entrada del sistema. **Responsable:** Esteban Andrés Gómez Álvarez · **nginx 1.27** · puerto **8080** (el único servicio publicado al host junto con la web).

Rutas y reglas: `contracts/http-routes.md`. Configuración: `nginx.conf`.

- **Enrutamiento** de `/api/*`, `/ws/*` y `/admin/*` a cada microservicio por la red interna (resolución DNS en cada petición, así el gateway arranca aunque un servicio aún no exista).
- **Autenticación:** antes de enrutar una ruta protegida hace una subpetición `auth_request` a `auth-identity-ms /auth/verify`; en `/admin/*` exige además el rol `admin`. El token se acepta en `Authorization: Bearer` o, para WebSockets, en `?token=`.
- **WebSockets:** reenvía el upgrade del juego (`/ws/{code}`) y del radar (`/admin/cluster/metrics`).
- **CORS** para `http://localhost:*` y **límite** de 10 peticiones/s por IP en `/api/auth/*`.
- **Errores** con la forma común `{"error","message"}` (401, 403, 404, 503).

## Por qué nginx y no KrakenD

El plan inicial era KrakenD, pero su edición gratuita no hace proxy de WebSockets, y tanto el juego como el radar los necesitan (riesgo R1 de la guía del equipo). Se aplicó el plan B acordado: un gateway nginx con **las mismas rutas del contrato**, así ningún servicio ni cliente tuvo que cambiar.

## Probar

```bash
curl -s localhost:8080/health
```

```bash
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/api/rooms
```

El segundo debe responder `401` (ruta protegida sin token).
