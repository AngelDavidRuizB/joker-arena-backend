"""
JokerArena E2E Test Suite - Client and Protocol Harness
Provides REST client, WebSocket client, DB probes, and interface validators.
"""
import asyncio
import json
import re
import socket
import threading
import time
import uuid
from typing import Any, Dict, List, Optional, Tuple

import requests
import websockets

from tests.config import (
    FRONTEND_PORT,
    FRONTEND_URL,
    GAME_ENGINE_PORT,
    GAME_ENGINE_URL,
    GAME_ENGINE_WS_URL,
    MONGO_DB,
    MONGO_PORT,
    POSTGRES_DB,
    POSTGRES_PASSWORD,
    POSTGRES_PORT,
    POSTGRES_USER,
    TARGET_HOST,
    TELEMETRY_GRPC_ADDR,
    TELEMETRY_GRPC_PORT,
    TELEMETRY_HTTP_PORT,
    TELEMETRY_HTTP_URL,
    TIMEOUT_SECONDS,
    is_mock_mode,
)

# ------------------------------------------------------------------------------
# Validation Helpers
# ------------------------------------------------------------------------------

FRENCH_SUITS = {"SPADES", "HEARTS", "CLUBS", "DIAMONDS"}
FRENCH_RANKS = {"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
RANK_CHIP_MAP = {
    "2": 2, "3": 3, "4": 4, "5": 5, "6": 6,
    "7": 7, "8": 8, "9": 9, "10": 10,
    "J": 10, "Q": 10, "K": 10, "A": 11,
}
SEED_JOKERS = {"banner", "joker", "the_duo"}


def is_valid_uuid(val: str) -> bool:
    try:
        uuid.UUID(str(val))
        return True
    except (ValueError, AttributeError, TypeError):
        return False


def validate_french_card(card: dict) -> Tuple[bool, str]:
    if not isinstance(card, dict):
        return False, "Card must be a dictionary"
    rank = str(card.get("rank", "")).upper()
    suit = str(card.get("suit", "")).upper()
    chips = card.get("chips")

    if rank not in FRENCH_RANKS:
        return False, f"Invalid card rank: {rank}"
    if suit not in FRENCH_SUITS:
        return False, f"Invalid card suit: {suit}"
    if not isinstance(chips, int) or chips != RANK_CHIP_MAP[rank]:
        return False, f"Invalid chips {chips} for rank {rank} (expected {RANK_CHIP_MAP[rank]})"
    return True, "Valid card"


def validate_room_state_update(payload: dict) -> Tuple[bool, str]:
    if not isinstance(payload, dict):
        return False, "Payload must be a dictionary"

    required_fields = [
        "room_code", "target_score", "hands_left", "discards_left",
        "current_score", "deck_count", "cards_in_hand"
    ]
    for field in required_fields:
        if field not in payload:
            return False, f"Missing required field: {field}"

    cards = payload.get("cards_in_hand", [])
    if not isinstance(cards, list):
        return False, "cards_in_hand must be a list"

    for i, c in enumerate(cards):
        valid, msg = validate_french_card(c)
        if not valid:
            return False, f"Card at index {i} invalid: {msg}"

    if payload["target_score"] != 300:
        return False, f"target_score expected 300, got {payload['target_score']}"

    if payload["hands_left"] < 0 or payload["hands_left"] > 4:
        return False, f"hands_left out of range [0, 4]: {payload['hands_left']}"

    if payload["discards_left"] < 0 or payload["discards_left"] > 3:
        return False, f"discards_left out of range [0, 3]: {payload['discards_left']}"

    return True, "Valid room state"


# ------------------------------------------------------------------------------
# REST Client
# ------------------------------------------------------------------------------

class RESTClient:
    """Client for Game Engine, Telemetry, and Frontend REST endpoints."""

    def __init__(self, base_url: str = GAME_ENGINE_URL, timeout: float = TIMEOUT_SECONDS):
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.session = requests.Session()

    def get(self, endpoint: str, **kwargs) -> requests.Response:
        url = f"{self.base_url}/{endpoint.lstrip('/')}"
        return self.session.get(url, timeout=kwargs.pop("timeout", self.timeout), **kwargs)

    def post(self, endpoint: str, json_data: dict = None, **kwargs) -> requests.Response:
        url = f"{self.base_url}/{endpoint.lstrip('/')}"
        payload = json_data if json_data is not None else kwargs.pop("json", None)
        return self.session.post(url, json=payload, timeout=kwargs.pop("timeout", self.timeout), **kwargs)

    # Game Engine endpoints
    def get_game_engine_health(self) -> requests.Response:
        return self.get("/health")

    def register_player(self, alias: str) -> requests.Response:
        return self.post("/api/players", {"alias": alias})

    def create_room(self, player_id: str, target_score: int = 300, with_bot: bool = True) -> requests.Response:
        return self.post("/api/rooms", {
            "player_id": player_id,
            "target_score": target_score,
            "with_bot": with_bot
        })

    def get_room(self, code: str) -> requests.Response:
        return self.get(f"/api/rooms/{code}")

    # Telemetry endpoints
    def get_telemetry_health(self) -> requests.Response:
        url = f"{TELEMETRY_HTTP_URL}/health"
        return self.session.get(url, timeout=self.timeout)

    def get_telemetry_docs(self) -> requests.Response:
        url = f"{TELEMETRY_HTTP_URL}/docs"
        return self.session.get(url, timeout=self.timeout)

    def get_telemetry_openapi(self) -> requests.Response:
        url = f"{TELEMETRY_HTTP_URL}/openapi.json"
        return self.session.get(url, timeout=self.timeout)

    # Frontend endpoint
    def get_frontend_root(self) -> requests.Response:
        return self.session.get(FRONTEND_URL, timeout=self.timeout)


# ------------------------------------------------------------------------------
# WebSocket Test Client
# ------------------------------------------------------------------------------

class WebSocketTestClient:
    """Async & sync helper for testing WebSocket handshake and state streams."""

    def __init__(self, room_code: str, player_id: Optional[str] = None, timeout: float = TIMEOUT_SECONDS):
        self.room_code = room_code
        self.player_id = player_id
        self.timeout = timeout
        query = f"?player_id={player_id}" if player_id else ""
        self.uri = f"{GAME_ENGINE_WS_URL}/ws/{room_code}{query}"
        self.connection = None

    async def aconnect(self):
        self.connection = await websockets.connect(self.uri, timeout=self.timeout)
        return self

    async def asend(self, msg_type: str, payload: dict):
        envelope = {"type": msg_type, "payload": payload}
        await self.connection.send(json.dumps(envelope))

    async def areceive(self, timeout: Optional[float] = None) -> dict:
        t = timeout or self.timeout
        msg_raw = await asyncio.wait_for(self.connection.recv(), timeout=t)
        return json.loads(msg_raw)

    async def aclose(self):
        if self.connection:
            await self.connection.close()


# ------------------------------------------------------------------------------
# Robust Service Probes
# ------------------------------------------------------------------------------

class NetworkProbe:
    """Accurately probes whether JokerArena services are running."""

    @staticmethod
    def is_tcp_port_open(host: str, port: int, timeout: float = 1.0) -> bool:
        try:
            with socket.create_connection((host, port), timeout=timeout):
                return True
        except (socket.timeout, ConnectionRefusedError, OSError):
            return False

    @staticmethod
    def is_game_engine_live() -> bool:
        if is_mock_mode():
            return False
        try:
            res = requests.get(f"{GAME_ENGINE_URL}/health", timeout=1.0)
            if res.status_code == 200:
                data = res.json()
                return data.get("status") == "ok"
            return False
        except Exception:
            return False

    @staticmethod
    def is_telemetry_live() -> bool:
        if is_mock_mode():
            return False
        try:
            res = requests.get(f"{TELEMETRY_HTTP_URL}/health", timeout=1.0)
            if res.status_code == 200:
                data = res.json()
                return data.get("status") == "ok"
            return False
        except Exception:
            return False

    @staticmethod
    def is_frontend_live() -> bool:
        if is_mock_mode():
            return False
        try:
            res = requests.get(FRONTEND_URL, timeout=1.0)
            return res.status_code == 200 and "JokerArena" in res.text
        except Exception:
            return False

    @staticmethod
    def probe_postgres() -> bool:
        return NetworkProbe.is_tcp_port_open(TARGET_HOST, POSTGRES_PORT)

    @staticmethod
    def probe_mongodb() -> bool:
        return NetworkProbe.is_tcp_port_open(TARGET_HOST, MONGO_PORT)

    @staticmethod
    def probe_telemetry_grpc() -> bool:
        return NetworkProbe.is_tcp_port_open(TARGET_HOST, TELEMETRY_GRPC_PORT)


class PostgresTestClient:
    """Helper for executing relational queries against PostgreSQL."""

    def __init__(self):
        self.host = TARGET_HOST
        self.port = POSTGRES_PORT
        self.user = POSTGRES_USER
        self.password = POSTGRES_PASSWORD
        self.dbname = POSTGRES_DB

    def get_connection(self):
        try:
            import psycopg2
            return psycopg2.connect(
                host=self.host,
                port=self.port,
                user=self.user,
                password=self.password,
                dbname=self.dbname,
                connect_timeout=2
            )
        except Exception:
            return None

    def query(self, sql: str, params: tuple = None) -> List[tuple]:
        conn = self.get_connection()
        if not conn:
            return []
        try:
            with conn.cursor() as cur:
                cur.execute(sql, params or ())
                return cur.fetchall()
        except Exception:
            return []
        finally:
            conn.close()

    def is_live(self) -> bool:
        if is_mock_mode():
            return False
        try:
            rows = self.query("SELECT 1")
            return len(rows) > 0
        except Exception:
            return False

    def table_exists(self, table_name: str) -> bool:
        rows = self.query(
            "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name=%s",
            (table_name,)
        )
        return len(rows) > 0


# ------------------------------------------------------------------------------
# In-Memory Mock Server & Contract Simulation Harness
# ------------------------------------------------------------------------------

class MockE2EServer:
    """
    Standard-compliant in-memory server adhering strictly to PROJECT.md contracts.
    Used for verifying test suite logic, boundary models, and state machine transitions
    in isolation (--mock / --verify-syntax) before or alongside container boot.
    """

    def __init__(self):
        self.players: Dict[str, dict] = {
            "00000000-0000-0000-0000-000000000001": {
                "id": "00000000-0000-0000-0000-000000000001",
                "alias": "Bot-JokerBot",
                "total_wins": 0,
                "created_at": "2026-10-06T00:00:00Z"
            }
        }
        self.alias_to_id: Dict[str, str] = {"Bot-JokerBot": "00000000-0000-0000-0000-000000000001"}
        self.rooms: Dict[str, dict] = {}
        self.jokers: Dict[str, dict] = {
            "banner": {
                "id": "banner",
                "name": "Banner",
                "description": "+30 Chips por cada descarte restante en la mano",
                "modifier_type": "ADD_CHIPS",
                "default_val": 30.00,
                "apply_order": 1
            },
            "joker": {
                "id": "joker",
                "name": "Joker Clásico",
                "description": "+4 Mult fijo a toda mano jugada",
                "modifier_type": "ADD_MULT",
                "default_val": 4.00,
                "apply_order": 2
            },
            "the_duo": {
                "id": "the_duo",
                "name": "The Duo",
                "description": "x2 Mult exponencial si la mano contiene al menos un Par",
                "modifier_type": "X_MULT",
                "default_val": 2.00,
                "apply_order": 3
            }
        }
        self.telemetry_events: List[dict] = []

    def register_player(self, alias: str) -> Tuple[int, dict]:
        if not alias or len(alias.strip()) < 2 or len(alias) > 30:
            return 400, {"error": "Invalid alias length (2-30 chars required)"}
        if not re.match(r"^[a-zA-Z0-9_-]+$", alias):
            return 400, {"error": "Invalid alias characters"}
        if alias in self.alias_to_id:
            existing = self.players[self.alias_to_id[alias]]
            return 200, existing

        p_id = str(uuid.uuid4())
        record = {
            "id": p_id,
            "alias": alias,
            "total_wins": 0,
            "created_at": "2026-10-06T05:00:00Z"
        }
        self.players[p_id] = record
        self.alias_to_id[alias] = p_id
        return 201, record

    def create_room(self, player_id: str, target_score: int = 300, with_bot: bool = True) -> Tuple[int, dict]:
        if player_id not in self.players:
            return 400, {"error": "Unknown player_id"}

        code_suffix = uuid.uuid4().hex[:3].upper()
        room_code = f"JKR-{code_suffix}"
        match_id = str(uuid.uuid4())
        room = {
            "room_code": room_code,
            "match_id": match_id,
            "target_score": target_score,
            "status": "WAITING",
            "with_bot": with_bot,
            "hands_left": 4,
            "discards_left": 3,
            "current_score": 0,
            "deck_count": 44,
            "players": [self.players[player_id]],
            "active_jokers": ["banner", "joker", "the_duo"],
            "cards_in_hand": [
                {"rank": "A", "suit": "SPADES", "chips": 11},
                {"rank": "K", "suit": "HEARTS", "chips": 10},
                {"rank": "Q", "suit": "CLUBS", "chips": 10},
                {"rank": "J", "suit": "DIAMONDS", "chips": 10},
                {"rank": "10", "suit": "SPADES", "chips": 10},
                {"rank": "9", "suit": "HEARTS", "chips": 9},
                {"rank": "8", "suit": "CLUBS", "chips": 8},
                {"rank": "7", "suit": "DIAMONDS", "chips": 7}
            ]
        }
        if with_bot:
            room["players"].append(self.players["00000000-0000-0000-0000-000000000001"])
        self.rooms[room_code] = room
        return 201, {
            "room_code": room_code,
            "match_id": match_id,
            "status": "WAITING",
            "with_bot": with_bot
        }

    def get_room(self, code: str) -> Tuple[int, dict]:
        norm = code.upper()
        if norm in self.rooms:
            return 200, self.rooms[norm]
        return 404, {"error": "Room not found"}

    def simulate_play_hand(self, room_code: str, card_indices: List[int]) -> Tuple[int, dict]:
        if room_code not in self.rooms:
            return 404, {"error": "Room not found"}
        room = self.rooms[room_code]
        if len(card_indices) < 1 or len(card_indices) > 5:
            return 400, {"error": "Must select between 1 and 5 cards"}
        if len(set(card_indices)) != len(card_indices):
            return 400, {"error": "Duplicate card indices"}
        if any(i < 0 or i >= len(room["cards_in_hand"]) for i in card_indices):
            return 400, {"error": "Card index out of bounds"}
        if room["hands_left"] <= 0:
            return 400, {"error": "No hands remaining"}

        room["hands_left"] -= 1
        room["current_score"] += 80  # Mock score increase
        room["deck_count"] = max(0, room["deck_count"] - len(card_indices))
        return 200, {
            "type": "ROOM_STATE_UPDATE",
            "payload": {
                "room_code": room_code,
                "match_id": room["match_id"],
                "status": "IN_PROGRESS",
                "current_score": room["current_score"],
                "target_score": room["target_score"],
                "hands_left": room["hands_left"],
                "discards_left": room["discards_left"],
                "deck_count": room["deck_count"],
                "cards_in_hand": room["cards_in_hand"],
                "active_jokers": room["active_jokers"]
            }
        }
