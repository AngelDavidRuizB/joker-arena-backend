# cluster-monitor-ms

NOC del clúster. **Responsable:** Ángel David Ruiz Barbosa · **Go 1.22** · puerto interno **8088**.

Cada 5 s arma un snapshot con: estado, salud, CPU y memoria de cada contenedor del proyecto `jokerarena` (Docker Engine API); mensajes y colas de RabbitMQ (management API); conexiones de PostgreSQL (`pg_stat_activity` con el usuario de solo lectura `svc_monitor`); clientes de Redis (`INFO clients`). Calcula un semáforo por contenedor y alarmas.

## Endpoints (por el gateway, rol admin)

- `GET /admin/cluster/snapshot` — último snapshot (JSON).
- `WS /admin/cluster/metrics?token=<jwt>` — envía el snapshot al conectar y luego uno por tick. Un cliente lento se salta ticks: nunca bloquea al recolector.
- `GET /health`.

## Semáforo y alarmas

| Condición | Semáforo / alarma |
|---|---|
| Contenedor detenido o `unhealthy` | rojo · crítica |
| `starting`, CPU ≥ 80 % o memoria ≥ 85 % | amarillo · advertencia |
| Una sonda no responde (Docker, RabbitMQ, PostgreSQL, Redis) | fuente `degraded` · crítica (las demás siguen reportando) |
| Conexiones de PostgreSQL ≥ 80 % del máximo | advertencia |
| Cola con ≥ 1000 mensajes o sin consumidores | advertencia |

## ⚠️ Concesión de seguridad

El socket de Docker se monta **en solo lectura** y **solo en este contenedor** (`compose.yml`). Aun en solo lectura, el acceso a la API de Docker es amplio: es una herramienta de observabilidad para administración en el prototipo, **no una configuración para producción**. El contenedor corre como usuario no root con el grupo del socket (`DOCKER_GID`). La API de management de RabbitMQ se consulta con el usuario de la aplicación (RabbitMQ no permite crear un usuario adicional sin un script de definiciones); en PostgreSQL sí se usa un usuario de mínimo privilegio (`pg_monitor`).

## Pruebas

```bash
go test ./...
```
