# Escenario 2 · Exposición directa de los servicios internos — Reverse Proxy

Responsable: Santiago Zamora

## Escenario

| Parte | Descripción |
|---|---|
| Fuente | Un atacante externo en Internet que escanea los puertos del host donde corre JokerArena. |
| Estímulo | Intenta conectarse directamente al API Gateway (`8080`), al cliente web (`3000`) o a cualquier microservicio, y recolecta información de versiones (cabecera `Server`) para buscar vulnerabilidades conocidas. |
| Artefacto | Punto de entrada del sistema: API Gateway, cliente web y microservicios. |
| Entorno | Operación normal, sistema desplegado con `docker compose`. |
| Respuesta | Un único Reverse Proxy (`jokerarena-reverse-proxy`) es el único contenedor con puertos publicados (`80` y `443`). Reenvía `/api`, `/ws`, `/admin/*` y `/health` al API Gateway y el resto al cliente web por la red `edge-net`. Oculta la versión de nginx, agrega cabeceras de seguridad, limita el tamaño y el tiempo de las peticiones y entrega al gateway la IP real del cliente. |
| Medida de la respuesta | Solo `80` y `443` aparecen abiertos en el host; el 100 % de las conexiones directas a `8080`, `3000` o a los puertos de los microservicios fallan; todas las rutas funcionales siguen respondiendo a través de `https://localhost`. |

## Tácticas aplicadas

- **Limitar la exposición (Resistir ataques):** se retiraron los puertos publicados del API Gateway (`8080`). El Reverse Proxy es el único servicio con `ports:` en `docker-compose.p2.yml` y solo está en `edge-net`. Los microservicios están en `app-net` (`internal: true`), así que no tiene acceso directo a ellos.
- **Limitar el acceso (Resistir ataques):** solo se publican las rutas definidas en `edge/reverse-proxy/locations.conf`. El cuerpo de las peticiones se limita a 64 KB (`client_max_body_size`), con tiempos máximos de lectura y envío para mitigar peticiones lentas.
- **Ocultar información (Resistir ataques):** `server_tokens off` quita la versión de nginx. Se agregan `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` y `Permissions-Policy` en todas las respuestas, junto con el HSTS del escenario 1.
- **Mantener la trazabilidad:** el proxy envía `X-Forwarded-For`, `X-Real-IP` y `X-Forwarded-Proto`, y el gateway recupera la IP real (`set_real_ip_from` + `real_ip_header`). Sin esto, el rate limit del login (escenario 4) contaría a todos los clientes como una sola IP.

## Patrón aplicado

**Reverse Proxy** en el componente de borde `jokerarena-reverse-proxy` (nginx 1.27, `edge/reverse-proxy/`). Es el único intermediario entre los clientes externos (navegador y CLI) y el sistema. Separa la responsabilidad del borde (exponer, cifrar, filtrar y endurecer) de la del API Gateway (enrutar, autenticar y limitar por ruta). El mismo componente aloja la terminación TLS del escenario 1 (Secure Channel): `conf.d/10-tls.conf` redirige `80 → 443` y cifra HTTPS y WSS.

```
Internet ──► :80/:443  jokerarena-reverse-proxy  [edge-net]
                        ├─ /api, /ws, /admin/*, /health ──► jokerarena-gateway:8080 [edge-net + app-net] ──► microservicios [app-net]
                        └─ /                             ──► jokerarena-web-ssr:3000 (repo joker-arena-frontend) [edge-net]
```

## Implementación

- **Archivos creados o modificados:**
  - `edge/reverse-proxy/nginx.conf` *(creado)*: configuración global (resolver de Docker, logs, límites, `server_tokens off`) e `include conf.d/*.conf`.
  - `edge/reverse-proxy/locations.conf` *(creado)*: tabla de rutas hacia el gateway y el cliente web. Los upstream van en variables, así que nginx arranca aunque el frontend todavía no esté levantado.
  - `edge/reverse-proxy/snippets/proxy-headers.conf` y `snippets/security-headers.conf` *(creados)*.
  - `edge/reverse-proxy/Dockerfile` y `edge/reverse-proxy/compose.yml` *(creados)*: imagen `nginx:1.27-alpine` con healthcheck; puertos `80/443`, red `edge-net` y certificados de `edge/certs/` montados en solo lectura.
  - `edge/reverse-proxy/conf.d/10-tls.conf` *(modificado, del escenario 1)*: agrega el healthcheck interno en el puerto 80 y las cabeceras de seguridad en el server 443.
  - `docker-compose.p2.yml` *(modificado)*: incluye `edge/reverse-proxy/compose.yml`.
  - `gateway/compose.yml` *(modificado)*: sin puertos publicados; para depurar con `8080` se usa `docker-compose.debug.yml`.
  - `gateway/nginx.conf` *(modificado)*: `set_real_ip_from` para las redes internas.
  - `.gitignore` *(modificado)*: excluye `edge/certs/`, que contiene la llave privada de la CA.

## Evidencia

### 1. Validación de configuración

```text
$ docker compose config -q            # OK
Servicios con puertos publicados: jokerarena-reverse-proxy ['80', '443']
Redes: reverse-proxy [edge-net] · gateway [app-net, edge-net]

$ nginx -t
nginx: configuration file /etc/nginx/nginx.conf test is successful

$ curl -s -o /dev/null -w "%{http_code} %{redirect_url}" http://127.0.0.1/api/rooms
301 https://127.0.0.1/api/rooms

$ curl -sk -D - -o /dev/null https://127.0.0.1/api/leaderboard/top
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: SAMEORIGIN
Referrer-Policy: strict-origin-when-cross-origin
```

### 2. Verificación integrada (sistema completo)

```bash
bash scripts/gen_certs.sh
docker compose up -d --build

# Solo 80 y 443 publicados
docker ps --format '{{.Names}}\t{{.Ports}}' | grep -- '->'

# Acceso directo bloqueado
curl -m 3 http://localhost:8080/health      # falla: connection refused
curl -m 3 http://localhost:3000             # falla: connection refused

# Todo funciona por el proxy
curl --cacert edge/certs/jokerarena-ca.crt https://localhost/health
curl --cacert edge/certs/jokerarena-ca.crt https://localhost/api/leaderboard/top
curl -sI --cacert edge/certs/jokerarena-ca.crt https://localhost/ | grep -i server   # sin versión
```

_[Captura: `docker ps` con solo 80/443]_ · _[Captura: `curl` directo a 8080 rechazado]_ · _[Captura: juego funcionando en https://localhost]_

## Resultado obtenido

El sistema tiene un único punto de entrada público. Los puertos del gateway, del cliente web y de los microservicios ya no son alcanzables desde fuera del host, y toda la funcionalidad sigue disponible por `https://localhost` a través del Reverse Proxy.

## Limitaciones

- El cliente web vive en otro repositorio (`joker-arena-frontend`) y debe unirse a `edge-net` con el nombre `jokerarena-web-ssr`. Si no está levantado, `/` responde `502` y el resto de rutas sigue funcionando.
- Se publica la imagen `nginx:1.27-alpine`; no se agregó un WAF ni filtrado por contenido.
