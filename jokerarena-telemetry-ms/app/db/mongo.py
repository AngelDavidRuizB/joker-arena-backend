import asyncio
import logging
import os
import uuid
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Tuple

from app.config import settings

logger = logging.getLogger("telemetry.db")

try:
    from motor.motor_asyncio import (
        AsyncIOMotorClient,
        AsyncIOMotorCollection,
        AsyncIOMotorDatabase,
    )
    MOTOR_AVAILABLE = True
except ImportError:
    AsyncIOMotorClient = None  # type: ignore
    AsyncIOMotorCollection = None  # type: ignore
    AsyncIOMotorDatabase = None  # type: ignore
    MOTOR_AVAILABLE = False


class MongoManager:
    """Async MongoDB manager using Motor with seamless offline fallback."""

    def __init__(self, mongo_url: Optional[str] = None, db_name: Optional[str] = None):
        self.mongo_url = mongo_url or settings.MONGO_URL
        self.db_name = db_name or settings.MONGO_DB
        self.client: Optional[AsyncIOMotorClient] = None
        self.db: Optional[AsyncIOMotorDatabase] = None
        self.is_connected: bool = False
        self.fallback_events: List[Dict[str, Any]] = []

    async def connect(self):
        """Initializes Motor client pool, pings server, and builds indexes."""
        if self.client is not None:
            return

        if not MOTOR_AVAILABLE:
            self.is_connected = False
            logger.warning("Motor is not installed; running in in-memory fallback mode.")
            return

        try:
            logger.info(f"Connecting to MongoDB at {self.mongo_url} (db: {self.db_name})...")
            self.client = AsyncIOMotorClient(
                self.mongo_url,
                serverSelectionTimeoutMS=2000,
                connectTimeoutMS=2000,
                maxPoolSize=50,
                minPoolSize=5,
            )
            self.db = self.client[self.db_name]
            await self.client.admin.command("ping", maxTimeMS=1000)
            self.is_connected = True
            logger.info("Successfully connected to MongoDB.")
            await self._ensure_indexes()
        except Exception as exc:
            if self.client is not None:
                try:
                    self.client.close()
                except Exception:
                    pass
            self.client = None
            self.db = None
            self.is_connected = False
            logger.warning(f"MongoDB connection failed ({exc}). Operating in resilient fallback mode.")

    async def _ensure_indexes(self):
        """Creates indexes on hand_events collection idempotently."""
        if self.db is None:
            return
        try:
            coll = self.db["hand_events"]
            await coll.create_index("match_id")
            await coll.create_index("player_id")
            await coll.create_index([("match_id", 1), ("round_number", 1), ("hand_number", 1)])
            await coll.create_index([("timestamp", -1)])
            logger.info("MongoDB indexes verified on hand_events collection.")
        except Exception as exc:
            logger.warning(f"Failed to create indexes on hand_events: {exc}")

    async def ping(self) -> bool:
        """Pings MongoDB with a short timeout to check health."""
        if not self.is_connected or self.client is None or not MOTOR_AVAILABLE:
            return False
        try:
            await asyncio.wait_for(self.client.admin.command("ping"), timeout=0.2)
            self.is_connected = True
            return True
        except Exception:
            self.is_connected = False
            return False

    async def close(self):
        """Closes Motor client pool."""
        if self.client:
            logger.info("Closing MongoDB connection pool...")
            try:
                self.client.close()
            except Exception:
                pass
            self.client = None
            self.db = None
            self.is_connected = False

    def get_collection(self) -> Optional[AsyncIOMotorCollection]:
        if self.is_connected and self.db is not None:
            return self.db["hand_events"]
        return None

    async def record_event(self, event_data: Dict[str, Any]) -> str:
        """Persists a hand event either to MongoDB or to in-memory fallback store."""
        if "event_id" not in event_data or not event_data["event_id"]:
            event_data["event_id"] = str(uuid.uuid4())
        if "timestamp" not in event_data or not event_data["timestamp"]:
            event_data["timestamp"] = datetime.now(timezone.utc).isoformat()
        if "created_at" not in event_data:
            event_data["created_at"] = datetime.now(timezone.utc).isoformat()

        if self.is_connected and self.db is not None:
            try:
                coll = self.db["hand_events"]
                # Create a document copy without modifying input
                doc = dict(event_data)
                res = await coll.insert_one(doc)
                return event_data["event_id"]
            except Exception as exc:
                logger.error(f"MongoDB write failed: {exc}. Buffering to in-memory fallback store.")

        # In-memory fallback
        self.fallback_events.append(dict(event_data))
        return event_data["event_id"]

    async def get_hands(
        self,
        query: Optional[Dict[str, Any]] = None,
        limit: int = 50,
        skip: int = 0,
    ) -> Tuple[int, List[Dict[str, Any]]]:
        """Lists hand events with filtering and pagination."""
        q = query or {}
        if self.is_connected and self.db is not None:
            try:
                coll = self.db["hand_events"]
                total = await coll.count_documents(q)
                cursor = coll.find(q, {"_id": 0}).sort("timestamp", -1).skip(skip).limit(limit)
                events = [doc async for doc in cursor]
                return total, events
            except Exception as exc:
                logger.warning(f"Error querying MongoDB: {exc}. Reading from fallback buffer.")

        # Filter from fallback buffer
        matched = []
        for ev in self.fallback_events:
            matches = True
            for k, v in q.items():
                if ev.get(k) != v:
                    matches = False
                    break
            if matches:
                matched.append(ev)

        matched.sort(key=lambda x: x.get("timestamp", ""), reverse=True)
        total = len(matched)
        return total, matched[skip : skip + limit]

    async def get_hand_by_id(self, event_id: str) -> Optional[Dict[str, Any]]:
        """Retrieves single hand event by event_id."""
        if self.is_connected and self.db is not None:
            try:
                coll = self.db["hand_events"]
                doc = await coll.find_one({"event_id": event_id}, {"_id": 0})
                if doc:
                    return doc
            except Exception as exc:
                logger.warning(f"Error fetching event {event_id} from MongoDB: {exc}")

        for ev in self.fallback_events:
            if ev.get("event_id") == event_id:
                return ev
        return None

    async def get_matches_summary(self) -> List[Dict[str, Any]]:
        """Summarizes distinct matches."""
        if self.is_connected and self.db is not None:
            try:
                coll = self.db["hand_events"]
                pipeline = [
                    {
                        "$group": {
                            "_id": "$match_id",
                            "hands_played": {"$sum": 1},
                            "latest_timestamp": {"$max": "$timestamp"},
                            "final_score": {"$max": "$accumulated_score"},
                        }
                    },
                    {"$sort": {"latest_timestamp": -1}},
                    {"$limit": 50},
                ]
                results = []
                async for item in coll.aggregate(pipeline):
                    results.append({
                        "match_id": item["_id"],
                        "hands_played": item["hands_played"],
                        "latest_timestamp": item.get("latest_timestamp"),
                        "final_score": item.get("final_score", 0),
                    })
                return results
            except Exception as exc:
                logger.warning(f"Error aggregating matches from MongoDB: {exc}")

        # In-memory aggregation
        grouped: Dict[str, Dict[str, Any]] = {}
        for ev in self.fallback_events:
            m_id = ev.get("match_id", "unknown")
            if m_id not in grouped:
                grouped[m_id] = {
                    "match_id": m_id,
                    "hands_played": 0,
                    "latest_timestamp": None,
                    "final_score": 0,
                }
            g = grouped[m_id]
            g["hands_played"] += 1
            ts = ev.get("timestamp")
            if ts and (g["latest_timestamp"] is None or ts > g["latest_timestamp"]):
                g["latest_timestamp"] = ts
            score = ev.get("accumulated_score", 0)
            if score > g["final_score"]:
                g["final_score"] = score

        results = list(grouped.values())
        results.sort(key=lambda x: x.get("latest_timestamp") or "", reverse=True)
        return results[:50]

    async def get_match_events(self, match_id: str) -> List[Dict[str, Any]]:
        """Returns all hand events for a match sorted sequentially."""
        if self.is_connected and self.db is not None:
            try:
                coll = self.db["hand_events"]
                cursor = coll.find({"match_id": match_id}, {"_id": 0}).sort(
                    [("round_number", 1), ("hand_number", 1), ("timestamp", 1)]
                )
                return [doc async for doc in cursor]
            except Exception as exc:
                logger.warning(f"Error fetching match events from MongoDB: {exc}")

        matched = [ev for ev in self.fallback_events if ev.get("match_id") == match_id]
        matched.sort(
            key=lambda x: (
                x.get("round_number", 1),
                x.get("hand_number", 1),
                x.get("timestamp", ""),
            )
        )
        return matched


mongo_manager = MongoManager()
db_manager = mongo_manager


async def init_mongo() -> MongoManager:
    await mongo_manager.connect()
    return mongo_manager


async def close_mongo():
    await mongo_manager.close()


def get_db():
    return mongo_manager.db
