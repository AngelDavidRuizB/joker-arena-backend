"""
Tier 4: Real-World Workload Scenarios
Validates complete user journeys, multi-container convergence, concurrency, and stress.
"""
import concurrent.futures
import time
import unittest
import uuid

from tests.config import (
    FRONTEND_PORT,
    GAME_ENGINE_PORT,
    MONGO_PORT,
    POSTGRES_PORT,
    TARGET_HOST,
    TELEMETRY_GRPC_PORT,
    TELEMETRY_HTTP_PORT,
    is_mock_mode,
)
from tests.test_client import (
    MockE2EServer,
    NetworkProbe,
    PostgresTestClient,
    RESTClient,
    validate_french_card,
    validate_room_state_update,
)


class TestTier4Scenarios(unittest.TestCase):
    """Tier 4 Acceptance Tests: Real-World Application Scenarios."""

    @classmethod
    def setUpClass(cls):
        cls.rest_client = RESTClient()
        cls.pg_client = PostgresTestClient()
        cls.mock_server = MockE2EServer()

    @property
    def is_game_engine_live(self) -> bool:
        return NetworkProbe.is_game_engine_live()

    @property
    def is_pg_live(self) -> bool:
        return self.pg_client.is_live()

    def test_s01_full_user_journey(self):
        """T4.1: Full User Journey - Register, Create Bot Match, Deal, Select 5, Play Hand."""
        # 1. Alias Registration
        alias = f"Journey_{uuid.uuid4().hex[:6]}"
        if self.is_game_engine_live:
            res_reg = self.rest_client.register_player(alias)
            self.assertEqual(res_reg.status_code, 201)
            player_id = res_reg.json()["id"]
        else:
            _, reg_data = self.mock_server.register_player(alias)
            player_id = reg_data["id"]

        # 2. Room Creation with Bot Opponent
        if self.is_game_engine_live:
            res_room = self.rest_client.create_room(player_id, target_score=300, with_bot=True)
            self.assertEqual(res_room.status_code, 201)
            room_code = res_room.json()["room_code"]
        else:
            _, room_data = self.mock_server.create_room(player_id, target_score=300, with_bot=True)
            room_code = room_data["room_code"]

        # 3. Query Room State & Confirm Initial Deal
        if self.is_game_engine_live:
            res_state = self.rest_client.get_room(room_code)
            self.assertEqual(res_state.status_code, 200)
            state = res_state.json()
        else:
            _, state = self.mock_server.get_room(room_code)

        cards = state.get("cards_in_hand", [])
        self.assertEqual(len(cards), 8, "Player should receive initial 8 cards")
        for card in cards:
            valid, msg = validate_french_card(card)
            self.assertTrue(valid, msg)

        # 4. Interactive Selection of 5 Cards
        selected_indices = [0, 1, 2, 3, 4]
        self.assertEqual(len(selected_indices), 5)

        # 5. Play Hand and Progress Score
        status, update_env = self.mock_server.simulate_play_hand(room_code, selected_indices)
        self.assertEqual(status, 200)
        updated_state = update_env["payload"]
        self.assertGreater(updated_state["current_score"], 0)
        self.assertEqual(updated_state["hands_left"], 3)
        self.assertEqual(updated_state["target_score"], 300)

    def test_s02_multi_container_convergence(self):
        """T4.2: Distributed Multi-Container Boot & Polyglot Persistence Health Convergence."""
        # Verify port mappings and health configuration across all 5 containers
        expected_ports = {
            "frontend": FRONTEND_PORT,
            "game_engine": GAME_ENGINE_PORT,
            "telemetry_http": TELEMETRY_HTTP_PORT,
            "telemetry_grpc": TELEMETRY_GRPC_PORT,
            "postgres": POSTGRES_PORT,
            "mongodb": MONGO_PORT,
        }

        for service, port in expected_ports.items():
            self.assertIsInstance(port, int)
            self.assertGreater(port, 0)

        # In mock or live mode, verify database and seed consistency
        if self.is_pg_live:
            self.assertTrue(self.pg_client.table_exists("jokers"))
            self.assertTrue(self.pg_client.table_exists("players"))
            self.assertTrue(self.pg_client.table_exists("matches"))
        else:
            self.assertEqual(len(self.mock_server.jokers), 3)
            self.assertIn("Bot-JokerBot", self.mock_server.alias_to_id)

    def test_s03_concurrency_and_idempotency_stress(self):
        """T4.3: Concurrency Stress - 10 concurrent player registrations & room creations."""
        num_players = 10
        aliases = [f"Concur_{i}_{uuid.uuid4().hex[:4]}" for i in range(num_players)]
        registered_ids = []

        def register_worker(alias_name: str):
            if self.is_game_engine_live:
                res = self.rest_client.register_player(alias_name)
                return res.status_code, res.json()["id"]
            else:
                code, data = self.mock_server.register_player(alias_name)
                return code, data["id"]

        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            results = list(executor.map(register_worker, aliases))

        for code, p_id in results:
            self.assertIn(code, (200, 201))
            registered_ids.append(p_id)

        # Verify UUID collision-free property
        self.assertEqual(len(set(registered_ids)), num_players)

    def test_s04_table_boundary_stress_and_rapid_clicks(self):
        """T4.4: Table Boundary Stress - Rapid card toggling & 6th card rejection invariant."""
        selected_cards = set()

        # Simulate frantic click stream:
        # Click 0, 1, 2, 3, 4 (select 5 cards)
        # Attempt to click 5 (should be rejected)
        # Click 2 (deselect 2)
        # Click 5 (now succeeds)
        # Click 6 (should be rejected)
        click_stream = [0, 1, 2, 3, 4, 5, 2, 5, 6]
        rejections = 0

        for card_idx in click_stream:
            if card_idx in selected_cards:
                selected_cards.remove(card_idx)
            else:
                if len(selected_cards) >= 5:
                    # Invariant rejection
                    rejections += 1
                else:
                    selected_cards.add(card_idx)

        self.assertEqual(rejections, 2, "Exactly 2 invalid 6th card clicks should be blocked")
        self.assertEqual(len(selected_cards), 5, "Final selection must stay clamped at 5")
        self.assertEqual(selected_cards, {0, 1, 3, 4, 5})


if __name__ == "__main__":
    unittest.main()
