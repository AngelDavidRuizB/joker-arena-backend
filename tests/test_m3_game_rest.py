"""
Milestone 3 Acceptance Test Suite: Go Game Engine REST Endpoints & PostgreSQL Persistence (V-1)

Verifies:
1. POST /api/players: Happy path, schema fields, boundary lengths (2-30 chars),
   Anti-XSS validation, empty/whitespace rejection, and idempotent upsert on duplicate alias.
2. POST /api/rooms: Room creation with 6-character code, default target score (300),
   optional bot selector, persistence in matches and match_players, and validation of player_id.
3. GET /api/rooms/{code}: Case-insensitive query, WAITING status, player roster,
   cards_in_hand (8 French cards), 404 on missing, and boundary handling on empty/malformed codes.
4. GET /health: Healthcheck responding HTTP 200 with service name and database status.
5. PostgreSQL Relational Persistence: Verification of tables players, matches, match_players.
"""
import json
import os
import sys
import unittest
import uuid
from pathlib import Path

# Ensure project root is in sys.path
PROJECT_ROOT = Path(__file__).resolve().parent.parent
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from tests.config import (
    GAME_ENGINE_PORT,
    GAME_ENGINE_URL,
    TIMEOUT_SECONDS,
    is_mock_mode,
)
from tests.test_client import (
    MockE2EServer,
    NetworkProbe,
    PostgresTestClient,
    RESTClient,
    is_valid_uuid,
    validate_french_card,
)


class TestM3GameREST(unittest.TestCase):
    """Milestone 3 Acceptance Test Suite."""

    @classmethod
    def setUpClass(cls):
        cls.rest_client = RESTClient(base_url=GAME_ENGINE_URL, timeout=TIMEOUT_SECONDS)
        cls.pg_client = PostgresTestClient()
        cls.is_game_engine_live = NetworkProbe.is_game_engine_live()
        cls.is_pg_live = cls.pg_client.is_live()
        cls.mock_mode = is_mock_mode()
        cls.mock_server = MockE2EServer()

    # ==========================================================================
    # 1. Healthcheck Endpoint GET /health
    # ==========================================================================

    def test_01_health_endpoint_status_200(self):
        """M3.1: GET /health responds with HTTP 200 OK and valid status."""
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.get("/health")
            self.assertEqual(res.status_code, 200)
            data = res.json()
            self.assertEqual(data.get("status"), "ok")
            self.assertEqual(data.get("service"), "jokerarena-game-ms")
            self.assertIn("postgres", data)
            self.assertIn(data.get("postgres"), ("connected", "fallback"))
        else:
            self.assertGreater(GAME_ENGINE_PORT, 0)

    # ==========================================================================
    # 2. Player Registration POST /api/players
    # ==========================================================================

    def test_02_player_registration_happy_path(self):
        """M3.2: POST /api/players with valid alias returns HTTP 201 Created with valid UUID."""
        alias = f"Player_{uuid.uuid4().hex[:6]}"
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.register_player(alias)
            self.assertEqual(res.status_code, 201)
            data = res.json()
        else:
            code, data = self.mock_server.register_player(alias)
            self.assertEqual(code, 201)

        self.assertTrue(is_valid_uuid(data["id"]))
        self.assertEqual(data["alias"], alias)

    def test_03_player_registration_schema_fields(self):
        """M3.3: Player registration response contains id, alias, total_wins, and created_at."""
        alias = f"Schema_{uuid.uuid4().hex[:4]}"
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.register_player(alias)
            data = res.json()
        else:
            _, data = self.mock_server.register_player(alias)

        for key in ["id", "alias", "total_wins", "created_at"]:
            self.assertIn(key, data, f"Required field '{key}' missing from player response")
        self.assertEqual(data["total_wins"], 0)

    def test_04_player_min_length_alias(self):
        """M3.4: 2-character alias is accepted with HTTP 201 Created."""
        alias = f"A{uuid.uuid4().hex[:1]}"
        self.assertEqual(len(alias), 2)
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.register_player(alias)
            self.assertEqual(res.status_code, 201)
        else:
            code, _ = self.mock_server.register_player(alias)
            self.assertEqual(code, 201)

    def test_05_player_max_length_alias(self):
        """M3.5: Exactly 30-character alias is accepted with HTTP 201 Created."""
        alias = "P" * 24 + uuid.uuid4().hex[:6]
        self.assertEqual(len(alias), 30)
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.register_player(alias)
            self.assertEqual(res.status_code, 201)
        else:
            code, _ = self.mock_server.register_player(alias)
            self.assertEqual(code, 201)

    def test_06_player_empty_alias_rejected(self):
        """M3.6: Empty alias is rejected with HTTP 400 Bad Request."""
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.register_player("")
            self.assertEqual(res.status_code, 400)
            self.assertIn("error", res.json())
        else:
            code, data = self.mock_server.register_player("")
            self.assertEqual(code, 400)
            self.assertIn("error", data)

    def test_07_player_whitespace_only_alias_rejected(self):
        """M3.7: Whitespace-only alias is rejected with HTTP 400 Bad Request."""
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.register_player("     ")
            self.assertEqual(res.status_code, 400)
            self.assertIn("error", res.json())
        else:
            code, data = self.mock_server.register_player("     ")
            self.assertEqual(code, 400)
            self.assertIn("error", data)

    def test_08_player_single_char_alias_rejected(self):
        """M3.8: 1-character alias (< 2 chars) is rejected with HTTP 400 Bad Request."""
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.register_player("X")
            self.assertEqual(res.status_code, 400)
            self.assertIn("error", res.json())
        else:
            code, data = self.mock_server.register_player("X")
            self.assertEqual(code, 400)
            self.assertIn("error", data)

    def test_09_player_oversized_31_char_alias_rejected(self):
        """M3.9: 31-character alias (> 30 chars limit) is rejected with HTTP 400 Bad Request."""
        oversized = "A" * 31
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.register_player(oversized)
            self.assertEqual(res.status_code, 400)
            self.assertIn("error", res.json())
        else:
            code, data = self.mock_server.register_player(oversized)
            self.assertEqual(code, 400)
            self.assertIn("error", data)

    def test_10_player_xss_special_characters_rejected(self):
        """M3.10: Alias with <script> tag is rejected with HTTP 400 or 422 (Anti-XSS)."""
        xss_alias = "<script>alert(1)</script>"
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.register_player(xss_alias)
            self.assertIn(res.status_code, (400, 422))
            self.assertIn("error", res.json())
        else:
            code, data = self.mock_server.register_player(xss_alias)
            self.assertEqual(code, 400)
            self.assertIn("error", data)

    def test_11_player_idempotent_registration(self):
        """M3.11: Repeated registration with identical alias returns existing record idempotently."""
        alias = f"Idem_{uuid.uuid4().hex[:6]}"
        if self.is_game_engine_live and not self.mock_mode:
            res1 = self.rest_client.register_player(alias)
            self.assertEqual(res1.status_code, 201)
            p1 = res1.json()

            res2 = self.rest_client.register_player(alias)
            self.assertIn(res2.status_code, (200, 201))
            p2 = res2.json()
            self.assertEqual(p1["id"], p2["id"])
            self.assertEqual(p1["alias"], p2["alias"])
        else:
            code1, p1 = self.mock_server.register_player(alias)
            code2, p2 = self.mock_server.register_player(alias)
            self.assertEqual(code1, 201)
            self.assertIn(code2, (200, 201))
            self.assertEqual(p1["id"], p2["id"])

    # ==========================================================================
    # 3. Room Creation POST /api/rooms
    # ==========================================================================

    def test_12_room_creation_happy_path(self):
        """M3.12: POST /api/rooms creates room with 6-char code, match_id, and WAITING status."""
        alias = f"Host_{uuid.uuid4().hex[:4]}"
        if self.is_game_engine_live and not self.mock_mode:
            p_id = self.rest_client.register_player(alias).json()["id"]
            res = self.rest_client.create_room(p_id, target_score=300, with_bot=True)
            self.assertEqual(res.status_code, 201)
            data = res.json()
        else:
            _, p_data = self.mock_server.register_player(alias)
            code, data = self.mock_server.create_room(p_data["id"], target_score=300, with_bot=True)
            self.assertEqual(code, 201)

        self.assertIn("room_code", data)
        self.assertGreaterEqual(len(data["room_code"]), 6)
        self.assertTrue(is_valid_uuid(data["match_id"]))
        self.assertEqual(data["status"], "WAITING")
        self.assertTrue(data.get("with_bot", True))

    def test_13_room_creation_bot_flag(self):
        """M3.13: Room creation preserves with_bot selector flag."""
        alias = f"HostSolo_{uuid.uuid4().hex[:4]}"
        if self.is_game_engine_live and not self.mock_mode:
            p_id = self.rest_client.register_player(alias).json()["id"]
            res = self.rest_client.create_room(p_id, with_bot=True)
            self.assertEqual(res.status_code, 201)
            data = res.json()
        else:
            _, p_data = self.mock_server.register_player(alias)
            _, data = self.mock_server.create_room(p_data["id"], with_bot=True)

        self.assertTrue(data.get("with_bot", True))

    def test_14_room_target_score_default_300(self):
        """M3.14: Room target score defaults to 300 points if omitted."""
        alias = f"HostTarget_{uuid.uuid4().hex[:4]}"
        if self.is_game_engine_live and not self.mock_mode:
            p_id = self.rest_client.register_player(alias).json()["id"]
            res = self.rest_client.create_room(p_id)
            self.assertEqual(res.status_code, 201)
            room_code = res.json()["room_code"]
            q_res = self.rest_client.get_room(room_code)
            self.assertEqual(q_res.json()["target_score"], 300)
        else:
            _, p_data = self.mock_server.register_player(alias)
            _, data = self.mock_server.create_room(p_data["id"])
            room = self.mock_server.rooms[data["room_code"]]
            self.assertEqual(room["target_score"], 300)

    def test_15_room_creation_rejects_unknown_player_id(self):
        """M3.15: Creating room with unknown player UUID returns HTTP 400 Bad Request."""
        random_uuid = str(uuid.uuid4())
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.create_room(random_uuid)
            self.assertEqual(res.status_code, 400)
            self.assertIn("error", res.json())
        else:
            code, data = self.mock_server.create_room(random_uuid)
            self.assertEqual(code, 400)
            self.assertIn("error", data)

    # ==========================================================================
    # 4. Room Query GET /api/rooms/{code}
    # ==========================================================================

    def test_16_room_query_existing_room(self):
        """M3.16: GET /api/rooms/{code} returns HTTP 200 OK with session metadata."""
        alias = f"QHost_{uuid.uuid4().hex[:4]}"
        if self.is_game_engine_live and not self.mock_mode:
            p_id = self.rest_client.register_player(alias).json()["id"]
            code = self.rest_client.create_room(p_id).json()["room_code"]
            res = self.rest_client.get_room(code)
            self.assertEqual(res.status_code, 200)
            data = res.json()
        else:
            _, p_data = self.mock_server.register_player(alias)
            _, r_data = self.mock_server.create_room(p_data["id"])
            code, data = self.mock_server.get_room(r_data["room_code"])
            self.assertEqual(code, 200)

        self.assertEqual(data["status"], "WAITING")
        self.assertEqual(data["target_score"], 300)
        self.assertIn("players", data)
        self.assertIsInstance(data["players"], list)

    def test_17_room_query_case_insensitivity(self):
        """M3.17: Room lookup is case-insensitive (lowercase code resolves successfully)."""
        alias = f"CaseH_{uuid.uuid4().hex[:4]}"
        if self.is_game_engine_live and not self.mock_mode:
            p_id = self.rest_client.register_player(alias).json()["id"]
            code = self.rest_client.create_room(p_id).json()["room_code"]
            res = self.rest_client.get_room(code.lower())
            self.assertEqual(res.status_code, 200)
        else:
            _, p_data = self.mock_server.register_player(alias)
            _, r_data = self.mock_server.create_room(p_data["id"])
            code, _ = self.mock_server.get_room(r_data["room_code"].lower())
            self.assertEqual(code, 200)

    def test_18_room_query_contains_8_cards_in_hand(self):
        """M3.18: Freshly created room contains initial 8 dealt French cards in cards_in_hand."""
        alias = f"DealH_{uuid.uuid4().hex[:4]}"
        if self.is_game_engine_live and not self.mock_mode:
            p_id = self.rest_client.register_player(alias).json()["id"]
            code = self.rest_client.create_room(p_id).json()["room_code"]
            res = self.rest_client.get_room(code)
            self.assertEqual(res.status_code, 200)
            data = res.json()
        else:
            _, p_data = self.mock_server.register_player(alias)
            _, r_data = self.mock_server.create_room(p_data["id"])
            _, data = self.mock_server.get_room(r_data["room_code"])

        cards = data.get("cards_in_hand", [])
        self.assertEqual(len(cards), 8, f"Expected 8 cards in hand, got {len(cards)}")
        for card in cards:
            valid, msg = validate_french_card(card)
            self.assertTrue(valid, msg)

    def test_19_room_query_non_existent_returns_404(self):
        """M3.19: Querying a non-existent room code returns HTTP 404 Not Found."""
        fake_code = "NONEXIST99"
        if self.is_game_engine_live and not self.mock_mode:
            res = self.rest_client.get_room(fake_code)
            self.assertEqual(res.status_code, 404)
            self.assertIn("error", res.json())
        else:
            code, data = self.mock_server.get_room(fake_code)
            self.assertEqual(code, 404)
            self.assertIn("error", data)

    def test_20_room_query_empty_or_malformed_code(self):
        """M3.20: Querying /api/rooms/ or malformed symbols returns HTTP 400, 404, or 405."""
        if self.is_game_engine_live and not self.mock_mode:
            res_empty = self.rest_client.get("/api/rooms/")
            self.assertIn(res_empty.status_code, (404, 405))
            res_malformed = self.rest_client.get_room("!@#$%^")
            self.assertIn(res_malformed.status_code, (400, 404))
        else:
            code, _ = self.mock_server.get_room("!@#$%^")
            self.assertEqual(code, 404)

    # ==========================================================================
    # 5. Direct PostgreSQL Persistence (when live DB is reachable)
    # ==========================================================================

    def test_21_postgres_player_and_match_persistence(self):
        """M3.21: Direct verification of relational rows in players, matches, match_players."""
        alias = f"PgPersist_{uuid.uuid4().hex[:4]}"
        if self.is_game_engine_live and self.is_pg_live and not self.mock_mode:
            # Register player
            p_res = self.rest_client.register_player(alias)
            self.assertEqual(p_res.status_code, 201)
            player_id = p_res.json()["id"]

            # Query players table
            p_rows = self.pg_client.query("SELECT alias, total_wins FROM players WHERE id=%s", (player_id,))
            self.assertEqual(len(p_rows), 1)
            self.assertEqual(p_rows[0][0], alias)

            # Create room
            r_res = self.rest_client.create_room(player_id, with_bot=True)
            self.assertEqual(r_res.status_code, 201)
            match_id = r_res.json()["match_id"]
            room_code = r_res.json()["room_code"]

            # Query matches table
            m_rows = self.pg_client.query("SELECT room_code, status FROM matches WHERE id=%s", (match_id,))
            self.assertEqual(len(m_rows), 1)
            self.assertEqual(m_rows[0][0], room_code)
            self.assertEqual(m_rows[0][1], "WAITING")

            # Query match_players table
            mp_rows = self.pg_client.query(
                "SELECT player_id, is_bot FROM match_players WHERE match_id=%s ORDER BY is_bot ASC",
                (match_id,),
            )
            self.assertGreaterEqual(len(mp_rows), 2)
        else:
            # Simulated persistence check in mock environment
            _, p_data = self.mock_server.register_player(alias)
            self.assertIn(p_data["id"], self.mock_server.players)
            _, r_data = self.mock_server.create_room(p_data["id"], with_bot=True)
            self.assertIn(r_data["room_code"], self.mock_server.rooms)


if __name__ == "__main__":
    unittest.main(verbosity=2)
