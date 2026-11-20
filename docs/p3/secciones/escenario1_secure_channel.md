# Escenario 1 · Interceptación del tráfico — Secure Channel

Responsable: Ángel David Ruiz Barbosa

## Escenario

| Parte | Descripción |
|---|---|
| Fuente | Un atacante conectado a la misma red que el jugador (por ejemplo, un wifi público). |
| Estímulo | Captura el tráfico entre el cliente y el servidor para leer la contraseña de `admin` o los tokens JWT y reutilizarlos. |
| Artefacto | Canal de comunicación entre los clientes (navegador y CLI) y el sistema. |
| Entorno | Operación normal. |
| Respuesta | Todo el tráfico externo viaja cifrado con TLS (HTTPS y WSS). Las peticiones por HTTP se redirigen a HTTPS y el navegador recibe HSTS. |
| Medida de la respuesta | En una captura de tráfico no se puede leer ninguna credencial ni token; el 100 % de las peticiones al puerto 80 recibe 301 hacia HTTPS. |

## Tácticas aplicadas

- **Cifrar los datos en tránsito (Resistir ataques):** Se configuró la terminación TLS (`TLSv1.2` y `TLSv1.3`) en el puerto `443` del `Reverse Proxy` (`edge/reverse-proxy/conf.d/10-tls.conf`) utilizando certificados X.509 v3 emitidos por la autoridad certificadora local generada por `scripts/gen_certs.sh`. Tanto el `Front-end web` como el `Cliente de terminal (CLI)` operan sobre `https://` y `wss://`, y la CLI verifica estrictamente la cadena de confianza contra la CA local (`edge/certs/jokerarena-ca.crt` o `JOKERARENA_CA`).
- **Prevenir degradación de protocolo / forzar canal seguro (Resistir ataques):** Todo el tráfico entrante por el puerto `80` recibe una redirección permanente `301` hacia `https://$host$request_uri` y las respuestas HTTPS incluyen la cabecera `Strict-Transport-Security: max-age=31536000; includeSubDomains` (HSTS) con la directiva `always`.

## Patrón aplicado

**Secure Channel** aplicado en el componente de borde **`Reverse Proxy`** (`jokerarena-reverse-proxy`, configuración `edge/reverse-proxy/conf.d/10-tls.conf`), en coordinación con los dos clientes externos del sistema: **`Front-end web`** (`clients/web-ssr`) y **`Cliente de terminal (CLI)`** (`clients/cli`).

El patrón encapsula todo el intercambio de credenciales (contraseña de `admin`), tokens JWT (`Authorization: Bearer ...` en REST y `?token=...` en WebSocket) y eventos de juego/monitoreo dentro de un túnel TLS autenticado entre los clientes y el punto único de entrada del sistema.

## Implementación

- **Archivos creados o modificados:**
  - `scripts/gen_certs.sh` *(creado, ejecutable)*
  - `edge/reverse-proxy/conf.d/10-tls.conf` *(creado)*
  - `clients/cli/jokerarena_cli/api.py` *(modificado)*
  - `clients/cli/jokerarena_cli/main.py` *(modificado)*
  - `clients/cli/tests/test_cli.py` *(modificado)*
  - `clients/cli/README.md` *(modificado)*
  - `clients/web-ssr/src/ws/gameSocket.ts` *(modificado)*
  - `clients/web-ssr/src/views/Admin.tsx` *(modificado)*
  - `clients/web-ssr/src/test/gameSocket.test.ts` *(modificado)*
  - `clients/web-ssr/src/test/admin.test.ts` *(modificado)*

- **Qué hace cada cambio:**
  - **`scripts/gen_certs.sh`**: Script idempotente (soporta `--force`, `CERTS_DIR` y `EXTRA_SANS`) que genera con OpenSSL una CA raíz local (`jokerarena-ca.key` RSA 4096 con permiso `600`, `jokerarena-ca.crt` válida 3650 días) y el par de llaves del servidor (`server.key` RSA 2048 con permiso `644` para lectura desde nginx no-root montado `:ro` y `server.crt` válido 365 días con SAN `DNS:localhost`, `DNS:jokerarena-reverse-proxy`, `IP:127.0.0.1`, `IP:::1` y `extendedKeyUsage=serverAuth`), verificando la cadena al finalizar con `openssl verify`. Los artefactos generados se ubican en `edge/certs/` (directorio excluido del control de versiones).
  - **`edge/reverse-proxy/conf.d/10-tls.conf`**: Define dos bloques `server` para el Reverse Proxy:
    1. Puerto `80`: responde `301 https://$host$request_uri` para toda petición HTTP.
    2. Puerto `443 ssl` con `http2 on;`: monta `/etc/nginx/certs/server.crt` y `/etc/nginx/certs/server.key`, restringe protocolos a `TLSv1.2 TLSv1.3`, deshabilita `ssl_session_tickets`, agrega `Strict-Transport-Security "max-age=31536000; includeSubDomains" always` e incluye `/etc/nginx/locations.conf` para enrutar HTTPS y WSS hacia `Front-end web` y `API Gateway`.
  - **`clients/cli/jokerarena_cli/api.py`, `main.py` y `README.md`**: Cambia `DEFAULT_GATEWAY` de `http://localhost:8080` a `https://localhost`. Implementa `ca_bundle_path()`, `build_ssl_context()`, `ws_url()` (`https://` → `wss://`) y `ws_ssl_context()` (pasando `ssl=SSLContext` solo para `wss://`). Detecta errores `ssl.SSLError` / `CERTIFICATE_VERIFY_FAILED` para mostrar instrucciones claras de generación/configuración de la CA (`scripts/gen_certs.sh` y `JOKERARENA_CA`). Actualiza el comando `rooms create` para imprimir `{gateway_url()}/room/{code}` en lugar de `:3000`.
  - **`clients/web-ssr/src/ws/gameSocket.ts` y `src/views/Admin.tsx`**: Elimina el puerto fijo `:8080` en `buildWsUrl()` y `wsAdminUrl()`, construyendo URLs WebSocket contra el mismo origen del navegador (`loc.host`, cambiando automáticamente `https:` → `wss:` y `http:` → `ws:`), de modo que tanto las salas de juego (`/ws/{code}`) como el radar administrativo (`/admin/cluster/metrics`) viajen cifrados por el Reverse Proxy.
  - **`clients/cli/tests/test_cli.py`, `clients/web-ssr/src/test/gameSocket.test.ts` y `admin.test.ts`**: Pruebas unitarias actualizadas y ampliadas para verificar URLs `https://`/`wss://`, resolución de `JOKERARENA_CA`, rechazo ante fallo de verificación TLS y construcción de URLs mismo origen sin `:8080`.

## Evidencia

### 1. Generación y validación criptográfica de certificados (`scripts/gen_certs.sh`)

```text
$ bash scripts/gen_certs.sh
Generando CA local en edge/certs/jokerarena-ca.crt ...
Generando certificado TLS del servidor en edge/certs/server.crt ...
Certificate request self-signature ok
subject=C=CO, O=JokerArena, OU=Equipo 2b, CN=localhost
Verificando cadena de confianza...
edge/certs/server.crt: OK

$ openssl x509 -in edge/certs/server.crt -noout -subject -issuer -dates -ext subjectAltName,extendedKeyUsage
subject=C=CO, O=JokerArena, OU=Equipo 2b, CN=localhost
issuer=C=CO, O=JokerArena, OU=Equipo 2b, CN=JokerArena Local Root CA
notBefore=Nov 20 17:37:05 2026 GMT
notAfter=Nov 20 17:37:05 2027 GMT
X509v3 Extended Key Usage: 
    TLS Web Server Authentication
X509v3 Subject Alternative Name: 
    DNS:localhost, DNS:jokerarena-reverse-proxy, IP Address:127.0.0.1, IP Address:0:0:0:0:0:0:0:1

$ openssl verify -CAfile edge/certs/jokerarena-ca.crt edge/certs/server.crt
edge/certs/server.crt: OK

$ bash scripts/gen_certs.sh
Los certificados ya existen en edge/certs (usa --force para regenerarlos).
edge/certs/server.crt: OK
```

### 2. Verificación HTTPS y WSS con captura TCP de tráfico en loopback

Se levantó un servidor HTTPS y un servidor WSS locales usando `edge/certs/server.crt` y `edge/certs/server.key` detrás de un proxy TCP que capturó todos los bytes crudos transmitidos por el cable, realizando peticiones reales desde `jokerarena_cli.api.Gateway` y `websockets.connect` con `ws_url` + `ws_ssl_context`:

```text
HTTPS_OK tls=TLSv1.3 rooms=1 wire_bytes=4106 pass_in_wire=0 token_in_wire=0
TLS_UNTRUSTED_REJECTED_OK msg=No se pudo verificar el certificado TLS de https://localhost:56196. Genera la CA con 'bash scripts/gen_certs.sh' o indica su ruta en JOKERARENA_CA.
WSS_OK url_scheme=wss msg_type=HAND_SCORED wire_bytes=3762 ws_token_in_wire=0 payload_in_wire=0
```

- En `4106` bytes TCP capturados durante el login y consulta de salas sobre `TLSv1.3`, la contraseña y el JWT aparecieron `0` veces en texto plano.
- Al intentar conectar sin confiar en `jokerarena-ca.crt` (`verify=True` contra el almacén del sistema), la CLI rechazó el certificado no confiable e indicó el procedimiento para confiar en la CA local.
- En `3762` bytes TCP capturados durante la conexión WebSocket (`wss://`), ni el token en `?token=` ni el mensaje `HAND_SCORED` aparecieron en texto plano.

### 3. Pruebas unitarias y verificación de tipos

```text
$ cd clients/cli && python -m pytest -v
...
10 passed in 2.08s

$ cd clients/web-ssr && npm run typecheck && npm test
> tsc --noEmit
> vitest run
 Test Files  6 passed (6)
      Tests  60 passed (60)
```

### 4. Comandos de verificación integrada (post-integración con Reverse Proxy y Docker)

Una vez integrado el PR de Esteban (`edge/reverse-proxy/`) y levantado el clúster con `docker compose up -d --build --wait`:

```bash
# 1. Verificar redirección 301 desde el puerto 80 hacia HTTPS
curl -i http://localhost/health

# 2. Verificar terminación TLS, respuesta HTTPS 200 y cabecera HSTS
curl -i --cacert edge/certs/jokerarena-ca.crt https://localhost/health

# 3. Probar la CLI contra el borde HTTPS/WSS
JOKERARENA_CA=edge/certs/jokerarena-ca.crt jokerarena login admin --password <password>
JOKERARENA_CA=edge/certs/jokerarena-ca.crt jokerarena rooms create

# 4. Captura de tráfico en interfaz loopback/docker (tshark/Wireshark) durante login + WebSocket:
#    confirmar TLSv1.2/TLSv1.3 y 0 credenciales o JWT legibles en el flujo TCP
```

## Resultado obtenido

Se cumplió el objetivo de cifrado y verificación estricta en los artefactos implementados:
- La cadena de certificados local se genera de forma reproducible e idempotente (`openssl verify` devuelve `OK`).
- Las pruebas reales sobre loopback validaron negociación **`TLSv1.3`** tanto en `HTTPS` como en `WSS`, **`0` credenciales/tokens expuestos en los bytes TCP capturados**, y rechazo automático cuando falta la CA de confianza.
- Las suites unitarias y de tipos de ambos clientes pasaron al 100 % (**10/10** en `pytest` para la CLI y **60/60** en `vitest` + `tsc --noEmit` sin errores para `web-ssr`).
- La comprobación en vivo de la redirección `301` del puerto `80` y de la cabecera `Strict-Transport-Security` en `jokerarena-reverse-proxy` queda lista en `edge/reverse-proxy/conf.d/10-tls.conf` para ejecutarse inmediatamente al integrar el contenedor del Reverse Proxy.

## Limitaciones o cambios respecto al plan

1. **Convención de rutas en `10-tls.conf` (contrato P3 pendiente):** Al momento de implementar esta rama aún no se había publicado `docs/p3/contrato_p3.md` en el repositorio. Se adoptaron las rutas estándar `/etc/nginx/certs/server.crt`, `/etc/nginx/certs/server.key` y `include /etc/nginx/locations.conf;`. Al integrar con el PR de Esteban (`edge/reverse-proxy/`), debe confirmarse que el montaje en `edge/reverse-proxy/compose.yml` y la ubicación de `locations.conf` coincidan exactamente con estos nombres.
2. **Eliminación de `edge/reverse-proxy/conf.d/00-http.conf`:** Según el orden de integración del equipo, `00-http.conf` será creado por Esteban para pruebas sin TLS y debe eliminarse cuando esta rama se integre después de la suya (en esta rama `00-http.conf` todavía no existe en el repositorio).
3. **Herencia de `add_header` en nginx:** En nginx, declarar `add_header Strict-Transport-Security ...` dentro del bloque `server` de `10-tls.conf` reemplaza cualquier `add_header` heredado del bloque `http {}`. Al integrar con `edge/reverse-proxy/nginx.conf`, se debe verificar con `curl -i --cacert edge/certs/jokerarena-ca.crt https://localhost/health` que coexistan HSTS y las cabeceras de seguridad del Escenario 2 (o mover las cabeceras compartidas a un snippet incluido en `server {}`).
4. **Pruebas integradas con Docker y Wireshark pendientes:** Docker Desktop no se encontraba activo ni estaba aún desplegado el contenedor `jokerarena-reverse-proxy` de Esteban, por lo que la evidencia de captura de tráfico de esta versión se obtuvo mediante proxy TCP en loopback contra servidores TLS/WSS reales usando los certificados generados por `scripts/gen_certs.sh`. La captura final en Wireshark sobre el clúster completo se ejecutará el domingo tras unir los PR de borde.
5. **Protección de la rama `p3` (Tarea A6):** Se activó el ruleset de protección (exigir PR con 1 aprobación, impedir borrado y force-push) sobre `main`. Para `p3` se dejó creado el ruleset equivalente en estado `disabled` (ID `27746663`) para no impedir que Nicole ejecute la Tarea 0 (`A1–A3`) sobre `p3`; deberá habilitarse apenas exista `p3` con la Tarea 0 completada.
