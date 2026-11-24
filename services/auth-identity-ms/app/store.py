"""Persistencia: jugadores en PostgreSQL (esquema auth) y sesiones en Redis."""
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Optional, Protocol

import asyncpg
import redis.asyncio as aioredis


@dataclass
class Player:
    player_id: str
    alias: str
    role: str
    created_at: datetime
    password_hash: Optional[str] = None


class AliasTaken(Exception):
    pass


class PlayerStore(Protocol):
    async def ensure_schema(self) -> None: ...
    async def create(self, alias: str, role: str, password_hash: Optional[str] = None) -> Player: ...
    async def attach_password(self, alias: str, password_hash: str) -> Optional[Player]: ...
    async def by_alias(self, alias: str) -> Optional[Player]: ...
    async def by_id(self, player_id: str) -> Optional[Player]: ...
    async def ping(self) -> None: ...


class SessionStore(Protocol):
    async def start(self, jti: str, player_id: str, ttl: int) -> None: ...
    async def revoke(self, jti: str, ttl: int) -> None: ...
    async def is_revoked(self, jti: str) -> bool: ...
    async def ping(self) -> None: ...


class LoginThrottle(Protocol):
    """Cuenta intentos fallidos de credenciales y bloquea la cuenta un tiempo."""

    async def remaining_lock(self, alias: str) -> int: ...
    async def register_failure(self, alias: str, max_attempts: int, lockout_seconds: int) -> int: ...
    async def clear(self, alias: str) -> None: ...
    async def ping(self) -> None: ...


class PostgresPlayerStore:
    # DDL idempotente: se aplica al arrancar para que un volumen creado antes del
    # registro con contraseña reciba la columna password_hash sin recrear la base.
    SCHEMA_STATEMENTS = (
        "CREATE TABLE IF NOT EXISTS auth.players ("
        " player_id UUID PRIMARY KEY,"
        " alias VARCHAR(30) NOT NULL,"
        " role VARCHAR(10) NOT NULL DEFAULT 'player' CHECK (role IN ('player', 'admin')),"
        " password_hash TEXT,"
        " created_at TIMESTAMPTZ NOT NULL DEFAULT now()"
        ")",
        "CREATE UNIQUE INDEX IF NOT EXISTS ux_auth_players_alias ON auth.players (lower(alias))",
        "ALTER TABLE auth.players ADD COLUMN IF NOT EXISTS password_hash TEXT",
    )
    COLUMNS = "player_id, alias, role, created_at, password_hash"

    def __init__(self, dsn: str):
        self.dsn = dsn
        self.pool: Optional[asyncpg.Pool] = None

    async def _pool(self) -> asyncpg.Pool:
        if self.pool is None:
            self.pool = await asyncpg.create_pool(self.dsn, min_size=1, max_size=5, timeout=3)
        return self.pool

    @staticmethod
    def _row(r) -> Player:
        return Player(str(r["player_id"]), r["alias"], r["role"], r["created_at"], r["password_hash"])

    async def ensure_schema(self) -> None:
        pool = await self._pool()
        async with pool.acquire() as conn:
            for statement in self.SCHEMA_STATEMENTS:
                await conn.execute(statement)

    async def create(self, alias: str, role: str, password_hash: Optional[str] = None) -> Player:
        pool = await self._pool()
        try:
            r = await pool.fetchrow(
                f"INSERT INTO auth.players (player_id, alias, role, password_hash) VALUES ($1, $2, $3, $4) "
                f"RETURNING {self.COLUMNS}",
                uuid.uuid4(), alias, role, password_hash,
            )
        except asyncpg.UniqueViolationError as exc:
            raise AliasTaken(alias) from exc
        return self._row(r)

    async def attach_password(self, alias: str, password_hash: str) -> Optional[Player]:
        """Fija la contraseña de una cuenta legacy (sin hash). None si ya tenía una."""
        pool = await self._pool()
        r = await pool.fetchrow(
            f"UPDATE auth.players SET password_hash = $2 "
            f"WHERE lower(alias) = lower($1) AND password_hash IS NULL RETURNING {self.COLUMNS}",
            alias, password_hash,
        )
        return self._row(r) if r else None

    async def by_alias(self, alias: str) -> Optional[Player]:
        pool = await self._pool()
        r = await pool.fetchrow(
            f"SELECT {self.COLUMNS} FROM auth.players WHERE lower(alias) = lower($1)", alias
        )
        return self._row(r) if r else None

    async def by_id(self, player_id: str) -> Optional[Player]:
        try:
            pid = uuid.UUID(player_id)
        except ValueError:
            return None
        pool = await self._pool()
        r = await pool.fetchrow(f"SELECT {self.COLUMNS} FROM auth.players WHERE player_id = $1", pid)
        return self._row(r) if r else None

    async def ping(self) -> None:
        pool = await self._pool()
        await pool.fetchval("SELECT 1")

    async def close(self) -> None:
        if self.pool:
            await self.pool.close()


class RedisSessionStore:
    def __init__(self, url: str):
        self.client = aioredis.from_url(url, socket_connect_timeout=2, socket_timeout=2)

    async def start(self, jti: str, player_id: str, ttl: int) -> None:
        await self.client.set(f"auth:session:{jti}", player_id, ex=ttl)

    async def revoke(self, jti: str, ttl: int) -> None:
        await self.client.delete(f"auth:session:{jti}")
        await self.client.set(f"auth:revoked:{jti}", "1", ex=max(ttl, 1))

    async def is_revoked(self, jti: str) -> bool:
        return bool(await self.client.exists(f"auth:revoked:{jti}"))

    async def ping(self) -> None:
        await self.client.ping()

    async def close(self) -> None:
        await self.client.aclose()


class RedisLoginThrottle:
    """Intentos fallidos por usuario en Redis.

    - ``auth:fail:<usuario>``: contador de fallos (TTL = ventana de bloqueo).
    - ``auth:lock:<usuario>``: marca de bloqueo (TTL = duración del bloqueo); mientras exista,
      /auth/login y /auth/register responden 423 para ese usuario.
    """

    def __init__(self, url: str):
        self.client = aioredis.from_url(url, socket_connect_timeout=2, socket_timeout=2)

    @staticmethod
    def _fail_key(alias: str) -> str:
        return f"auth:fail:{alias.lower()}"

    @staticmethod
    def _lock_key(alias: str) -> str:
        return f"auth:lock:{alias.lower()}"

    async def remaining_lock(self, alias: str) -> int:
        ttl = await self.client.ttl(self._lock_key(alias))
        return max(int(ttl), 0)

    async def register_failure(self, alias: str, max_attempts: int, lockout_seconds: int) -> int:
        """Suma un fallo y devuelve los intentos restantes (0 = cuenta bloqueada ahora)."""
        failures = int(await self.client.incr(self._fail_key(alias)))
        await self.client.expire(self._fail_key(alias), lockout_seconds)
        if failures >= max(max_attempts, 1):
            await self.client.set(self._lock_key(alias), "1", ex=lockout_seconds)
            await self.client.delete(self._fail_key(alias))
            return 0
        return max_attempts - failures

    async def clear(self, alias: str) -> None:
        await self.client.delete(self._fail_key(alias), self._lock_key(alias))

    async def ping(self) -> None:
        await self.client.ping()

    async def close(self) -> None:
        await self.client.aclose()


def utcnow() -> datetime:
    return datetime.now(timezone.utc)
