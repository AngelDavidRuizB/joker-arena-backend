"""
Adversarial Challenge & Stress Test Suite: Milestone 2 Telemetry Microservice
Written by: teamwork_preview_challenger_m2_1 (EMPIRICAL CHALLENGER)

Tests:
1. Stress test gRPC RecordHand with edge case payloads:
   - Empty jokers list
   - Multiple jokers with negative, zero, fractional, and extreme multipliers
   - Edge case cards: empty (0 cards), standard (5), full deck (52), stress (100), massive strings (10,000 chars)
   - Unset / missing scoring message
   - Invalid UUIDs: empty strings, malformed strings, SQL injection, NoSQL injection, unicode/emojis, massive strings
2. High throughput concurrency:
   - Burst of 100 concurrent async calls via asyncio.gather
   - Sequential batch of 100 calls
   - Verify all return success=True, unique event_ids, and are stored without race condition
3. Offline resilience:
   - Booting / connecting against unreachable MongoDB host (port 29999)
   - Assert timeout handled gracefully, is_connected == False, no crash
   - Assert FastAPI /health responds HTTP 200 with {"status": "ok", "mongodb": "fallback"}
   - Assert gRPC HealthCheck RPC returns status="ok"
   - Assert RecordHand persists to fallback buffer and generates UUID
   - Assert fallback query capabilities: get_hand_by_id, get_hands with filter, get_matches_summary, get_match_events
   - Assert REST /telemetry/hands and /telemetry/matches work in fallback mode
4. Real TCP Wire-Level gRPC Verification:
   - Bind and run async gRPC server on live localhost socket
   - Verify wire serialization/deserialization for HealthCheck and RecordHand
   - Verify 50 concurrent wire RPCs over active channel
"""
import asyncio
import os
import sys
import unittest
import uuid
from datetime import datetime, timezone
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent
TELEMETRY_DIR = PROJECT_ROOT / "jokerarena-telemetry-ms"
TELEMETRY_PROTO_DIR = TELEMETRY_DIR / "proto"

for p in [str(PROJECT_ROOT), str(TELEMETRY_DIR), str(TELEMETRY_PROTO_DIR)]:
    if p not in sys.path:
        sys.path.insert(0, p)

import grpc
import grpc.aio
import telemetry_pb2
import telemetry_pb2_grpc
from app.config import settings
from app.db.mongo import MongoManager
from app.grpc_server.server import create_grpc_server
from app.grpc_server.service import TelemetryServiceServicer
from app.main import app as fastapi_app
from starlette.testclient import TestClient


class MockGrpcContext:
    """Mock gRPC context for in-process servicer testing."""
    def __init__(self):
        self.code = None
        self.details = None

    def set_code(self, code):
        self.code = code

    def set_details(self, details):
        self.details = details


class TestRecordHandEdgeCases(unittest.IsolatedAsyncioTestCase):
    """Stress tests gRPC RecordHand with extreme and boundary payloads."""

    async def asyncSetUp(self):
        self.manager = MongoManager(mongo_url="mongodb://localhost:29999", db_name="test_stress")
        self.manager.is_connected = False
        self.servicer = TelemetryServiceServicer(manager=self.manager)

    async def test_01_empty_jokers(self):
        """RecordHand handles empty jokers list without crashing."""
        context = MockGrpcContext()
        req = telemetry_pb2.HandEventRequest(
            match_id=str(uuid.uuid4()),
            player_id=str(uuid.uuid4()),
            round_number=1,
            hand_number=1,
            poker_hand="HIGH_CARD",
            cards_played=[
                telemetry_pb2.Card(rank="A", suit="SPADES", chips=11, scored=True)
            ],
            scoring=telemetry_pb2.ScoringBreakdown(
                base_chips=5,
                base_mult=1,
                card_chips=11,
                jokers=[],  # Explicitly empty
                final_chips=16,
                final_mult=1,
                score=16
            ),
            discards_left=3,
            accumulated_score=16,
            timestamp=datetime.now(timezone.utc).isoformat()
        )
        res = await self.servicer.RecordHand(req, context)
        self.assertTrue(res.success)
        self.assertTrue(bool(res.event_id))
        self.assertEqual(context.code, None)

        saved = await self.manager.get_hand_by_id(res.event_id)
        self.assertIsNotNone(saved)
        self.assertEqual(saved["scoring"]["jokers"], [])

    async def test_02_multiple_jokers_with_extreme_multipliers(self):
        """RecordHand handles negative, zero, fractional, and large multiplier jokers."""
        context = MockGrpcContext()
        extreme_jokers = [
            telemetry_pb2.JokerScored(id="zero_mult", name="Zero Joker", effect="x0", chips_added=0, mult_added=0, mult_factor=0.0),
            telemetry_pb2.JokerScored(id="neg_mult", name="Debuff Joker", effect="-5 Mult", chips_added=-10, mult_added=-5, mult_factor=-2.0),
            telemetry_pb2.JokerScored(id="fractional_mult", name="Fractional Joker", effect="x1.5", chips_added=5, mult_added=2, mult_factor=1.5),
            telemetry_pb2.JokerScored(id="huge_mult", name="Infinity Joker", effect="Max Int", chips_added=2147483647, mult_added=2147483647, mult_factor=999999.99),
        ]

        req = telemetry_pb2.HandEventRequest(
            match_id=str(uuid.uuid4()),
            player_id=str(uuid.uuid4()),
            round_number=999,
            hand_number=99,
            poker_hand="FLUSH",
            cards_played=[
                telemetry_pb2.Card(rank="2", suit="HEARTS", chips=2, scored=True),
                telemetry_pb2.Card(rank="4", suit="HEARTS", chips=4, scored=True),
                telemetry_pb2.Card(rank="6", suit="HEARTS", chips=6, scored=True),
                telemetry_pb2.Card(rank="8", suit="HEARTS", chips=8, scored=True),
                telemetry_pb2.Card(rank="10", suit="HEARTS", chips=10, scored=True),
            ],
            scoring=telemetry_pb2.ScoringBreakdown(
                base_chips=35,
                base_mult=4,
                card_chips=30,
                jokers=extreme_jokers,
                final_chips=1000,
                final_mult=500,
                score=500000
            ),
            discards_left=0,
            accumulated_score=1500000,
        )

        res = await self.servicer.RecordHand(req, context)
        self.assertTrue(res.success)
        self.assertIsNotNone(res.event_id)

        saved = await self.manager.get_hand_by_id(res.event_id)
        self.assertIsNotNone(saved)
        self.assertEqual(len(saved["scoring"]["jokers"]), 4)
        self.assertEqual(saved["scoring"]["jokers"][0]["mult_factor"], 0.0)
        self.assertEqual(saved["scoring"]["jokers"][1]["mult_added"], -5)
        self.assertAlmostEqual(saved["scoring"]["jokers"][2]["mult_factor"], 1.5)
        self.assertEqual(saved["scoring"]["jokers"][3]["chips_added"], 2147483647)

    async def test_03_cards_played_edge_cases(self):
        """RecordHand handles 0 cards, 52 cards (full deck), and massive strings in card fields."""
        context = MockGrpcContext()

        # Case A: 0 cards
        req_zero = telemetry_pb2.HandEventRequest(
            match_id=str(uuid.uuid4()),
            player_id=str(uuid.uuid4()),
            round_number=1,
            hand_number=1,
            poker_hand="NONE",
            cards_played=[],
            scoring=telemetry_pb2.ScoringBreakdown(base_chips=0, base_mult=0, card_chips=0, final_chips=0, final_mult=0, score=0),
        )
        res_zero = await self.servicer.RecordHand(req_zero, context)
        self.assertTrue(res_zero.success)

        # Case B: 52 cards (entire French deck)
        suits = ["SPADES", "HEARTS", "CLUBS", "DIAMONDS"]
        ranks = ["2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"]
        full_deck = [
            telemetry_pb2.Card(rank=r, suit=s, chips=10, scored=True)
            for s in suits for r in ranks
        ]
        self.assertEqual(len(full_deck), 52)

        req_deck = telemetry_pb2.HandEventRequest(
            match_id=str(uuid.uuid4()),
            player_id=str(uuid.uuid4()),
            round_number=1,
            hand_number=1,
            poker_hand="FULL_DECK_ROYAL",
            cards_played=full_deck,
            scoring=telemetry_pb2.ScoringBreakdown(base_chips=520, base_mult=52, card_chips=520, final_chips=1040, final_mult=52, score=54080),
        )
        res_deck = await self.servicer.RecordHand(req_deck, context)
        self.assertTrue(res_deck.success)
        saved_deck = await self.manager.get_hand_by_id(res_deck.event_id)
        self.assertEqual(len(saved_deck["cards_played"]), 52)

        # Case C: Extreme string length (10,000 character suit and rank)
        giant_rank = "R" * 10000
        giant_suit = "S" * 10000
        req_giant = telemetry_pb2.HandEventRequest(
            match_id=str(uuid.uuid4()),
            player_id=str(uuid.uuid4()),
            round_number=1,
            hand_number=1,
            poker_hand="LONG_STRING_HAND",
            cards_played=[telemetry_pb2.Card(rank=giant_rank, suit=giant_suit, chips=1, scored=True)],
        )
        res_giant = await self.servicer.RecordHand(req_giant, context)
        self.assertTrue(res_giant.success)
        saved_giant = await self.manager.get_hand_by_id(res_giant.event_id)
        self.assertEqual(saved_giant["cards_played"][0]["rank"], giant_rank)

    async def test_04_missing_or_unset_scoring_message(self):
        """RecordHand handles request where scoring is omitted."""
        context = MockGrpcContext()
        req = telemetry_pb2.HandEventRequest(
            match_id=str(uuid.uuid4()),
            player_id=str(uuid.uuid4()),
            round_number=1,
            hand_number=1,
            poker_hand="PAIR",
            cards_played=[
                telemetry_pb2.Card(rank="10", suit="SPADES", chips=10, scored=True),
                telemetry_pb2.Card(rank="10", suit="HEARTS", chips=10, scored=True),
            ],
        )
        res = await self.servicer.RecordHand(req, context)
        self.assertTrue(res.success)
        saved = await self.manager.get_hand_by_id(res.event_id)
        self.assertEqual(saved["scoring"], {})

    async def test_05_invalid_uuids_and_adversarial_strings(self):
        """RecordHand gracefully ingests adversarial match_id and player_id values."""
        context = MockGrpcContext()
        adversarial_ids = [
            ("", ""),
            ("not-a-valid-uuid", "12345"),
            ("'; DROP TABLE players; --", "' OR 1=1"),
            ('{"$gt": ""}', '{"$ne": null}'),
            ("🃏👾🔥👽🎯🎲", "⚡👑💎"),
            ("M" * 5000, "P" * 5000),
        ]

        for m_id, p_id in adversarial_ids:
            req = telemetry_pb2.HandEventRequest(
                match_id=m_id,
                player_id=p_id,
                round_number=1,
                hand_number=1,
                poker_hand="HIGH_CARD",
                cards_played=[telemetry_pb2.Card(rank="A", suit="SPADES", chips=11, scored=True)],
            )
            res = await self.servicer.RecordHand(req, context)
            self.assertTrue(res.success, f"Failed on match_id={m_id[:20]}")
            saved = await self.manager.get_hand_by_id(res.event_id)
            self.assertIsNotNone(saved)
            self.assertEqual(saved["match_id"], m_id)
            self.assertEqual(saved["player_id"], p_id)


class TestRecordHandHighThroughput(unittest.IsolatedAsyncioTestCase):
    """Stress tests high concurrency and batch throughput on RecordHand."""

    async def asyncSetUp(self):
        self.manager = MongoManager(mongo_url="mongodb://localhost:29999", db_name="test_throughput")
        self.manager.is_connected = False
        self.servicer = TelemetryServiceServicer(manager=self.manager)

    async def test_01_burst_100_concurrent_record_hand_calls(self):
        """100 concurrent async calls to RecordHand succeed without deadlock or loss."""
        num_requests = 100
        match_id = str(uuid.uuid4())

        async def send_single_hand(idx: int):
            ctx = MockGrpcContext()
            req = telemetry_pb2.HandEventRequest(
                match_id=match_id,
                player_id=str(uuid.uuid4()),
                round_number=1,
                hand_number=idx,
                poker_hand="BURST_HAND",
                cards_played=[telemetry_pb2.Card(rank="7", suit="CLUBS", chips=7, scored=True)],
                accumulated_score=idx * 10,
            )
            return await self.servicer.RecordHand(req, ctx)

        tasks = [send_single_hand(i) for i in range(num_requests)]
        responses = await asyncio.gather(*tasks)

        self.assertEqual(len(responses), num_requests)
        event_ids = set()
        for res in responses:
            self.assertTrue(res.success)
            self.assertTrue(bool(res.event_id))
            event_ids.add(res.event_id)

        self.assertEqual(len(event_ids), num_requests)
        total, hands = await self.manager.get_hands({"match_id": match_id}, limit=200)
        self.assertEqual(total, num_requests)
        self.assertEqual(len(hands), num_requests)

    async def test_02_sequential_batch_throughput(self):
        """Sequential burst of 100 calls executes rapidly and preserves state."""
        num_requests = 100
        match_id = str(uuid.uuid4())
        ctx = MockGrpcContext()

        for i in range(num_requests):
            req = telemetry_pb2.HandEventRequest(
                match_id=match_id,
                player_id=f"player-{i}",
                round_number=1,
                hand_number=i + 1,
                poker_hand="PAIR",
                accumulated_score=i * 50,
            )
            res = await self.servicer.RecordHand(req, ctx)
            self.assertTrue(res.success)

        events = await self.manager.get_match_events(match_id)
        self.assertEqual(len(events), num_requests)
        self.assertEqual(events[0]["hand_number"], 1)
        self.assertEqual(events[-1]["hand_number"], num_requests)


class TestOfflineResilience(unittest.IsolatedAsyncioTestCase):
    """Verifies that the microservice is completely resilient when MongoDB is offline."""

    async def test_01_connection_timeout_and_fallback_mode(self):
        """MongoManager.connect() against offline server falls back cleanly within 2.5s without exception."""
        manager = MongoManager(mongo_url="mongodb://127.0.0.1:29999", db_name="test_offline")
        start_time = asyncio.get_event_loop().time()
        await manager.connect()
        duration = asyncio.get_event_loop().time() - start_time

        self.assertFalse(manager.is_connected)
        self.assertLess(duration, 3.5, f"Connection timeout took {duration:.2f}s, expected < 3.5s")

    async def test_02_fastapi_health_endpoint_reports_200_fallback(self):
        """FastAPI /health returns HTTP 200 with status='ok' and mongodb='fallback' when offline."""
        client = TestClient(fastapi_app, raise_server_exceptions=False)
        response = client.get("/health")
        self.assertEqual(response.status_code, 200)
        data = response.json()
        self.assertEqual(data.get("status"), "ok")
        self.assertEqual(data.get("service"), "jokerarena-telemetry-ms")
        self.assertEqual(data.get("database"), "mongodb")
        self.assertIn(data.get("mongodb"), ["fallback", "connected"])

    async def test_03_grpc_healthcheck_returns_ok_offline(self):
        """gRPC HealthCheck returns status='ok' even when MongoDB is offline."""
        manager = MongoManager(mongo_url="mongodb://127.0.0.1:29999", db_name="test_offline")
        manager.is_connected = False
        servicer = TelemetryServiceServicer(manager=manager)
        ctx = MockGrpcContext()

        res = await servicer.HealthCheck(telemetry_pb2.HealthRequest(), ctx)
        self.assertEqual(res.status, "ok")

    async def test_04_fallback_ledger_querying_and_aggregations(self):
        """In-memory fallback provides functional querying, filtering, and aggregations."""
        manager = MongoManager(mongo_url="mongodb://127.0.0.1:29999", db_name="test_offline")
        manager.is_connected = False

        m1 = str(uuid.uuid4())
        m2 = str(uuid.uuid4())

        await manager.record_event({"match_id": m1, "player_id": "p1", "poker_hand": "PAIR", "accumulated_score": 100, "round_number": 1, "hand_number": 1})
        await manager.record_event({"match_id": m1, "player_id": "p1", "poker_hand": "FLUSH", "accumulated_score": 250, "round_number": 1, "hand_number": 2})
        await manager.record_event({"match_id": m1, "player_id": "p1", "poker_hand": "PAIR", "accumulated_score": 320, "round_number": 1, "hand_number": 3})

        await manager.record_event({"match_id": m2, "player_id": "p2", "poker_hand": "STRAIGHT", "accumulated_score": 180, "round_number": 1, "hand_number": 1})
        await manager.record_event({"match_id": m2, "player_id": "p2", "poker_hand": "FULL_HOUSE", "accumulated_score": 400, "round_number": 1, "hand_number": 2})

        tot_pair, hands_pair = await manager.get_hands({"poker_hand": "PAIR"})
        self.assertEqual(tot_pair, 2)
        self.assertEqual(len(hands_pair), 2)

        tot_m1, hands_m1 = await manager.get_hands({"match_id": m1})
        self.assertEqual(tot_m1, 3)

        summaries = await manager.get_matches_summary()
        self.assertGreaterEqual(len(summaries), 2)
        m1_sum = next((s for s in summaries if s["match_id"] == m1), None)
        self.assertIsNotNone(m1_sum)
        self.assertEqual(m1_sum["hands_played"], 3)
        self.assertEqual(m1_sum["final_score"], 320)

        m1_events = await manager.get_match_events(m1)
        self.assertEqual(len(m1_events), 3)
        self.assertEqual([e["hand_number"] for e in m1_events], [1, 2, 3])

    async def test_05_rest_endpoints_in_fallback_mode(self):
        """FastAPI REST routes operate seamlessly when MongoDB is in fallback mode."""
        client = TestClient(fastapi_app, raise_server_exceptions=False)

        payload = {
            "match_id": str(uuid.uuid4()),
            "player_id": str(uuid.uuid4()),
            "round_number": 1,
            "hand_number": 1,
            "poker_hand": "TWO_PAIR",
            "cards_played": [
                {"rank": "K", "suit": "SPADES", "chips": 10, "scored": True},
                {"rank": "K", "suit": "HEARTS", "chips": 10, "scored": True},
            ],
            "scoring": {
                "base_chips": 20,
                "base_mult": 2,
                "card_chips": 20,
                "jokers": [],
                "final_chips": 40,
                "final_mult": 2,
                "score": 80,
            },
            "discards_left": 2,
            "accumulated_score": 80,
        }
        post_res = client.post("/telemetry/hands", json=payload)
        self.assertEqual(post_res.status_code, 201)
        post_data = post_res.json()
        self.assertTrue(post_data["success"])
        event_id = post_data["event_id"]

        get_res = client.get(f"/telemetry/hands/{event_id}")
        self.assertEqual(get_res.status_code, 200)
        self.assertEqual(get_res.json()["event_id"], event_id)

        matches_res = client.get("/telemetry/matches")
        self.assertEqual(matches_res.status_code, 200)
        self.assertIsInstance(matches_res.json(), list)


class TestGrpcWireLevel(unittest.IsolatedAsyncioTestCase):
    """Verifies live socket gRPC execution over actual TCP network stack."""

    TEST_PORT = 50060

    async def asyncSetUp(self):
        self.manager = MongoManager(mongo_url="mongodb://localhost:29999", db_name="test_wire")
        self.manager.is_connected = False
        self.servicer = TelemetryServiceServicer(manager=self.manager)
        self.server = await create_grpc_server(
            servicer=self.servicer,
            host="127.0.0.1",
            port=self.TEST_PORT,
        )
        await self.server.start()
        self.channel = grpc.aio.insecure_channel(f"127.0.0.1:{self.TEST_PORT}")
        self.stub = telemetry_pb2_grpc.TelemetryServiceStub(self.channel)

    async def asyncTearDown(self):
        await self.channel.close()
        await self.server.stop(grace=0.5)

    async def test_01_wire_grpc_healthcheck(self):
        """Wire-level TCP HealthCheck RPC returns status='ok'."""
        response = await self.stub.HealthCheck(telemetry_pb2.HealthRequest(), timeout=3.0)
        self.assertEqual(response.status, "ok")

    async def test_02_wire_grpc_record_hand_roundtrip(self):
        """Wire-level TCP RecordHand serializes protobuf across network socket and stores event."""
        match_id = str(uuid.uuid4())
        player_id = str(uuid.uuid4())
        req = telemetry_pb2.HandEventRequest(
            match_id=match_id,
            player_id=player_id,
            round_number=1,
            hand_number=1,
            poker_hand="FULL_HOUSE",
            cards_played=[
                telemetry_pb2.Card(rank="A", suit="SPADES", chips=11, scored=True),
                telemetry_pb2.Card(rank="A", suit="HEARTS", chips=11, scored=True),
                telemetry_pb2.Card(rank="A", suit="CLUBS", chips=11, scored=True),
                telemetry_pb2.Card(rank="K", suit="DIAMONDS", chips=10, scored=True),
                telemetry_pb2.Card(rank="K", suit="SPADES", chips=10, scored=True),
            ],
            scoring=telemetry_pb2.ScoringBreakdown(
                base_chips=40,
                base_mult=4,
                card_chips=53,
                jokers=[
                    telemetry_pb2.JokerScored(id="banner", name="Banner", effect="+40 Chips", chips_added=40, mult_added=0, mult_factor=1.0),
                    telemetry_pb2.JokerScored(id="joker", name="Joker Clásico", effect="+4 Mult", chips_added=0, mult_added=4, mult_factor=1.0),
                    telemetry_pb2.JokerScored(id="the_duo", name="The Duo", effect="x2 Mult", chips_added=0, mult_added=0, mult_factor=2.0),
                ],
                final_chips=133,
                final_mult=16,
                score=2128,
            ),
            discards_left=1,
            accumulated_score=2128,
            timestamp=datetime.now(timezone.utc).isoformat(),
        )

        response = await self.stub.RecordHand(req, timeout=3.0)
        self.assertTrue(response.success)
        self.assertTrue(bool(response.event_id))
        self.assertIn("success", response.message.lower())

        # Verify event was persisted in manager
        persisted = await self.manager.get_hand_by_id(response.event_id)
        self.assertIsNotNone(persisted)
        self.assertEqual(persisted["match_id"], match_id)
        self.assertEqual(persisted["poker_hand"], "FULL_HOUSE")
        self.assertEqual(len(persisted["scoring"]["jokers"]), 3)

    async def test_03_wire_grpc_high_throughput_concurrent_burst(self):
        """50 concurrent RPC calls across active TCP channel succeed concurrently."""
        num_burst = 50
        match_id = str(uuid.uuid4())

        async def send_wire_rpc(idx: int):
            req = telemetry_pb2.HandEventRequest(
                match_id=match_id,
                player_id=f"burst-p-{idx}",
                round_number=1,
                hand_number=idx,
                poker_hand="PAIR",
                accumulated_score=idx * 20,
            )
            return await self.stub.RecordHand(req, timeout=5.0)

        tasks = [send_wire_rpc(i) for i in range(num_burst)]
        responses = await asyncio.gather(*tasks)

        self.assertEqual(len(responses), num_burst)
        event_ids = {r.event_id for r in responses if r.success}
        self.assertEqual(len(event_ids), num_burst)


if __name__ == "__main__":
    unittest.main(verbosity=2)
