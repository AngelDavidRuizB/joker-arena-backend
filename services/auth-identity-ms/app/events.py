"""Publicación de eventos en RabbitMQ (contracts/events.md) con reconexión automática."""
import asyncio
import json
import logging
import uuid
from datetime import datetime, timezone
from typing import Optional, Protocol

import aio_pika

log = logging.getLogger("auth.events")


class Publisher(Protocol):
    async def publish(self, routing_key: str, payload: dict) -> None: ...
    @property
    def connected(self) -> bool: ...


def envelope(routing_key: str, payload: dict, producer: str = "auth-identity-ms") -> dict:
    return {
        "event_id": str(uuid.uuid4()),
        "type": routing_key,
        "version": 1,
        "occurred_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "producer": producer,
        "payload": payload,
    }


class RabbitPublisher:
    def __init__(self, url: str, exchange: str):
        self.url, self.exchange_name = url, exchange
        self._conn: Optional[aio_pika.abc.AbstractRobustConnection] = None
        self._exchange = None
        self._task: Optional[asyncio.Task] = None

    @property
    def connected(self) -> bool:
        return self._exchange is not None and self._conn is not None and not self._conn.is_closed

    def start(self) -> None:
        self._task = asyncio.create_task(self._connect_loop())

    async def _connect_loop(self) -> None:
        delay = 1
        while True:
            try:
                self._conn = await aio_pika.connect_robust(self.url, timeout=5)
                channel = await self._conn.channel(publisher_confirms=True)
                self._exchange = await channel.declare_exchange(self.exchange_name, aio_pika.ExchangeType.TOPIC, durable=True)
                log.info("RabbitMQ conectado (exchange %s)", self.exchange_name)
                return
            except Exception as exc:  # noqa: BLE001 - el broker puede tardar en arrancar
                log.warning("RabbitMQ no disponible (%s); reintento en %ss", exc, delay)
                await asyncio.sleep(delay)
                delay = min(delay * 2, 15)

    async def publish(self, routing_key: str, payload: dict) -> None:
        if not self.connected:
            log.warning("evento %s no publicado: broker no conectado", routing_key)
            return
        body = json.dumps(envelope(routing_key, payload)).encode()
        try:
            await self._exchange.publish(
                aio_pika.Message(body, content_type="application/json", delivery_mode=aio_pika.DeliveryMode.PERSISTENT),
                routing_key=routing_key,
            )
        except Exception as exc:  # noqa: BLE001 - publicar nunca debe tumbar el registro
            log.error("no se pudo publicar %s: %s", routing_key, exc)

    async def close(self) -> None:
        if self._task:
            self._task.cancel()
        if self._conn:
            await self._conn.close()
