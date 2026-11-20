"""Convierte los eventos del broker (contracts/events.md) en documentos de telemetría."""
import logging
from datetime import datetime

from .store import TelemetryStore

log = logging.getLogger("telemetry.projector")

HANDLED = {"game.hand.scored", "game.match.started", "game.match.ended", "lobby.room.created"}
REQUIRED = {"event_id", "type", "version", "occurred_at", "producer", "payload"}


class InvalidEvent(Exception):
    """Evento fuera de contrato: no se reintenta."""


def _ts(value: str) -> datetime:
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except (AttributeError, ValueError) as exc:
        raise InvalidEvent(f"occurred_at inválido: {value!r}") from exc


def _need(payload: dict, *keys: str) -> None:
    missing = [k for k in keys if k not in payload]
    if missing:
        raise InvalidEvent(f"faltan campos en el payload: {missing}")


async def project(store: TelemetryStore, envelope: dict) -> str:
    """Devuelve 'processed', 'duplicate' o 'ignored'. Lanza InvalidEvent si no cumple el contrato."""
    if not isinstance(envelope, dict) or not REQUIRED.issubset(envelope):
        raise InvalidEvent("sobre incompleto")
    etype, payload = envelope["type"], envelope["payload"]
    if not isinstance(payload, dict):
        raise InvalidEvent("payload debe ser un objeto")
    if etype not in HANDLED:
        return "ignored"
    occurred = _ts(envelope["occurred_at"])
    if etype == "game.hand.scored":
        _need(payload, "match_id", "player_id", "hand_number", "hand_type", "score")
    elif etype in ("game.match.started", "game.match.ended"):
        _need(payload, "match_id", "room_code")
    if await store.seen(envelope["event_id"]):
        return "duplicate"

    # Primero se proyecta y al final se registra el evento: si Mongo falla a mitad, el reintento
    # no lo toma por duplicado. Las proyecciones son upserts, así que repetirlas es seguro.
    if etype == "game.hand.scored":
        await store.upsert_hand({**payload, "occurred_at": occurred, "event_id": envelope["event_id"]})
    elif etype == "game.match.started":
        await store.match_started({**payload, "started_at": occurred})
    elif etype == "game.match.ended":
        await store.match_ended({**payload, "ended_at": occurred})
    await store.save_raw(envelope)
    return "processed"
