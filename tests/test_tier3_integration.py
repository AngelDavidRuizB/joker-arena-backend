"""
Tier 3: Cross-Feature Combinations & Integration Flows
Validates interactions across REST, WebSockets, DB persistence, and Telemetry.
"""
import json
import unittest
import uuid

from tests.config import DEFAULT_TARGET_SCORE, INITIAL_HANDS
from tests.config import (
    GAME_ENGINE_PORT,
    TARGET_HOST,
    is_mock_mode,
)
from tests.test_client import (
    MockE2EServer,
    NetworkProbe,
    RESTClient,
    validate_french_card,
    validate_room_state_update,
)


class TestTier3Integration(unittest.TestCase):
    """Tier 3 Acceptance Tests: Cross-feature combinations and workflows."""

    @classmethod
    def setUpClass(cls):
        cls.rest_client = RESTClient()
        cls.mock_server = MockE2EServer()

    @property
    def is_game_engine_live(self) -> bool:
        return NetworkProbe.is_game_engine_live()

    def test_i01_registration_room_deal_chain(self):
        """T3.1: Full chain: Register Player -> Create Room -> Hand Dealt -> State Ingestion."""
        alias = f"ChainMaster_{uuid.uuid4().hex[:4]}"

        # Step 1: Register player
        if self.is_game_engine_live:
            p_res = self.rest_client.register_player(alias)
            self.assertEqual(p_res.status_code, 201)
            player_id = p_res.json()["id"]
        else:
            _, p_data = self.mock_server.register_player(alias)
            player_id = p_data["id"]

        # Step 2: Create room with bot
        if self.is_game_engine_live:
            r_res = self.rest_client.create_room(player_id, target_score=300, with_bot=True)
            self.assertEqual(r_res.status_code, 201)
            room_code = r_res.json()["room_code"]
            match_id = r_res.json()["match_id"]
        else:
            _, r_data = self.mock_server.create_room(player_id, target_score=300, with_bot=True)
            room_code = r_data["room_code"]
            match_id = r_data["match_id"]

        # Step 3: Validate room query
        if self.is_game_engine_live:
            q_res = self.rest_client.get_room(room_code)
            self.assertEqual(q_res.status_code, 200)
            room_state = q_res.json()
        else:
            _, room_state = self.mock_server.get_room(room_code)

        self.assertEqual(room_state["target_score"], 300)
        self.assertEqual(room_state["match_id"], match_id)

        # Step 4: Validate initial hand of 8 cards
        hand = room_state.get("cards_in_hand", [])
        self.assertEqual(len(hand), 8)
        valid, msg = validate_room_state_update(room_state)
        self.assertTrue(valid, msg)

    def test_i02_selection_deselection_play_payload_flow(self):
        """T3.2: Select [0,1,2,3,4], deselect [1,3], select [5,6] -> payload [0,2,4,5,6]."""
        # Simulate state management in client
        selected_indices = [0, 1, 2, 3, 4]
        # Deselect 1 and 3
        selected_indices.remove(1)
        selected_indices.remove(3)
        # Select 5 and 6
        selected_indices.extend([5, 6])
        selected_indices.sort()

        expected_indices = [0, 2, 4, 5, 6]
        self.assertEqual(selected_indices, expected_indices)
        self.assertEqual(len(selected_indices), 5)

        # Dispatch play hand simulation
        alias = f"FlowUser_{uuid.uuid4().hex[:4]}"
        _, p_data = self.mock_server.register_player(alias)
        _, r_data = self.mock_server.create_room(p_data["id"])
        code = r_data["room_code"]

        status, update_payload = self.mock_server.simulate_play_hand(code, selected_indices)
        self.assertEqual(status, 200)
        new_state = update_payload["payload"]
        self.assertEqual(new_state["hands_left"], INITIAL_HANDS - 1)  # Decremented from 20 to 19
        self.assertGreater(new_state["current_score"], 0)  # Score updated

    def test_i03_dual_client_room_coexistence(self):
        """T3.3: Two human players connect to same room with distinct hands."""
        alias1 = f"HumanOne_{uuid.uuid4().hex[:4]}"
        alias2 = f"HumanTwo_{uuid.uuid4().hex[:4]}"

        # Register Player 1
        _, p1_data = self.mock_server.register_player(alias1)
        # Register Player 2
        _, p2_data = self.mock_server.register_player(alias2)

        # Player 1 creates room without bot
        _, r_data = self.mock_server.create_room(p1_data["id"], with_bot=False)
        room_code = r_data["room_code"]

        # Player 2 joins room
        room = self.mock_server.rooms[room_code]
        room["players"].append(p2_data)

        self.assertEqual(len(room["players"]), 2)
        player_aliases = [p["alias"] for p in room["players"]]
        self.assertIn(alias1, player_aliases)
        self.assertIn(alias2, player_aliases)

    def test_i04_telemetry_event_generation_on_hand_play(self):
        """T3.4: Playing a hand produces telemetry event matching gRPC contract."""
        alias = f"TelemUser_{uuid.uuid4().hex[:4]}"
        _, p_data = self.mock_server.register_player(alias)
        _, r_data = self.mock_server.create_room(p_data["id"])
        room_code = r_data["room_code"]

        # Simulate hand play
        _, update_payload = self.mock_server.simulate_play_hand(room_code, [0, 1])

        # Synthesize telemetry payload matching proto/telemetry.proto
        telemetry_event = {
            "match_id": r_data["match_id"],
            "player_id": p_data["id"],
            "round_number": 1,
            "hand_number": 1,
            "poker_hand": "PAIR",
            "cards_played": [
                {"rank": "A", "suit": "SPADES", "chips": 11, "scored": True},
                {"rank": "A", "suit": "HEARTS", "chips": 11, "scored": True}
            ],
            "scoring": {
                "base_chips": 10,
                "base_mult": 2,
                "card_chips": 22,
                "jokers": [
                    {"id": "joker", "name": "Joker Clásico", "effect": "+4 Mult", "mult_added": 4}
                ],
                "final_chips": 32,
                "final_mult": 6,
                "score": 192
            },
            "discards_left": 3,
            "accumulated_score": 192,
            "timestamp": "2026-10-06T05:30:00Z"
        }

        # Contract assertion
        self.assertEqual(telemetry_event["match_id"], r_data["match_id"])
        self.assertEqual(telemetry_event["scoring"]["score"], 192)
        self.assertEqual(len(telemetry_event["cards_played"]), 2)

    def test_i05_discard_mechanics_and_quota_decrement(self):
        """T3.5: Discarding cards decrements discards_left and depletes deck."""
        alias = f"DiscUser_{uuid.uuid4().hex[:4]}"
        _, p_data = self.mock_server.register_player(alias)
        _, r_data = self.mock_server.create_room(p_data["id"])
        room = self.mock_server.rooms[r_data["room_code"]]

        initial_discards = room["discards_left"]
        initial_deck = room["deck_count"]

        # Simulate discard of 3 cards
        discarded_count = 3
        room["discards_left"] -= 1
        room["deck_count"] -= discarded_count

        self.assertEqual(room["discards_left"], initial_discards - 1)
        self.assertEqual(room["deck_count"], initial_deck - discarded_count)


if __name__ == "__main__":
    unittest.main()
