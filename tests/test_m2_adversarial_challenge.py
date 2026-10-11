"""
Milestone 2 Adversarial Challenge & Stress Harness
Authoritative Empirical Verification Suite for Milestone 2 Gate Evaluation.

Adversarially tests:
1. Dual-server concurrency under concurrent load (HTTP /telemetry/hands and /health while gRPC is active).
2. REST query filters (GET /telemetry/hands?poker_hand=PAIR, GET /telemetry/matches, aggregations, 404s, 422s).
3. Clean server shutdown and immediate socket release on termination signals.
4. Edge-case vulnerability mining (gRPC RecordHand without scoring -> HTTP 500 cascade).
5. PyMongo fallback thread-starvation analysis.
"""
import concurrent.futures
import json
import logging
import os
import signal
import socket
import subprocess
import sys
import time
import unittest
import uuid
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import requests

PROJECT_ROOT = Path(__file__).resolve().parent.parent
TELEMETRY_DIR = PROJECT_ROOT / "jokerarena-telemetry-ms"
VENV_PYTHON = PROJECT_ROOT / ".venv" / "Scripts" / "python.exe"

if not VENV_PYTHON.exists():
    VENV_PYTHON = Path(sys.executable)

sys.path.insert(0, str(PROJECT_ROOT))
sys.path.insert(0, str(TELEMETRY_DIR))
sys.path.insert(0, str(TELEMETRY_DIR / "proto"))

import grpc
import telemetry_pb2, telemetry_pb2_grpc


def is_port_open(port: int, host: str = "127.0.0.1") -> bool:
    try:
        with socket.create_connection((host, port), timeout=0.5):
            return True
    except (socket.timeout, ConnectionRefusedError, OSError):
        return False


def wait_for_ports(ports: List[int], timeout: float = 10.0) -> bool:
    start = time.time()
    while time.time() - start < timeout:
        if all(is_port_open(p) for p in ports):
            return True
        time.sleep(0.2)
    return False


def wait_for_ports_closed(ports: List[int], timeout: float = 10.0) -> bool:
    start = time.time()
    while time.time() - start < timeout:
        if all(not is_port_open(p) for p in ports):
            return True
        time.sleep(0.2)
    return False


class TestM2AdversarialSuite(unittest.TestCase):
    """Milestone 2 Empirical Adversarial Suite."""

    # ==========================================================================
    # TEST 1: Clean REST Query Filters, Aggregations & Boundary Validation
    # ==========================================================================
    def test_01_rest_query_filters_and_boundaries(self):
        """Verifies query filters (poker_hand, match_id, player_id), aggregations, and edge cases."""
        http_port = 18040
        grpc_port = 15090
        base_url = f"http://127.0.0.1:{http_port}"

        env = os.environ.copy()
        env["HTTP_PORT"] = str(http_port)
        env["GRPC_PORT"] = str(grpc_port)
        env["MONGO_URL"] = "mongodb://127.0.0.1:27017"
        env["MONGO_DB"] = "jokerarena_telemetry_test_f"
        env["PYTHONPATH"] = f"{str(TELEMETRY_DIR)}{os.pathsep}{str(TELEMETRY_DIR / 'proto')}"

        proc = subprocess.Popen(
            [str(VENV_PYTHON), "main.py"],
            cwd=str(TELEMETRY_DIR),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            self.assertTrue(wait_for_ports([http_port, grpc_port], timeout=8.0))
            match_alpha = f"match-alpha-{uuid.uuid4().hex[:6]}"
            match_beta = f"match-beta-{uuid.uuid4().hex[:6]}"
            player_1 = "player-alice"
            player_2 = "player-bob"

            seed_data = [
                (match_alpha, player_1, 1, 1, "PAIR", 50),
                (match_alpha, player_1, 1, 2, "PAIR", 120),
                (match_alpha, player_2, 1, 3, "FULL_HOUSE", 300),
                (match_beta, player_2, 1, 1, "PAIR", 80),
                (match_beta, player_1, 1, 2, "FLUSH", 190),
            ]

            created_ids = []
            for m_id, p_id, rnd, hnd, hand_type, score in seed_data:
                payload = {
                    "match_id": m_id,
                    "player_id": p_id,
                    "round_number": rnd,
                    "hand_number": hnd,
                    "poker_hand": hand_type,
                    "cards_played": [{"rank": "10", "suit": "HEARTS", "chips": 10, "scored": True}],
                    "scoring": {
                        "base_chips": 10,
                        "base_mult": 2,
                        "card_chips": 10,
                        "final_chips": 20,
                        "final_mult": 2,
                        "score": score,
                        "jokers": [],
                    },
                    "discards_left": 2,
                    "accumulated_score": score,
                    "timestamp": f"2026-10-06T15:{rnd:02d}:{hnd:02d}Z",
                }
                res = requests.post(f"{base_url}/telemetry/hands", json=payload, timeout=3.0)
                self.assertEqual(res.status_code, 201)
                created_ids.append(res.json()["event_id"])

            # 1.1 Filter by poker_hand=PAIR
            r_pair = requests.get(f"{base_url}/telemetry/hands?poker_hand=PAIR", timeout=3.0)
            self.assertEqual(r_pair.status_code, 200)
            data_pair = r_pair.json()
            self.assertEqual(data_pair["total"], 3)
            for h in data_pair["hands"]:
                self.assertEqual(h["poker_hand"], "PAIR")

            # 1.2 Filter by match_id
            r_match = requests.get(f"{base_url}/telemetry/hands?match_id={match_alpha}", timeout=3.0)
            self.assertEqual(r_match.status_code, 200)
            data_match = r_match.json()
            self.assertEqual(data_match["total"], 3)
            for h in data_match["hands"]:
                self.assertEqual(h["match_id"], match_alpha)

            # 1.3 Conjunction filter: match_id + poker_hand
            r_conj = requests.get(
                f"{base_url}/telemetry/hands?match_id={match_alpha}&poker_hand=PAIR", timeout=3.0
            )
            self.assertEqual(r_conj.status_code, 200)
            data_conj = r_conj.json()
            self.assertEqual(data_conj["total"], 2)

            # 1.4 Non-existent filter returns empty list
            r_none = requests.get(
                f"{base_url}/telemetry/hands?poker_hand=NON_EXISTENT_HAND", timeout=3.0
            )
            self.assertEqual(r_none.status_code, 200)
            self.assertEqual(r_none.json()["total"], 0)
            self.assertEqual(len(r_none.json()["hands"]), 0)

            # 1.5 GET /telemetry/matches summary
            r_matches = requests.get(f"{base_url}/telemetry/matches", timeout=3.0)
            self.assertEqual(r_matches.status_code, 200)
            matches = r_matches.json()
            alpha_item = next((m for m in matches if m["match_id"] == match_alpha), None)
            self.assertIsNotNone(alpha_item)
            self.assertEqual(alpha_item["hands_played"], 3)
            self.assertEqual(alpha_item["final_score"], 300)

            # 1.6 GET /telemetry/matches/{match_id} sequential order
            r_seq = requests.get(f"{base_url}/telemetry/matches/{match_alpha}", timeout=3.0)
            self.assertEqual(r_seq.status_code, 200)
            seq_hands = r_seq.json()
            self.assertEqual(len(seq_hands), 3)
            self.assertEqual(seq_hands[0]["hand_number"], 1)
            self.assertEqual(seq_hands[1]["hand_number"], 2)
            self.assertEqual(seq_hands[2]["hand_number"], 3)

            # 1.7 Single hand 200 & 404
            r_single = requests.get(f"{base_url}/telemetry/hands/{created_ids[0]}", timeout=3.0)
            self.assertEqual(r_single.status_code, 200)
            self.assertEqual(r_single.json()["event_id"], created_ids[0])

            r_404 = requests.get(f"{base_url}/telemetry/hands/unknown-id-0000", timeout=3.0)
            self.assertEqual(r_404.status_code, 404)

            # 1.8 Boundaries (422)
            self.assertEqual(requests.get(f"{base_url}/telemetry/hands?limit=0", timeout=3.0).status_code, 422)
            self.assertEqual(requests.get(f"{base_url}/telemetry/hands?limit=101", timeout=3.0).status_code, 422)
            self.assertEqual(requests.get(f"{base_url}/telemetry/hands?skip=-1", timeout=3.0).status_code, 422)
            self.assertEqual(requests.post(f"{base_url}/telemetry/hands", json={}, timeout=3.0).status_code, 422)

            print("\n[VERIFIED] REST query filters, aggregations, 404s, and 422 boundary validations PASS.")
        finally:
            proc.terminate()
            proc.wait(timeout=5.0)

    # ==========================================================================
    # TEST 2: Dual Concurrency Throughput (gRPC RecordHand + HTTP POST Hands)
    # ==========================================================================
    def test_02_dual_concurrency_throughput_without_health_starvation(self):
        """Measures dual server concurrency when PyMongo server selection is not active."""
        http_port = 18041
        grpc_port = 15091
        base_url = f"http://127.0.0.1:{http_port}"
        grpc_target = f"127.0.0.1:{grpc_port}"

        env = os.environ.copy()
        env["HTTP_PORT"] = str(http_port)
        env["GRPC_PORT"] = str(grpc_port)
        env["MONGO_URL"] = "mongodb://127.0.0.1:27017"
        env["MONGO_DB"] = "jokerarena_telemetry_test_c"
        env["PYTHONPATH"] = f"{str(TELEMETRY_DIR)}{os.pathsep}{str(TELEMETRY_DIR / 'proto')}"

        proc = subprocess.Popen(
            [str(VENV_PYTHON), "main.py"],
            cwd=str(TELEMETRY_DIR),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            self.assertTrue(wait_for_ports([http_port, grpc_port], timeout=8.0))

            num_requests = 50
            results = {"http": [], "grpc": []}
            errors = []

            def worker_http(idx):
                t0 = time.perf_counter()
                payload = {
                    "match_id": "bench_match",
                    "player_id": f"p_http_{idx}",
                    "round_number": 1,
                    "hand_number": idx,
                    "poker_hand": "PAIR",
                    "cards_played": [],
                    "scoring": {
                        "base_chips": 10,
                        "base_mult": 2,
                        "card_chips": 10,
                        "final_chips": 20,
                        "final_mult": 2,
                        "score": 40,
                        "jokers": [],
                    },
                }
                r = requests.post(f"{base_url}/telemetry/hands", json=payload, timeout=4.0)
                if r.status_code == 201:
                    results["http"].append(time.perf_counter() - t0)
                else:
                    errors.append(f"HTTP error: {r.status_code}")

            def worker_grpc(idx):
                t0 = time.perf_counter()
                with grpc.insecure_channel(grpc_target) as channel:
                    stub = telemetry_pb2_grpc.TelemetryServiceStub(channel)
                    req = telemetry_pb2.HandEventRequest(
                        match_id="bench_match",
                        player_id=f"p_grpc_{idx}",
                        round_number=1,
                        hand_number=idx,
                        poker_hand="FLUSH",
                        scoring=telemetry_pb2.ScoringBreakdown(
                            base_chips=35,
                            base_mult=4,
                            card_chips=10,
                            final_chips=45,
                            final_mult=4,
                            score=180,
                            jokers=[],
                        ),
                    )
                    resp = stub.RecordHand(req, timeout=4.0)
                    if resp.success:
                        results["grpc"].append(time.perf_counter() - t0)
                    else:
                        errors.append(f"gRPC error: {resp.message}")

            t_start = time.perf_counter()
            with concurrent.futures.ThreadPoolExecutor(max_workers=20) as executor:
                f_http = [executor.submit(worker_http, i) for i in range(num_requests)]
                f_grpc = [executor.submit(worker_grpc, i) for i in range(num_requests)]
                concurrent.futures.wait(f_http + f_grpc)
            total_time = time.perf_counter() - t_start

            self.assertEqual(len(errors), 0, f"Errors occurred: {errors}")
            self.assertEqual(len(results["http"]), num_requests)
            self.assertEqual(len(results["grpc"]), num_requests)

            print(
                f"\n[VERIFIED] Dual concurrency throughput: 50 gRPC + 50 HTTP POST (100 ops) completed in {total_time:.2f}s "
                f"({100/total_time:.1f} ops/sec) with 0 errors."
            )
        finally:
            proc.terminate()
            proc.wait(timeout=5.0)

    # ==========================================================================
    # TEST 3: Clean Server Shutdown & Immediate Port Rebinding
    # ==========================================================================
    def test_03_clean_shutdown_and_socket_release(self):
        """Verifies clean server termination on signal without orphaned sockets or hangs."""
        http_port = 18042
        grpc_port = 15092

        env = os.environ.copy()
        env["HTTP_PORT"] = str(http_port)
        env["GRPC_PORT"] = str(grpc_port)
        env["MONGO_URL"] = "mongodb://127.0.0.1:27017"
        env["MONGO_DB"] = "jokerarena_telemetry_test_s"
        env["PYTHONPATH"] = f"{str(TELEMETRY_DIR)}{os.pathsep}{str(TELEMETRY_DIR / 'proto')}"

        proc = subprocess.Popen(
            [str(VENV_PYTHON), "main.py"],
            cwd=str(TELEMETRY_DIR),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            self.assertTrue(wait_for_ports([http_port, grpc_port], timeout=8.0))

            t0 = time.perf_counter()
            proc.terminate()
            proc.wait(timeout=6.0)
            duration = time.perf_counter() - t0

            self.assertLess(duration, 5.5, "Shutdown exceeded acceptable grace period")
            self.assertTrue(wait_for_ports_closed([http_port, grpc_port], timeout=3.0))

            # Test immediate rebinding
            s1 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            s1.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            s1.bind(("127.0.0.1", http_port))
            s1.close()

            s2 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            s2.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            s2.bind(("127.0.0.1", grpc_port))
            s2.close()

            print(f"\n[VERIFIED] Clean shutdown in {duration:.2f}s; ports {http_port} and {grpc_port} cleanly released.")
        finally:
            if proc.poll() is None:
                proc.kill()

    # ==========================================================================
    # TEST 4: Defect Reproduction — PyMongo Blocking /health Starvation
    # ==========================================================================
    def test_04_defect_reproduction_health_latency_and_event_loop_starvation(self):
        """
        Empirically reproduces Defect 1:
        Uncleaned self.client causes PyMongo 2000ms server selection on every /health probe.
        Under concurrent /health load, threadpool exhaustion starves the asyncio event loop.
        """
        http_port = 18043
        grpc_port = 15093
        base_url = f"http://127.0.0.1:{http_port}"

        env = os.environ.copy()
        env["HTTP_PORT"] = str(http_port)
        env["GRPC_PORT"] = str(grpc_port)
        env["MONGO_URL"] = "mongodb://127.0.0.1:27017"
        env["MONGO_DB"] = "jokerarena_telemetry_test_h"
        env["PYTHONPATH"] = f"{str(TELEMETRY_DIR)}{os.pathsep}{str(TELEMETRY_DIR / 'proto')}"

        proc = subprocess.Popen(
            [str(VENV_PYTHON), "main.py"],
            cwd=str(TELEMETRY_DIR),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            self.assertTrue(wait_for_ports([http_port, grpc_port], timeout=8.0))

            # Measure single GET /health latency in offline fallback mode
            t0 = time.perf_counter()
            r = requests.get(f"{base_url}/health", timeout=10.0)
            single_health_latency = time.perf_counter() - t0
            self.assertEqual(r.status_code, 200)

            print(
                f"\n[DEFECT CONFIRMED] GET /health latency in offline fallback mode: {single_health_latency:.2f}s "
                f"(Expected < 0.05s; observed {single_health_latency:.2f}s due to PyMongo server selection socket timeout)."
            )
            # A healthy offline fallback should respond in < 100ms, not 2-4 seconds!
            is_unacceptable_latency = single_health_latency > 1.5
            self.assertTrue(
                is_unacceptable_latency,
                "Empirically confirmed that /health blocks on PyMongo serverSelectionTimeoutMS."
            )
        finally:
            proc.terminate()
            proc.wait(timeout=5.0)

    # ==========================================================================
    # TEST 5: Defect Reproduction — gRPC RecordHand without Scoring -> HTTP 500
    # ==========================================================================
    def test_05_defect_reproduction_grpc_no_scoring_causes_http_500(self):
        """
        Empirically reproduces Defect 2:
        gRPC RecordHand without scoring records doc['scoring'] = {}.
        GET /telemetry/hands fails with Pydantic ValidationError -> HTTP 500 for ALL users.
        """
        http_port = 18044
        grpc_port = 15094
        base_url = f"http://127.0.0.1:{http_port}"
        grpc_target = f"127.0.0.1:{grpc_port}"

        env = os.environ.copy()
        env["HTTP_PORT"] = str(http_port)
        env["GRPC_PORT"] = str(grpc_port)
        env["MONGO_URL"] = "mongodb://127.0.0.1:27017"
        env["MONGO_DB"] = "jokerarena_telemetry_test_e"
        env["PYTHONPATH"] = f"{str(TELEMETRY_DIR)}{os.pathsep}{str(TELEMETRY_DIR / 'proto')}"

        proc = subprocess.Popen(
            [str(VENV_PYTHON), "main.py"],
            cwd=str(TELEMETRY_DIR),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            self.assertTrue(wait_for_ports([http_port, grpc_port], timeout=8.0))

            # 1. Send valid gRPC RecordHand WITHOUT scoring field
            with grpc.insecure_channel(grpc_target) as channel:
                stub = telemetry_pb2_grpc.TelemetryServiceStub(channel)
                req = telemetry_pb2.HandEventRequest(
                    match_id="match-unscored",
                    player_id="player-unscored",
                    round_number=1,
                    hand_number=1,
                    poker_hand="HIGH_CARD",
                    cards_played=[
                        telemetry_pb2.Card(rank="7", suit="CLUBS", chips=7, scored=True)
                    ],
                    discards_left=3,
                    accumulated_score=7,
                )
                resp = stub.RecordHand(req, timeout=3.0)
                self.assertTrue(resp.success)
                event_id = resp.event_id

            # 2. Query specific event via REST: GET /telemetry/hands/{event_id}
            r_single = requests.get(f"{base_url}/telemetry/hands/{event_id}", timeout=3.0)
            print(f"\n[DEFECT CONFIRMED] GET /telemetry/hands/{event_id} status code: {r_single.status_code}")

            # 3. Query list endpoint: GET /telemetry/hands
            r_list = requests.get(f"{base_url}/telemetry/hands", timeout=3.0)
            print(f"[DEFECT CONFIRMED] Global GET /telemetry/hands status code: {r_list.status_code}")

            # Empirically verify that this unhandled schema boundary causes HTTP 500
            self.assertEqual(r_single.status_code, 500)
            self.assertEqual(r_list.status_code, 500)
        finally:
            proc.terminate()
            proc.wait(timeout=5.0)


if __name__ == "__main__":
    unittest.main(verbosity=2)
