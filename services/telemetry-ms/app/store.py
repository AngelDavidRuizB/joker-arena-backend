"""Proyecciones de telemetría en MongoDB (base "telemetry").

- events:          copia cruda de cada evento recibido (event_id único → idempotencia).
- hand_events:     una fila por mano jugada (match_id, player_id, hand_number únicos).
- match_summaries: resumen por partida (inicio, resultado final).
"""
from datetime import datetime, timezone
from typing import Optional, Protocol

from motor.motor_asyncio import AsyncIOMotorClient
from pymongo import ASCENDING, DESCENDING
from pymongo.errors import DuplicateKeyError


class TelemetryStore(Protocol):
    async def save_raw(self, envelope: dict) -> bool: ...
    async def upsert_hand(self, doc: dict) -> None: ...
    async def match_started(self, doc: dict) -> None: ...
    async def match_ended(self, doc: dict) -> None: ...
    async def list_hands(self, match_id: Optional[str], player_id: Optional[str], limit: int) -> list: ...
    async def list_matches(self, limit: int) -> list: ...
    async def get_match(self, match_id: str) -> Optional[dict]: ...
    async def stats(self) -> dict: ...
    async def ping(self) -> None: ...


def _clean(doc: Optional[dict]) -> Optional[dict]:
    if doc is not None:
        doc.pop("_id", None)
    return doc


class MongoStore:
    def __init__(self, url: str, db: str = "telemetry"):
        self.client = AsyncIOMotorClient(url, serverSelectionTimeoutMS=3000, uuidRepresentation="standard")
        self.db = self.client[db]

    async def ensure_indexes(self) -> None:
        await self.db.events.create_index("event_id", unique=True)
        await self.db.hand_events.create_index([("match_id", ASCENDING), ("player_id", ASCENDING), ("hand_number", ASCENDING)], unique=True)
        await self.db.hand_events.create_index([("occurred_at", DESCENDING)])
        await self.db.match_summaries.create_index("match_id", unique=True)
        await self.db.match_summaries.create_index([("started_at", DESCENDING)])

    async def save_raw(self, envelope: dict) -> bool:
        try:
            await self.db.events.insert_one({**envelope, "received_at": datetime.now(timezone.utc)})
            return True
        except DuplicateKeyError:
            return False

    async def upsert_hand(self, doc: dict) -> None:
        key = {k: doc[k] for k in ("match_id", "player_id", "hand_number")}
        await self.db.hand_events.update_one(key, {"$set": doc}, upsert=True)

    async def match_started(self, doc: dict) -> None:
        await self.db.match_summaries.update_one(
            {"match_id": doc["match_id"]},
            {"$setOnInsert": {"status": "in_progress"}, "$set": {k: v for k, v in doc.items() if k != "status"}},
            upsert=True,
        )

    async def match_ended(self, doc: dict) -> None:
        await self.db.match_summaries.update_one({"match_id": doc["match_id"]}, {"$set": {**doc, "status": "finished"}}, upsert=True)

    async def list_hands(self, match_id, player_id, limit):
        q = {}
        if match_id:
            q["match_id"] = match_id
        if player_id:
            q["player_id"] = player_id
        sort = [("hand_number", ASCENDING), ("occurred_at", ASCENDING)] if match_id else [("occurred_at", DESCENDING)]
        return [_clean(d) async for d in self.db.hand_events.find(q).sort(sort).limit(limit)]

    async def list_matches(self, limit):
        return [_clean(d) async for d in self.db.match_summaries.find({}).sort("started_at", DESCENDING).limit(limit)]

    async def get_match(self, match_id):
        return _clean(await self.db.match_summaries.find_one({"match_id": match_id}))

    async def stats(self):
        by_type = [
            {"hand_type": d["_id"], "count": d["count"], "avg_score": round(d["avg"], 1)}
            async for d in self.db.hand_events.aggregate([
                {"$group": {"_id": "$hand_type", "count": {"$sum": 1}, "avg": {"$avg": "$score"}}},
                {"$sort": {"count": -1}},
            ])
        ]
        return {
            "matches": await self.db.match_summaries.count_documents({}),
            "finished_matches": await self.db.match_summaries.count_documents({"status": "finished"}),
            "hands": await self.db.hand_events.count_documents({}),
            "hands_by_type": by_type,
        }

    async def ping(self) -> None:
        await self.client.admin.command("ping")

    def close(self) -> None:
        self.client.close()
