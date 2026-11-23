# Escenario 4 · Account Lockout + Rate Limiting — Bloqueo de cuenta por intentos fallidos con estado compartido en Redis y limitación de tasa en el API Gateway

**Responsable:** Esteban Andrés Gómez Álvarez (`auth-identity-ms`) · **cliente web:** Jose Arturo Herrera Rivera (`joker-arena-frontend`)

## Escenario

| Parte | Descripción |
|---|---|
| **Fuente** | Un jugador (o el administrador) que se autentica desde el cliente web SSR (`/`), la CLI o directamente contra el API Gateway; incluye a un atacante que prueba contraseñas sobre una cuenta concreta (fuerza bruta / *credential stuffing*). |
| **Estímulo** | Ráfaga de intentos de autenticación con **contraseña incorrecta** sobre el mismo usuario: `POST /api/auth/login`, o `POST /api/auth/register` cuando el usuario es `admin` (el registro también pide la contraseña de administración). En paralelo, una ráfaga de peticiones a `/api/auth/*` desde una misma IP. |
| **Artefacto** | `gateway` (nginx, todas las rutas `/api/auth/*`), `auth-identity-ms` (`/auth/login`, `/auth/register`, emisión de JWT), `jokerarena-redis` (contador de fallos y bloqueo), `jokerarena-postgres` (`auth.players`, hash PBKDF2 de la contraseña) y el cliente web SSR. |
| **Entorno** | Despliegue del Prototipo 2 con Docker Compose (`docker-compose.p2.yml`): 14 contenedores sanos + cliente web SSR; tráfico normal; la cuenta objetivo está registrada con contraseña. |
| **Respuesta** | Tras **5 fallos de credenciales** (configurable) la cuenta queda **bloqueada temporalmente**: durante el bloqueo, `login` y `register` responden `423 account_locked` con la cabecera `Retry-After` y un mensaje con los minutos restantes, **incluso si la contraseña es correcta**. El gateway rechaza además las ráfagas que superan **10 peticiones/s por IP** en `/api/auth/*`, el contador se reinicia al autenticarse correctamente y el token del rol `admin` pasa a durar **30 minutos**. |
| **Medida de la respuesta** | Admitir como máximo **5** intentos fallidos por cuenta; bloqueo efectivo durante el tiempo configurado (**15 min** por defecto, `AUTH_LOCKOUT_MINUTES`); **0** autenticaciones exitosas mientras dure el bloqueo; contador reiniciado tras un login correcto; rechazo del gateway ante ráfagas > 10 req/s; latencia del rechazo < 100 ms; y ningún efecto sobre otras cuentas (aislamiento por usuario). |

## Tácticas aplicadas

- **Limitar el acceso (*Limit access*)** — rate limiting por IP en el **API Gateway**: `limit_req_zone $binary_remote_addr zone=auth_rl:10m rate=10r/s` aplicado con `limit_req zone=auth_rl burst=20 nodelay` a `^/api/auth/(register|login|me|logout)$` (`gateway/nginx.conf`).
- **Detectar ataques / limitar el acceso por identidad** — cada fallo de credenciales se cuenta **por usuario** (no por IP) en Redis: `auth:fail:<usuario>`, con TTL igual a la ventana de bloqueo.
- **Reaccionar a los ataques: bloquear la cuenta (*Lock computer* / *Revoke access*)** — al llegar a `AUTH_MAX_FAILED_ATTEMPTS` se crea `auth:lock:<usuario>` con TTL `AUTH_LOCKOUT_MINUTES`; mientras exista, los dos endpoints de credenciales lanzan `423` sin consultar la base de datos.
- **Limitar la exposición (*Limit exposure*)** — el JWT del rol `admin` dura **30 min** (`AUTH_ADMIN_TOKEN_TTL_SECONDS=1800`) en lugar de las 12 h del jugador; al expirar hay que volver a autenticarse.
- **Informar al actor (*Inform actors*)** — el error incluye cuántos intentos quedan o cuántos minutos faltan, y viaja con `Retry-After` para que cualquier cliente (web, CLI) pueda esperar; el cliente web muestra un aviso destacado y desactiva el envío.
- **Separar entidades con estado compartido** — el estado de intentos no vive en memoria del proceso: está en Redis, por lo que el comportamiento es el mismo con varias réplicas y sobrevive a reinicios del servicio de identidad.
- **Auditoría (*Audit*)** — `log.warning("cuenta %s bloqueada: %d intentos fallidos")` en el servicio y traza del gateway con el `423` en el *access log* (formato `gw`).

## Patrón aplicado

- **Account Lockout (Login Throttling) con estado compartido** en **`auth-identity-ms`**: contador y bloqueo en **Redis** (`auth:fail:<usuario>`, `auth:lock:<usuario>`), verificado antes de comprobar credenciales y alimentado solo con fallos reales de contraseña.
- **Rate Limiter (token bucket)** en el **API Gateway** (`gateway/nginx.conf`, zona `auth_rl`): primera línea de defensa perimetral, independiente del usuario y del servicio.
- **Credenciales de vida corta** para el rol `admin` (JWT de 30 min) emitidas por `auth-identity-ms`.
- **Retroalimentación inmediata en el cliente** (`joker-arena-frontend`): el `423` se distingue de un `401` y se bloquea el botón de envío mientras la cuenta esté bloqueada.

## Implementación

**Backend (`joker-arena-backend`)**

- `services/auth-identity-ms/app/config.py` — *(modificado)* nuevos ajustes: `max_failed_attempts` (`AUTH_MAX_FAILED_ATTEMPTS`, 5), `lockout_minutes` (`AUTH_LOCKOUT_MINUTES`, 15) y `admin_token_ttl_seconds` (`AUTH_ADMIN_TOKEN_TTL_SECONDS`, 1800) + propiedad `lockout_seconds`.
- `services/auth-identity-ms/app/store.py` — *(modificado)* protocolo `LoginThrottle` y clase `RedisLoginThrottle`: `remaining_lock()` (TTL del bloqueo), `register_failure()` (`INCR` + `EXPIRE` del contador, crea `auth:lock:<usuario>` al llegar al máximo y devuelve los intentos restantes) y `clear()`.
- `services/auth-identity-ms/app/main.py` — *(modificado)* `ApiError` admite `retry_after` y el *exception handler* añade la cabecera `Retry-After`; `issue()` elige el TTL según el rol (`admin` → 30 min, `player` → 12 h) y la sesión en Redis usa ese mismo TTL; helpers `lock_message()`, `ensure_not_locked()` (responde `423` sin tocar la BD), `fail_credentials()` (cuenta el fallo y devuelve «Te quedan N intentos» / «423») y `clear_failures()`; `/auth/login` y `/auth/register` llaman a esos helpers antes y después de validar credenciales.
- `services/auth-identity-ms/tests/test_auth.py` — *(modificado)* `FakeThrottle` en memoria y 6 pruebas: bloqueo a los 5 fallos (15 min, `Retry-After: 900`), fallos de la contraseña de admin en `/register` que también cuentan, reinicio del contador al acertar, configurabilidad (2 fallos / 1 minuto → `Retry-After: 60`), TTL del token admin vs jugador, y funcionamiento con Redis caído.
- `services/auth-identity-ms/compose.yml` — *(modificado)* variables con valores por defecto `5` / `15` / `1800`.
- `services/auth-identity-ms/README.md` — *(modificado)* sección «Bloqueo por intentos fallidos» con el detalle, **dónde cambiar el tiempo** y el desbloqueo manual.
- `.env.example` — *(modificado)* las tres variables documentadas (la duración se cambia en `AUTH_LOCKOUT_MINUTES`).
- `contracts/http-routes.md` — *(modificado)* contrato de `/api/auth/*`: `423 account_locked`, `Retry-After`, conteo en login y registro y vigencia del token por rol.
- `scripts/verify_p2.sh` — *(modificado)* verificación automática del escenario tras desplegar: registra un usuario, provoca 5 fallos y exige `423` + `Retry-After`.

**Cliente web (`joker-arena-frontend`)**

- `src/api/client.ts` — *(modificado)* `ApiError` expone `retryAfter` leído de la cabecera `Retry-After`.
- `src/views/Welcome.tsx` — *(modificado)* si llega `423 account_locked` se muestra un aviso destacado en ámbar con 🔒, el mensaje del servidor (minutos restantes), `aria-live="assertive"` y el **botón de envío deshabilitado** mientras dure el bloqueo.
- `src/test/welcome.test.tsx` — *(modificado)* prueba «muestra el mensaje de bloqueo y desactiva el envío».
- `README.md` — *(modificado)* fila de problemas comunes con el mensaje de bloqueo y el comando para desbloquear.

**Sin cambios de esquema**: el estado del bloqueo vive en Redis (`auth:fail:*`, `auth:lock:*` con TTL), no en PostgreSQL.
**Sin cambios en el gateway**: el rate limit por IP (`limit_req_zone`/`limit_req` en `gateway/nginx.conf`) ya existía; en este escenario se reutiliza como primera capa y se verifica.


## Evidencia

**E1 · Pruebas automatizadas del servicio de identidad**

```bash
docker run --rm -v "$PWD/services/auth-identity-ms:/src" -w /src python:3.12-slim \
  sh -c "pip install -q -r requirements-dev.txt && python -m pytest tests/test_auth.py -v"
```

```text
tests/test_auth.py::test_five_failed_logins_lock_the_account_for_15_minutes PASSED
tests/test_auth.py::test_failed_admin_password_in_register_also_counts PASSED
tests/test_auth.py::test_successful_login_clears_the_failed_attempts PASSED
tests/test_auth.py::test_attempts_and_lock_time_are_configurable PASSED
tests/test_auth.py::test_admin_token_lasts_30_minutes_and_player_token_12_hours PASSED
tests/test_auth.py::test_login_still_works_when_redis_is_down PASSED
======================== 23 passed, 1 warning in 3.92s ========================
# suite completa del servicio (incluye tests/test_passwords.py): 26 passed
```

**E2 · Bloqueo en vivo a través del API Gateway** (entorno de pruebas con `AUTH_LOCKOUT_MINUTES=1`)

```bash
curl -s -o /dev/null -w 'HTTP %{http_code} en %{time_total}s\n' -X POST localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"user":"evid30386","password":"evidencia123","repeat_password":"evidencia123"}'
# luego 5 intentos con contraseña errónea y, por último, la correcta
```

```text
== 1. Alta de la cuenta (registro) ==
HTTP 201 en 0.098529s
== 2. Intentos fallidos de login ==
fallo 1: HTTP 401 en 0.054737s -> {"error":"invalid_credentials","message":"Usuario o contraseña incorrectos. Te quedan 4 intentos."}
fallo 2: HTTP 401 en 0.052412s -> {"error":"invalid_credentials","message":"Usuario o contraseña incorrectos. Te quedan 3 intentos."}
fallo 3: HTTP 401 en 0.052183s -> {"error":"invalid_credentials","message":"Usuario o contraseña incorrectos. Te quedan 2 intentos."}
fallo 4: HTTP 401 en 0.053751s -> {"error":"invalid_credentials","message":"Usuario o contraseña incorrectos. Te queda 1 intento."}
fallo 5: HTTP 423 en 0.053939s -> {"error":"account_locked","message":"Cuenta bloqueada por demasiados intentos fallidos. Inténtalo de nuevo en 1 minuto."}
== 3. Login con la contraseña correcta estando bloqueada ==
HTTP 423 en 0.002733s
{"error":"account_locked","message":"Cuenta bloqueada por demasiados intentos fallidos. Inténtalo de nuevo en 1 minuto."}
retry-after: 60
```

**E3 · Estado del bloqueo en Redis**

```bash
docker exec jokerarena-redis redis-cli --scan --pattern "auth:*:evid309"
docker exec jokerarena-redis redis-cli ttl auth:lock:evid309
```

```text
auth:lock:evid309
59
```

**E4 · Rate limiting del gateway (30 peticiones seguidas a `/api/auth/me`)**

```text
22 401 8 503        # 8 peticiones rechazadas por limit_req (ráfaga > 10 r/s por IP)
# 2 s después, el servicio responde normal:
HTTP 401 en 0.001711s
```

**E5 · Bloqueo a través del cliente web SSR** (con la configuración por defecto de 15 minutos)

```text
registro=201
fallo 1..4 -> 401 ... "Te quedan 4/3/2 intentos." / "Te queda 1 intento."
fallo 5 (via Next) -> 423 {"error":"account_locked","message":"Cuenta bloqueada por demasiados intentos fallidos. Inténtalo de nuevo en 15 minutos."} [retry-after: 900]
home: formulario de acceso presente
```

```bash
cd ../joker-arena-frontend && npm run typecheck && npm test
```

```text
tsc --noEmit                                    # sin errores
Test Files  7 passed (7)
     Tests  70 passed (70)                      # incluye «muestra el mensaje de bloqueo y desactiva el envío»
```

**E6 · Verificación del despliegue completo**

```bash
bash scripts/verify_p2.sh
```

```text
  ✔ 5 intentos fallidos → 423 account_locked (Retry-After presente)
  ✔ 14 contenedores (8 lógica + gateway + 4 datos + broker)
  ✔ /leaderboard renderizado en el servidor
✅ Todo en orden.                               # exit code 0
```

## Resultado obtenido

| Medida de la respuesta | Resultado observado | ¿Cumple? |
|---|---|---|
| Máximo 5 intentos fallidos admitidos | 4 respuestas `401` con el contador restante y `423` en el 5.º | ✅ |
| Bloqueo durante el tiempo configurado | Con `AUTH_LOCKOUT_MINUTES=15` → `Retry-After: 900`; con `=1` → `Retry-After: 60` y TTL 59 s en Redis | ✅ |
| Cero accesos durante el bloqueo | Contraseña correcta estando bloqueada → `423 account_locked` | ✅ |
| El contador se reinicia al autenticarse | `test_successful_login_clears_the_failed_attempts`: 4 fallos + login correcto → 4 fallos nuevos no bloquean | ✅ |
| Gateway rechaza ráfagas > 10 r/s | 30 peticiones en ráfaga → **22×401 y 8×503**; se recupera a los 2 s | ✅ |
| Latencia del rechazo < 100 ms | `423` por cuenta bloqueada en **2.7 ms**; `401` con verificación PBKDF2 en **52–55 ms**; registro en **98 ms** | ✅ |
| Token de admin de 30 min | `expires_in = 1800` y `exp − iat = 1800` (jugador: `43200` = 12 h) | ✅ |
| Sin afectar a otras cuentas | El resto de cuentas (p. ej. `aanuel`) siguió autenticándose durante las pruebas; el bloqueo es por usuario | ✅ |
| Fallos del registro cuentan | `test_failed_admin_password_in_register_also_counts`: 4 fallos en `/register` + el 5.º → `423`, y después ni con `ADMIN_PASSWORD` entra | ✅ |
| Resistencia a fallos de Redis | Con Redis caído el login sigue funcionando (se pierde el bloqueo y se registra un warning) | ✅ |


## Limitaciones o cambios respecto al plan

1. **El entorno de pruebas quedó con `AUTH_LOCKOUT_MINUTES=1`** en `.env` (para ver el bloqueo sin esperar 15 min). El valor por defecto del servicio es **15**; para la entrega basta con volver a `AUTH_LOCKOUT_MINUTES=15` y reiniciar el contenedor (`docker compose -f docker-compose.p2.yml up -d jokerarena-auth-identity-ms`).
2. **Política *fail-open* si Redis no responde**: se decidió no tumbar la autenticación por la caída de un servicio auxiliar (solo se registra un `warning`). Si se exigiera *fail-closed*, habría que rechazar el login cuando Redis esté caído.
3. **El `409 alias_taken` y el `404 player_not_found` no cuentan** como fallo: de lo contrario, cualquiera podría bloquear cuentas ajenas enviando registros con su nombre. Como contrapartida, un atacante puede deducir si un usuario existe (`404` vs `401`); eso se mitiga con el rate limit del gateway, no con el bloqueo.
4. **No hay desbloqueo desde la consola de administración ni notificación al usuario** (correo, CAPTCHA): el desbloqueo operativo es
   `docker exec jokerarena-redis redis-cli del auth:lock:<usuario> auth:fail:<usuario>`.
5. **El rate limit es por IP**, no por usuario (nginx `limit_req_zone`): varios jugadores detrás del mismo NAT comparten cuota. Si la demo genera ráfagas legítimas, se ajustan `rate`/`burst` en `gateway/nginx.conf`.
6. **Evidencia en vivo del token de admin**: no se pudo capturar contra el despliegue porque la base actual solo tiene el usuario `aanuel` (el alias `admin` no existe tras recrear la BD); queda cubierta por la prueba unitaria `test_admin_token_lasts_30_minutes_and_player_token_12_hours` (`expires_in = 1800`, `exp − iat = 1800`).
7. **Redondeo y redacción del mensaje**: los segundos restantes se redondean hacia arriba a minutos (1–59 s → «1 minuto») y se usa singular/plural («Te queda 1 intento» / «Te quedan N intentos»), ajustado después del primer borrador.
8. **El estado del bloqueo no se consulta desde la consola de administración** (no hay endpoint de «intentos restantes»): se ve en Redis o en los logs del servicio.

