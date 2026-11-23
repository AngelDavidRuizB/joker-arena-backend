# auth-identity-ms

Identidad de jugadores. **Responsable:** Esteban Andrés Gómez Álvarez · **Python 3.12 / FastAPI** · puerto interno **8082**.

Registra jugadores con **usuario y contraseña** (contraseña guardada como hash PBKDF2-SHA256 en PostgreSQL) y emite **JWT firmados con RS256**. Publica las llaves públicas en `/.well-known/jwks.json` para que el gateway y el motor validen tokens sin llamar a este servicio en cada petición. El usuario reservado `admin` exige además `ADMIN_PASSWORD` y recibe el rol `admin` (rutas `/admin/*`).

## Endpoints

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/auth/register` | `{"user", "password", "repeat_password"?}` → `201 {token, player}`. `409 alias_taken` si ya existe con contraseña; `400 invalid_password` / `password_mismatch`. Publica `auth.player.registered`. |
| POST | `/auth/login` | `{"user", "password"}` → `{token, player}`. `401 invalid_credentials` si la contraseña no coincide, `404` si no existe, `401 password_not_set` en cuentas antiguas sin contraseña. |
| GET | `/auth/me` | Datos del token (`Authorization: Bearer`). |
| POST | `/auth/logout` | Revoca el token (Redis `auth:revoked:{jti}`). |
| GET | `/auth/verify` | Usado por el gateway (`auth_request`): `204` + `X-Player-Id/Alias/Role`, `401` o `403`. |
| GET | `/.well-known/jwks.json` | Llave pública RS256. |
| GET | `/health` | Estado de PostgreSQL, Redis y RabbitMQ. |

`alias` se sigue aceptando como sinónimo de `user` para clientes anteriores. Claims del token: `iss`, `sub` (player_id UUID), `alias`, `role`, `iat`, `exp` (12 h), `jti`.

## Datos

- PostgreSQL, esquema `auth`: tabla `auth.players` (alias único sin distinguir mayúsculas, `password_hash` con sal) — `db/postgres/init/10_auth.sql`. Al arrancar el servicio aplica DDL idempotente (`ALTER TABLE ... ADD COLUMN IF NOT EXISTS password_hash`) para volúmenes antiguos.
- Contraseñas: `pbkdf2_sha256$<iteraciones>$<sal>$<hash>` con 240 000 iteraciones (`app/passwords.py`, sin dependencias extra).
- Redis: `auth:session:{jti}` (sesión activa, TTL) y `auth:revoked:{jti}`.
- Llave privada RSA en el volumen `p2-authkeys` (sobrevive reinicios).

## Variables

`PORT`, `POSTGRES_URL`, `REDIS_URL`, `RABBITMQ_URL`, `ADMIN_PASSWORD`, `JWT_KEY_FILE`, `JWT_TTL_SECONDS`.

## Pruebas

```bash
pip install -r requirements-dev.txt && pytest -q
```
