"""Consumidor RabbitMQ (cola telemetry-ms.events) con reconexión automática."""
import asyncio
import json
import logging
from typing import Optional

import aio_pika

from .projector import InvalidEvent, project
from .store import TelemetryStore

log = logging.getLogger("telemetry.consumer")
QUEUE = "telemetry-ms.events"
BINDINGS = ("game.#", "lobby.room.created")


class Consumer:
    def __init__(self, url: str, exchange: str, store: TelemetryStore):
        self.url, self.exchange, self.store = url, exchange, store
        self._conn: Optional[aio_pika.abc.AbstractRobustConnection] = None
        self._task: Optional[asyncio.Task] = None

    @property
    def connected(self) -> bool:
        return self._conn is not None and not self._conn.is_closed

    def start(self) -> None:
        self._task = asyncio.create_task(self._run())

    async def _run(self) -> None:
        delay = 1
        while True:
            try:
                self._conn = await aio_pika.connect_robust(self.url, timeout=5)
                channel = await self._conn.channel()
                await channel.set_qos(prefetch_count=20)
                exchange = await channel.declare_exchange(self.exchange, aio_pika.ExchangeType.TOPIC, durable=True)
                queue = await channel.declare_queue(QUEUE, durable=True)
                for key in BINDINGS:
                    await queue.bind(exchange, key)
                await queue.consume(self._on_message)
                log.info("consumiendo %s (%s)", QUEUE, ", ".join(BINDINGS))
                return
            except Exception as exc:  # noqa: BLE001 - el broker puede tardar en arrancar
                log.warning("RabbitMQ no disponible (%s); reintento en %ss", exc, delay)
                await asyncio.sleep(delay)
                delay = min(delay * 2, 15)

    async def _on_message(self, message: aio_pika.abc.AbstractIncomingMessage) -> None:
        try:
            envelope = json.loads(message.body)
            result = await project(self.store, envelope)
            log.info("%s %s (%s)", result, envelope.get("type"), envelope.get("event_id"))
            await message.ack()
        except (json.JSONDecodeError, InvalidEvent) as exc:
            log.error("mensaje descartado: %s", exc)
            await message.reject(requeue=False)
        except Exception as exc:  # noqa: BLE001 - Mongo caído: reintentar
            log.warning("fallo transitorio (%s): se reintenta", exc)
            await asyncio.sleep(2)
            await message.nack(requeue=True)

    async def close(self) -> None:
        if self._task:
            self._task.cancel()
        if self._conn:
            await self._conn.close()
