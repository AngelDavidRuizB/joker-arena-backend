# Escenario 5 · Cambios no autorizados al balance del juego — Zero Trust

Responsable: Jaisson Andrés Machado Bautista

## Escenario

| Parte | Descripción |
|---|---|
| Fuente | Un contenedor comprometido dentro de la red interna de Docker (por ejemplo, un servicio con una dependencia vulnerable) o un jugador autenticado sin rol `admin`. |
| Estímulo | Envía `PUT /liveops/ruleset` o `POST /liveops/...` directamente a `jokerarena-liveops-rules-ms:8087`, saltándose el API Gateway, sin token, con un token falsificado, alterado, vencido o de jugador, para cambiar el balance de los Jokers. |
| Artefacto | `liveops-rules-ms` (ruleset de balance versionado que el motor aplica en caliente). |
| Entorno | Operación normal; el atacante ya está dentro de la red interna. |
| Respuesta | El servicio valida por su cuenta la firma del JWT (con el JWKS de `auth-identity-ms`), el emisor, el vencimiento, que la sesión no esté cerrada y el rol `admin` antes de aceptar el cambio. Responde `401` o `403`, registra el rechazo en el log sin escribir el token y no modifica el ruleset. El `GET` del ruleset sigue abierto, porque el motor lo consulta sin token al arrancar. |
| Medida de la respuesta | 0 cambios aceptados sin un token admin válido; una llamada directa sin token desde otro contenedor recibe `401`; la consola de administración sigue guardando cambios. |

## Tácticas aplicadas

- **Autenticar actores (Resistir ataques):** `liveops-rules-ms` verifica la firma RS256 de cada token contra la llave pública de `auth-identity-ms` (`/.well-known/jwks.json`). Solo auth puede firmar; liveops solo verifica, sin compartir secretos. Además se comprueba el emisor (`iss = jokerarena-auth`), el vencimiento (`exp`, con 30 s de tolerancia de reloj) y que la sesión no se haya cerrado (`auth:revoked:<jti>` en Redis, la misma lista que usa auth en `/auth/logout`).
- **Autorizar actores (Resistir ataques):** solo un token con `role = admin` puede hacer `PUT`/`POST`. Un token válido de jugador recibe `403`.
- **Limitar la exposición (Resistir ataques):** la protección se aplica por método y no por ruta. Toda petición que no sea `GET`, `HEAD` u `OPTIONS` pasa por la verificación, incluidas las rutas que se agreguen en el futuro. Las lecturas quedan abiertas solo porque el motor las necesita.
- **Detectar ataques / auditoría (Detectar ataques):** cada rechazo queda en el log con el motivo (`sin_token`, `firma_invalida`, `token_vencido`, `rol_no_admin`, etc.), el método, la ruta y la IP, sin el token. Cada cambio aceptado registra el alias del administrador tomado del token ya verificado, no de un token sin verificar como en el P2.

## Patrón aplicado

**Zero Trust (autorización en el servicio).** La red interna no es una zona de confianza: estar detrás del gateway o en la misma red de Docker no da permisos. Cada servicio con operaciones sensibles es su propio punto de decisión. El gateway sigue validando el rol admin en `/admin/liveops/*` (`auth_request /_auth_admin`), y `liveops-rules-ms` lo vuelve a validar. Son dos controles independientes (defensa en profundidad).

Decisiones del diseño:

- **Solo RS256.** Se rechazan `alg: none` y `HS*`, para evitar ataques de confusión de algoritmo, y también las llaves del JWKS que no sean RSA de firma.
- **Sin acoplar el arranque.** Las llaves se piden con reintentos (backoff exponencial de 0,5 s a 15 s) y se precargan en segundo plano. `liveops` arranca aunque `auth` no esté listo, sin tocar `compose.yml` ni agregar `depends_on`. Las llaves se guardan 10 minutos y se vuelven a pedir si llega un `kid` desconocido (rotación de llaves).
- **Fail closed.** Si `auth` no responde y no hay llaves, los cambios se rechazan con `503`; nunca se aceptan sin verificar.
- **Revocación tolerante a fallos.** Si Redis no responde al consultar `auth:revoked:<jti>`, se acepta el token ya verificado (firma, emisor, `exp` y rol), igual que hace `auth-identity-ms`, para que una caída de Redis no deje sin consola a los administradores.
- **Sin variables nuevas.** La URL del JWKS (`http://jokerarena-auth-identity-ms:8082/.well-known/jwks.json`), el emisor y el rol van fijos en el código.

## Implementación

- **Archivos creados o modificados** (todos en `services/liveops-rules-ms/`):
  - `src/auth.ts` *(creado)*
  - `src/app.ts` *(modificado)*
  - `src/main.ts` *(modificado)*
  - `src/infra.ts` *(modificado)*
  - `test/zero-trust.test.ts` *(creado)*
  - `test/jwt-helpers.ts` *(creado)*
  - `test/liveops.test.ts` *(modificado)*
  - `README.md` *(modificado)*

- **Qué hace cada cambio:**
  - **`src/auth.ts`**: `JwksClient` (descarga del JWKS con reintentos, caché y recarga por `kid`), `verifyAdminToken` (firma RS256, `iss`, `exp`, `nbf`, `sub` y rol) y `createAdminGuard` (responde `401`, `403` o `503`, consulta la revocación y registra el rechazo sin el token). Usa solo `node:crypto`, sin dependencias nuevas.
  - **`src/app.ts`**: `buildApp` recibe el guard como parámetro obligatorio y lo aplica en un hook `onRequest` a todo método que no sea de lectura. El autor del cambio (`updated_by`) sale del token verificado (`req.admin.alias`).
  - **`src/main.ts`**: crea el `JwksClient`, lo precarga en segundo plano y conecta el guard con la revocación en Redis.
  - **`src/infra.ts`**: `RedisRulesetCache.isRevoked(jti)` consulta `auth:revoked:<jti>`.
  - **Pruebas**: las 17 pruebas existentes ahora envían un token admin firmado con llaves de prueba, y 21 pruebas nuevas cubren el escenario.

## Evidencia

### 1. Pruebas automatizadas (`npm test`)

```text
✓ test/liveops.test.ts (17 tests)
✓ test/zero-trust.test.ts (21 tests)
  ✓ GET del ruleset e historial siguen abiertos (el motor lo consulta sin token)
  ✓ PUT y POST sin token → 401 y el ruleset no cambia
  ✓ token admin válido → 200 y se guarda con el alias del token
  ✓ token firmado con otra llave → 401
  ✓ token vencido → 401
  ✓ token sin exp → 401
  ✓ token de otro emisor → 401
  ✓ token con alg none → 401
  ✓ token con alg HS256 → 401
  ✓ token con kid desconocido → 401
  ✓ token malformado → 401
  ✓ token alterado (rol de jugador cambiado a admin) → 401
  ✓ token sin firma verificable (formato del P2: x.<payload>.y) → 401
  ✓ token válido de jugador (sin rol admin) → 403
  ✓ sesión cerrada en auth (jti revocado en Redis) → 401
  ✓ si Redis falla al consultar revocación, la firma ya validada basta (como en auth)
  ✓ JWKS con reintentos: auth arranca después que liveops y aun así se valida
  ✓ auth caído del todo → 503 (fail closed: no se acepta el cambio)
  ✓ las llaves se cachean: varias peticiones, una sola descarga del JWKS
  ✓ el rechazo queda en el log con el motivo, sin el token
  ✓ verifyAdminToken devuelve los claims del admin
Tests  38 passed (38)
```

`npm run typecheck` y `npm run build` sin errores.

### 2. Interoperabilidad con la firma real de auth

Se levantó el servicio compilado (`dist/`) **antes** que un servidor JWKS que usa la clase real `SigningKey` de `auth-identity-ms/app/keys.py` y firma con PyJWT, igual que `/auth/login`:

```text
GET sin token:         200
PUT sin token:         401
PUT token jugador:     403
PUT token admin:       200
POST happy-hour admin: 200
```

Log del servicio: tres intentos fallidos de cargar el JWKS (auth aún no existía), luego `JWKS de auth cargado`, y los rechazos con `reason: sin_token` y `reason: rol_no_admin`. El token no aparece en ninguna línea del log.

### 3. Comandos de verificación integrada (sistema completo en Docker)

```bash
# 1. Llamada directa desde otro contenedor de la red interna, sin token → 401
docker run --rm --network app-net curlimages/curl -s -o /dev/null -w "%{http_code}\n" \
  -X PUT http://jokerarena-liveops-rules-ms:8087/liveops/ruleset
# (antes de unir la segmentación: --network jokerarena-net)

# 2. El GET sigue abierto → 200
docker run --rm --network app-net curlimages/curl -s -o /dev/null -w "%{http_code}\n" \
  http://jokerarena-liveops-rules-ms:8087/liveops/ruleset

# 3. Rechazo registrado sin el token
docker logs jokerarena-liveops-rules-ms 2>&1 | grep "cambio rechazado por autorización"

# 4. La consola de administración sigue guardando cambios (https://localhost → Admin)
```

_[Captura: terminal con 401 / 200]_ · _[Captura: línea del log]_ · _[Captura: cambio guardado desde la consola de admin]_

## Resultado obtenido

Ningún cambio al ruleset se acepta sin un token admin válido emitido por `auth-identity-ms`, aunque la petición llegue desde dentro de la red interna sin pasar por el gateway. El motor sigue leyendo el ruleset sin token, y la consola de administración funciona igual, porque el gateway ya reenvía el `Authorization` del administrador.

## Relación con otros escenarios

- **Escenario 3 (Network Segmentation):** limita quién puede llegar a `liveops` por red (por ejemplo, `bot-ai-ms` no ve PostgreSQL). Este escenario garantiza que quien sí llega no puede cambiar nada sin credenciales.
- **Escenario 4 (Account Lockout):** protege cómo se obtiene el token admin y reduce su duración a 30 minutos. Este escenario verifica ese token en el destino, así que un token robado tiene una ventana de uso corta.

## Limitaciones

- Entre `gateway` y `liveops` el tráfico sigue sin TLS dentro de la red interna (el Secure Channel del escenario 1 cubre cliente ↔ borde). La firma del JWT protege la integridad del token, no la confidencialidad del canal interno.
- Si Redis no responde, no se puede comprobar si la sesión se cerró. El token se acepta mientras no venza (30 minutos para admin, según el escenario 4).
