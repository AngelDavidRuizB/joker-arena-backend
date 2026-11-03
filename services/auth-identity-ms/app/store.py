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


class AliasTaken(Exception):
    pass


class PlayerStore(Protocol):
    async def create(self, alias: str, role: str) -> Player: ...
    async def by_alias(self, alias: str) -> Optional[Player]: ...
    async def by_id(self, player_id: str) -> Optional[Player]: ...
    async def ping(self) -> None: ...


class SessionStore(Protocol):
    async def start(self, jti: str, player_id: str, ttl: int) -> None: ...
    async def revoke(self, jti: str, ttl: int) -> None: ...
    async def is_revoked(self, jti: str) -> bool: ...
    async def ping(self) -> None: ...


class PostgresPlayerStore:
    def __init__(self, dsn: str):
        self.dsn = dsn
        self.pool: Optional[asyncpg.Pool] = None

    async def _pool(self) -> asyncpg.Pool:
        if self.pool is None:
            self.pool = await asyncpg.create_pool(self.dsn, min_size=1, max_size=5, timeout=3)
        return self.pool

    @staticmethod
    def _row(r) -> Player:
        return Player(str(r["player_id"]), r["alias"], r["role"], r["created_at"])

    async def create(self, alias: str, role: str) -> Player:
        pool = await self._pool()
        try:
            r = await pool.fetchrow(
                "INSERT INTO auth.players (player_id, alias, role) VALUES ($1, $2, $3) "
                "RETURNING player_id, alias, role, created_at",
                uuid.uuid4(), alias, role,
            )
        except asyncpg.UniqueViolationError as exc:
            raise AliasTaken(alias) from exc
        return self._row(r)

    async def by_alias(self, alias: str) -> Optional[Player]:
        pool = await self._pool()
        r = await pool.fetchrow(
            "SELECT player_id, alias, role, created_at FROM auth.players WHERE lower(alias) = lower($1)", alias
        )
        return self._row(r) if r else None

    async def by_id(self, player_id: str) -> Optional[Player]:
        try:
            pid = uuid.UUID(player_id)
        except ValueError:
            return None
        pool = await self._pool()
        r = await pool.fetchrow("SELECT player_id, alias, role, created_at FROM auth.players WHERE player_id = $1", pid)
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


def utcnow() -> datetime:
    return datetime.now(timezone.utc)
