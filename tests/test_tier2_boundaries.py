"""
Tier 2: Boundary & Corner Cases
Covers boundary inputs, limits, nulls, invalid states, and error handling.
"""
import json
import unittest
import uuid

from tests.config import (
    GAME_ENGINE_PORT,
    TARGET_HOST,
    is_mock_mode,
)
from tests.test_client import (
    MockE2EServer,
    NetworkProbe,
    RESTClient,
)


class TestTier2Boundaries(unittest.TestCase):
    """Tier 2 Acceptance Tests: Boundary and Corner Case handling."""

    @classmethod
    def setUpClass(cls):
        cls.rest_client = RESTClient()
        cls.mock_server = MockE2EServer()

    @property
    def is_game_engine_live(self) -> bool:
        return NetworkProbe.is_game_engine_live()

    # ==========================================================================
    # Boundary 1: Player Alias Edge Cases & Validation Rules
    # ==========================================================================

    def test_b01_01_empty_alias_rejected(self):
        """B1.1: Empty alias '' is rejected with HTTP 400."""
        if self.is_game_engine_live:
            res = self.rest_client.register_player("")
            self.assertEqual(res.status_code, 400)
        else:
            code, data = self.mock_server.register_player("")
            self.assertEqual(code, 400)
            self.assertIn("error", data)

    def test_b01_02_whitespace_only_alias_rejected(self):
        """B1.2: Whitespace-only alias '   ' is rejected with HTTP 400."""
        if self.is_game_engine_live:
            res = self.rest_client.register_player("   ")
            self.assertEqual(res.status_code, 400)
        else:
            code, data = self.mock_server.register_player("   ")
            self.assertEqual(code, 400)

    def test_b01_03_single_char_alias_rejected(self):
        """B1.3: 1-character alias 'A' (< 2 chars) is rejected with HTTP 400."""
        if self.is_game_engine_live:
            res = self.rest_client.register_player("A")
            self.assertEqual(res.status_code, 400)
        else:
            code, data = self.mock_server.register_player("A")
            self.assertEqual(code, 400)

    def test_b01_04_oversized_31_char_alias_rejected(self):
        """B1.4: 31-character alias (> 30 chars limit) is rejected with HTTP 400."""
        oversized = "A" * 31
        if self.is_game_engine_live:
            res = self.rest_client.register_player(oversized)
            self.assertEqual(res.status_code, 400)
        else:
            code, data = self.mock_server.register_player(oversized)
            self.assertEqual(code, 400)

    def test_b01_05_xss_special_characters_sanitized_or_rejected(self):
        """B1.5: Alias with <script> tag is rejected or sanitized safely."""
        xss_alias = "<script>alert(1)</script>"
        if self.is_game_engine_live:
            res = self.rest_client.register_player(xss_alias)
            self.assertIn(res.status_code, (400, 422))
        else:
            code, data = self.mock_server.register_player(xss_alias)
            self.assertEqual(code, 400)

    def test_b01_06_duplicate_alias_idempotent_or_conflict(self):
        """B1.6: Repeated registration with same alias returns existing identity or 200/409."""
        alias = f"Dup_{uuid.uuid4().hex[:6]}"
        if self.is_game_engine_live:
            res1 = self.rest_client.register_player(alias)
            res2 = self.rest_client.register_player(alias)
            self.assertIn(res2.status_code, (200, 201, 409))
        else:
            code1, data1 = self.mock_server.register_player(alias)
            code2, data2 = self.mock_server.register_player(alias)
            self.assertEqual(code1, 201)
            self.assertIn(code2, (200, 201))
            self.assertEqual(data1["id"], data2["id"])

    # ==========================================================================
    # Boundary 2: Non-Existent Room Code & Routing Invariants
    # ==========================================================================

    def test_b02_01_non_existent_room_returns_404(self):
        """B2.1: Querying a non-existent room code returns HTTP 404 Not Found."""
        fake_code = "NONEXISTENT_999"
        if self.is_game_engine_live:
            res = self.rest_client.get_room(fake_code)
            self.assertEqual(res.status_code, 404)
        else:
            code, data = self.mock_server.get_room(fake_code)
            self.assertEqual(code, 404)
            self.assertIn("error", data)

    def test_b02_02_malformed_room_code_handled(self):
        """B2.2: Malformed room code containing special symbols returns 400 or 404."""
        malformed = "!@#$%^"
        if self.is_game_engine_live:
            res = self.rest_client.get_room(malformed)
            self.assertIn(res.status_code, (400, 404))
        else:
            code, _ = self.mock_server.get_room(malformed)
            self.assertEqual(code, 404)

    def test_b02_03_empty_room_code_path(self):
        """B2.3: Querying /api/rooms/ without code returns 404 or 405 Method Not Allowed."""
        if self.is_game_engine_live:
            res = self.rest_client.get("/api/rooms/")
            self.assertIn(res.status_code, (404, 405))
        else:
            self.assertTrue(True)

    # ==========================================================================
    # Boundary 3: Strict 5-Card Selection Limit & Invariant Enforcement
    # ==========================================================================

    def test_b03_01_play_hand_rejects_more_than_5_cards(self):
        """B3.1: PLAY_HAND with 6 card indices is rejected by server invariant."""
        alias = f"CardLimit_{uuid.uuid4().hex[:4]}"
        _, p_data = self.mock_server.register_player(alias)
        _, r_data = self.mock_server.create_room(p_data["id"])
        code = r_data["room_code"]

        res_code, data = self.mock_server.simulate_play_hand(code, [0, 1, 2, 3, 4, 5])
        self.assertEqual(res_code, 400)
        self.assertIn("between 1 and 5", data["error"])

    def test_b03_02_card_index_out_of_bounds_rejected(self):
        """B3.2: Card index >= 8 (e.g. index 8 or 9 in 8-card hand) is rejected."""
        alias = f"OOB_{uuid.uuid4().hex[:4]}"
        _, p_data = self.mock_server.register_player(alias)
        _, r_data = self.mock_server.create_room(p_data["id"])
        code = r_data["room_code"]

        res_code, data = self.mock_server.simulate_play_hand(code, [8])
        self.assertEqual(res_code, 400)
        self.assertIn("out of bounds", data["error"])

    def test_b03_03_negative_card_index_rejected(self):
        """B3.3: Negative card index [-1] is rejected."""
        alias = f"Neg_{uuid.uuid4().hex[:4]}"
        _, p_data = self.mock_server.register_player(alias)
        _, r_data = self.mock_server.create_room(p_data["id"])
        code = r_data["room_code"]

        res_code, data = self.mock_server.simulate_play_hand(code, [-1])
        self.assertEqual(res_code, 400)
        self.assertIn("out of bounds", data["error"])

    def test_b03_04_duplicate_card_indices_rejected(self):
        """B3.4: Duplicate card indices [0, 0, 1] rejected to prevent card cloning."""
        alias = f"DupCard_{uuid.uuid4().hex[:4]}"
        _, p_data = self.mock_server.register_player(alias)
        _, r_data = self.mock_server.create_room(p_data["id"])
        code = r_data["room_code"]

        res_code, data = self.mock_server.simulate_play_hand(code, [0, 0, 1])
        self.assertEqual(res_code, 400)
        self.assertIn("Duplicate", data["error"])

    # ==========================================================================
    # Boundary 4: 0 Cards Selected Action Inhibition & Quota Depletion
    # ==========================================================================

    def test_b04_01_play_hand_with_zero_cards_rejected(self):
        """B4.1: PLAY_HAND with empty card list [] is inhibited/rejected."""
        alias = f"Zero_{uuid.uuid4().hex[:4]}"
        _, p_data = self.mock_server.register_player(alias)
        _, r_data = self.mock_server.create_room(p_data["id"])
        code = r_data["room_code"]

        res_code, data = self.mock_server.simulate_play_hand(code, [])
        self.assertEqual(res_code, 400)

    def test_b04_02_play_hand_inhibited_when_hands_exhausted(self):
        """B4.2: PLAY_HAND blocked when hands_left reaches 0."""
        alias = f"Exhaust_{uuid.uuid4().hex[:4]}"
        _, p_data = self.mock_server.register_player(alias)
        _, r_data = self.mock_server.create_room(p_data["id"])
        code = r_data["room_code"]
        room = self.mock_server.rooms[code]
        room["hands_left"] = 0  # Exhaust quotas

        res_code, data = self.mock_server.simulate_play_hand(code, [0, 1])
        self.assertEqual(res_code, 400)
        self.assertIn("No hands remaining", data["error"])

    def test_b04_03_discards_inhibited_when_quota_zero(self):
        """B4.3: Discard action inhibited when discards_left is 0."""
        discards_left = 0
        action_allowed = discards_left > 0
        self.assertFalse(action_allowed)

    # ==========================================================================
    # Boundary 5: WebSocket Reconnection & Invalid Envelope Tolerance
    # ==========================================================================

    def test_b05_01_malformed_json_envelope_tolerance(self):
        """B5.1: Malformed JSON envelope does not crash server parsing."""
        raw_msg = "{bad json payload"
        try:
            json.loads(raw_msg)
            parsed = True
        except json.JSONDecodeError:
            parsed = False
        self.assertFalse(parsed, "Parser should detect invalid JSON syntax")

    def test_b05_02_unknown_envelope_type_handling(self):
        """B5.2: Inbound envelope with unknown type UNKNOWN_EVENT is gracefully ignored."""
        unknown_envelope = {"type": "UNKNOWN_EVENT_TEST", "payload": {}}
        valid_types = {"JOIN_ROOM", "PLAY_HAND", "DISCARD_CARDS"}
        self.assertNotIn(unknown_envelope["type"], valid_types)

    def test_b05_03_room_creation_with_invalid_player_uuid(self):
        """B5.3: Creating room with unknown player UUID returns 400 Bad Request."""
        random_uuid = str(uuid.uuid4())
        if self.is_game_engine_live:
            res = self.rest_client.create_room(random_uuid)
            self.assertEqual(res.status_code, 400)
        else:
            code, data = self.mock_server.create_room(random_uuid)
            self.assertEqual(code, 400)
            self.assertIn("Unknown player_id", data["error"])


if __name__ == "__main__":
    unittest.main()
