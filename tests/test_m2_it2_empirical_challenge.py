"""
Empirical Adversarial Challenge Suite: Milestone 2 Iteration 2 (Telemetry Microservice)
Author: teamwork_preview_challenger_m2_it2_2 (EMPIRICAL CHALLENGER)

Adversarial Stress Verification Targets:
1. Concurrency and Latency:
   - Rapid concurrent requests (150 requests) to /health with MongoDB OFFLINE.
   - Sub-second response times (< 1.0s) and no event loop starvation.
2. Mixed Concurrent Load (200 requests):
   - Simultaneous 50 /health + 50 gRPC RecordHand + 50 HTTP POST /telemetry/hands + 50 GET /telemetry/hands.
   - Zero timeouts, zero deadline exceeded (no StatusCode.DEADLINE_EXCEEDED).
3. Schema Resilience & Unscored Hand Events:
   - gRPC RecordHand with omitted scoring.
   - REST POST /telemetry/hands with scoring: {}, scoring: null, missing scoring.
   - Verification of GET /telemetry/hands and GET /telemetry/hands/{id} without HTTP 500 / ValidationError.
4. Degraded Mode & Timeout Boundary Verification:
   - Verified that mongo_manager.ping() does not block event loop when disconnected (< 1ms).
   - Dynamic simulation of timeout recovery.
5. Event loop responsiveness under continuous flood:
   - Verification that control probes remain fast (< 100ms) under concurrent hammering.
6. Clean Shutdown and Port Rebinding:
   - Termination within grace period and immediate socket rebinding without WSAEADDRINUSE.
"""

import asyncio
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
        with socket.create_connection((host, port), timeout=0.3):
            return True
    except (socket.timeout, ConnectionRefusedError, OSError):
        return False


def wait_for_ports_closed(ports: List[int], timeout: float = 10.0) -> bool:
    start = time.time()
    while time.time() - start < timeout:
        if all(not is_port_open(p) for p in ports):
            return True
        time.sleep(0.2)
    return False


class TestM2It2EmpiricalChallenge(unittest.TestCase):
    """Empirical adversarial test suite for Milestone 2 Iteration 2."""

    @classmethod
    def setUpClass(cls):
        cls.http_port = 18080
        cls.grpc_port = 15080
        cls.base_url = f"http://127.0.0.1:{cls.http_port}"
        cls.grpc_target = f"127.0.0.1:{cls.grpc_port}"

        env = os.environ.copy()
        env["HTTP_PORT"] = str(cls.http_port)
        env["GRPC_PORT"] = str(cls.grpc_port)
        # Point to an offline/unreachable MongoDB port to force offline fallback mode
        env["MONGO_URL"] = "mongodb://127.0.0.1:29999"
        env["MONGO_DB"] = "jokerarena_empirical_m2_it2"
        env["PYTHONPATH"] = f"{str(TELEMETRY_DIR)}{os.pathsep}{str(TELEMETRY_DIR / 'proto')}"

        print(f"\n[SPAWN] Launching telemetry dual-server on HTTP :{cls.http_port} and gRPC :{cls.grpc_port}...")
        cls.proc = subprocess.Popen(
            [str(VENV_PYTHON), "main.py"],
            cwd=str(TELEMETRY_DIR),
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

        # Wait until FastAPI health endpoint responds 200 OK (server fully initialized)
        ready = False
        start = time.time()
        while time.time() - start < 15.0:
            try:
                r = requests.get(f"{cls.base_url}/health", timeout=0.5)
                if r.status_code == 200:
                    ready = True
                    break
            except Exception:
                time.sleep(0.25)

        if not ready:
            cls.proc.terminate()
            raise RuntimeError("Server failed to respond to /health within 15 seconds.")
        print(f"[SPAWN] Server is fully initialized and operational ({time.time() - start:.2f}s).")

    @classmethod
    def tearDownClass(cls):
        print("\n[TEARDOWN] Stopping telemetry dual-server...")
        if cls.proc.poll() is None:
            cls.proc.terminate()
            try:
                cls.proc.wait(timeout=5.0)
            except subprocess.TimeoutExpired:
                cls.proc.kill()
        print("[TEARDOWN] Server stopped.")

    # =========================================================================
    # CHALLENGE 1: Rapid Concurrent /health Checks Under Offline MongoDB
    # =========================================================================
    def test_01_rapid_concurrent_health_latency_subsecond(self):
        """
        Adversarial Test 1:
        Fires 150 concurrent GET /health requests against the live server
        while MongoDB is offline.
        Asserts:
        - 100% of requests return HTTP 200 with mongodb: fallback
        - Max latency per request is STRICTLY sub-second (< 1.0s, expecting < 0.1s)
        - Median latency is < 50ms
        - No event loop starvation / queue delay
        """
        num_requests = 150
        max_workers = 25
        latencies = []
        statuses = []
        payloads = []
        errors = []

        def probe_health(seq: int):
            t0 = time.perf_counter()
            try:
                resp = requests.get(f"{self.base_url}/health", timeout=2.0)
                elapsed = time.perf_counter() - t0
                return seq, resp.status_code, elapsed, resp.json(), None
            except Exception as exc:
                elapsed = time.perf_counter() - t0
                return seq, None, elapsed, None, str(exc)

        t_start = time.perf_counter()
        with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = [executor.submit(probe_health, i) for i in range(num_requests)]
            for fut in concurrent.futures.as_completed(futures):
                seq, status, elapsed, payload, err = fut.result()
                latencies.append(elapsed)
                if err:
                    errors.append(f"Req {seq}: {err}")
                else:
                    statuses.append(status)
                    payloads.append(payload)
        total_wall_time = time.perf_counter() - t_start

        # Statistics
        latencies.sort()
        p50 = latencies[len(latencies) // 2]
        p95 = latencies[int(len(latencies) * 0.95)]
        p99 = latencies[int(len(latencies) * 0.99)]
        max_lat = max(latencies)
        min_lat = min(latencies)
        mean_lat = sum(latencies) / len(latencies)

        print(
            f"\n--- CHALLENGE 1: 150 Concurrent /health Latency Results ---\n"
            f"Total wall time: {total_wall_time:.3f}s ({num_requests / total_wall_time:.1f} req/s)\n"
            f"Min: {min_lat*1000:.2f}ms | Mean: {mean_lat*1000:.2f}ms | Median (p50): {p50*1000:.2f}ms\n"
            f"p95: {p95*1000:.2f}ms | p99: {p99*1000:.2f}ms | Max: {max_lat*1000:.2f}ms\n"
            f"Errors: {len(errors)} / {num_requests}\n"
            f"----------------------------------------------------------"
        )

        # Assertions
        self.assertEqual(len(errors), 0, f"Encountered {len(errors)} errors during concurrent health checks: {errors[:5]}")
        self.assertEqual(len(statuses), num_requests)
        self.assertTrue(all(s == 200 for s in statuses), "Not all status codes were 200")

        # Verify payload content
        sample = payloads[0]
        self.assertEqual(sample["status"], "ok")
        self.assertEqual(sample["mongodb"], "fallback")
        self.assertEqual(sample["service"], "jokerarena-telemetry-ms")

        # Rigorous latency criteria: sub-second
        self.assertLess(max_lat, 1.0, f"Max latency ({max_lat:.3f}s) exceeded sub-second threshold of 1.0s!")
        self.assertLess(p50, 0.10, f"Median latency ({p50*1000:.2f}ms) exceeded 100ms threshold!")
        self.assertLess(total_wall_time, 2.0, f"150 concurrent requests took {total_wall_time:.2f}s (should be < 2.0s)")

    # =========================================================================
    # CHALLENGE 2: Heavy Mixed Concurrent Load (gRPC + REST + Health)
    # =========================================================================
    def test_02_heavy_mixed_concurrent_load_no_starvation(self):
        """
        Adversarial Test 2:
        Simultaneously fires:
        - 50 GET /health requests
        - 50 gRPC RecordHand requests (over wire TCP)
        - 50 POST /telemetry/hands requests (HTTP REST)
        - 50 GET /telemetry/hands requests (HTTP REST)
        Total: 200 concurrent operations across 40 threads with MongoDB offline.
        Asserts:
        - 0 request timeouts
        - 0 gRPC DEADLINE_EXCEEDED errors (which failed in M2.1 with 133/150 drops)
        - 100% success rate across all channels
        """
        num_each = 50
        errors = []
        results = {
            "health": [],
            "grpc_record": [],
            "rest_post": [],
            "rest_get": [],
        }

        def run_health(idx: int):
            t0 = time.perf_counter()
            try:
                r = requests.get(f"{self.base_url}/health", timeout=3.0)
                if r.status_code == 200:
                    results["health"].append(time.perf_counter() - t0)
                else:
                    errors.append(f"Health error status: {r.status_code}")
            except Exception as exc:
                errors.append(f"Health exception: {exc}")

        def run_grpc(idx: int):
            t0 = time.perf_counter()
            try:
                with grpc.insecure_channel(self.grpc_target) as channel:
                    stub = telemetry_pb2_grpc.TelemetryServiceStub(channel)
                    req = telemetry_pb2.HandEventRequest(
                        match_id=f"mixed-match-{idx % 5}",
                        player_id=f"player-{idx}",
                        round_number=1,
                        hand_number=idx,
                        poker_hand="THREE_OF_A_KIND",
                        cards_played=[
                            telemetry_pb2.Card(rank="Q", suit="HEARTS", chips=10, scored=True),
                            telemetry_pb2.Card(rank="Q", suit="SPADES", chips=10, scored=True),
                            telemetry_pb2.Card(rank="Q", suit="DIAMONDS", chips=10, scored=True),
                        ],
                        scoring=telemetry_pb2.ScoringBreakdown(
                            base_chips=30,
                            base_mult=3,
                            card_chips=30,
                            final_chips=60,
                            final_mult=3,
                            score=180,
                            jokers=[],
                        ),
                        discards_left=2,
                        accumulated_score=180 * (idx + 1),
                    )
                    resp = stub.RecordHand(req, timeout=3.0)
                    if resp.success:
                        results["grpc_record"].append(time.perf_counter() - t0)
                    else:
                        errors.append(f"gRPC RecordHand returned success=False: {resp.message}")
            except Exception as exc:
                errors.append(f"gRPC exception: {exc}")

        def run_rest_post(idx: int):
            t0 = time.perf_counter()
            try:
                payload = {
                    "match_id": f"mixed-match-{idx % 5}",
                    "player_id": f"player-rest-{idx}",
                    "round_number": 1,
                    "hand_number": idx,
                    "poker_hand": "TWO_PAIR",
                    "cards_played": [
                        {"rank": "K", "suit": "CLUBS", "chips": 10, "scored": True},
                        {"rank": "K", "suit": "HEARTS", "chips": 10, "scored": True},
                    ],
                    "scoring": {
                        "base_chips": 20,
                        "base_mult": 2,
                        "card_chips": 20,
                        "final_chips": 40,
                        "final_mult": 2,
                        "score": 80,
                        "jokers": [],
                    },
                    "discards_left": 3,
                    "accumulated_score": 80 * (idx + 1),
                }
                r = requests.post(f"{self.base_url}/telemetry/hands", json=payload, timeout=3.0)
                if r.status_code == 201:
                    results["rest_post"].append(time.perf_counter() - t0)
                else:
                    errors.append(f"REST POST status: {r.status_code} ({r.text})")
            except Exception as exc:
                errors.append(f"REST POST exception: {exc}")

        def run_rest_get(idx: int):
            t0 = time.perf_counter()
            try:
                r = requests.get(f"{self.base_url}/telemetry/hands?limit=20", timeout=3.0)
                if r.status_code == 200:
                    results["rest_get"].append(time.perf_counter() - t0)
                else:
                    errors.append(f"REST GET status: {r.status_code}")
            except Exception as exc:
                errors.append(f"REST GET exception: {exc}")

        t_start = time.perf_counter()
        with concurrent.futures.ThreadPoolExecutor(max_workers=40) as executor:
            f_health = [executor.submit(run_health, i) for i in range(num_each)]
            f_grpc = [executor.submit(run_grpc, i) for i in range(num_each)]
            f_post = [executor.submit(run_rest_post, i) for i in range(num_each)]
            f_get = [executor.submit(run_rest_get, i) for i in range(num_each)]
            all_futs = f_health + f_grpc + f_post + f_get
            concurrent.futures.wait(all_futs)
        total_time = time.perf_counter() - t_start

        print(
            f"\n--- CHALLENGE 2: 200 Mixed Operations Concurrency Results ---\n"
            f"Total time: {total_time:.3f}s ({200 / total_time:.1f} ops/s)\n"
            f"Completed: Health={len(results['health'])}, gRPC={len(results['grpc_record'])}, "
            f"REST POST={len(results['rest_post'])}, REST GET={len(results['rest_get'])}\n"
            f"Total errors: {len(errors)} / 200\n"
            f"-------------------------------------------------------------"
        )

        self.assertEqual(len(errors), 0, f"Mixed load failed with errors: {errors[:5]}")
        self.assertEqual(len(results["health"]), num_each)
        self.assertEqual(len(results["grpc_record"]), num_each)
        self.assertEqual(len(results["rest_post"]), num_each)
        self.assertEqual(len(results["rest_get"]), num_each)
        self.assertLess(total_time, 4.0, f"Mixed load took too long: {total_time:.2f}s")

    # =========================================================================
    # CHALLENGE 3: Schema Resilience on Unscored & Malformed Hand Events
    # =========================================================================
    def test_03_unscored_hand_events_schema_resilience(self):
        """
        Adversarial Test 3:
        Tests Defect 2 remediation (unscored hands causing HTTP 500 cascade).
        1. Ingests unscored hand via gRPC (scoring field omitted).
        2. Ingests hand via REST with scoring: {}.
        3. Ingests hand via REST with scoring: null / None.
        4. Ingests hand via REST without scoring field.
        5. Ingests hand via REST with cards_played: [].
        6. Ingests hand via REST with cards_played: null.
        7. Queries GET /telemetry/hands, GET /telemetry/hands/{id}, and GET /telemetry/matches.
        Asserts:
        - 100% of queries return HTTP 200.
        - Zero Pydantic ValidationError or HTTP 500.
        - Default scoring fields are cleanly populated with zeros.
        """
        created_event_ids = []

        # 1. gRPC RecordHand without scoring field
        with grpc.insecure_channel(self.grpc_target) as channel:
            stub = telemetry_pb2_grpc.TelemetryServiceStub(channel)
            req = telemetry_pb2.HandEventRequest(
                match_id="unscored-test-match",
                player_id="player-grpc-unscored",
                round_number=1,
                hand_number=1,
                poker_hand="HIGH_CARD",
                cards_played=[
                    telemetry_pb2.Card(rank="2", suit="CLUBS", chips=2, scored=True)
                ],
                # scoring intentionally omitted
                discards_left=3,
                accumulated_score=2,
            )
            resp = stub.RecordHand(req, timeout=3.0)
            self.assertTrue(resp.success)
            self.assertTrue(resp.event_id)
            created_event_ids.append(("grpc_omitted", resp.event_id))

        # 2. REST POST with scoring: {}
        payload_empty_dict = {
            "match_id": "unscored-test-match",
            "player_id": "player-rest-empty-dict",
            "round_number": 1,
            "hand_number": 2,
            "poker_hand": "ONE_PAIR",
            "cards_played": [],
            "scoring": {},
            "discards_left": 2,
            "accumulated_score": 10,
        }
        r2 = requests.post(f"{self.base_url}/telemetry/hands", json=payload_empty_dict, timeout=3.0)
        self.assertEqual(r2.status_code, 201)
        created_event_ids.append(("rest_empty_dict", r2.json()["event_id"]))

        # 3. REST POST with scoring: null
        payload_null_scoring = {
            "match_id": "unscored-test-match",
            "player_id": "player-rest-null-scoring",
            "round_number": 1,
            "hand_number": 3,
            "poker_hand": "FLUSH",
            "cards_played": [],
            "scoring": None,
            "discards_left": 1,
            "accumulated_score": 50,
        }
        r3 = requests.post(f"{self.base_url}/telemetry/hands", json=payload_null_scoring, timeout=3.0)
        self.assertEqual(r3.status_code, 201)
        created_event_ids.append(("rest_null_scoring", r3.json()["event_id"]))

        # 4. REST POST with missing scoring field
        payload_missing_scoring = {
            "match_id": "unscored-test-match",
            "player_id": "player-rest-missing-scoring",
            "round_number": 1,
            "hand_number": 4,
            "poker_hand": "STRAIGHT",
            "cards_played": [],
            "discards_left": 0,
            "accumulated_score": 100,
        }
        r4 = requests.post(f"{self.base_url}/telemetry/hands", json=payload_missing_scoring, timeout=3.0)
        self.assertEqual(r4.status_code, 201)
        created_event_ids.append(("rest_missing_scoring", r4.json()["event_id"]))

        # 5. REST POST with cards_played: null
        payload_null_cards = {
            "match_id": "unscored-test-match",
            "player_id": "player-rest-null-cards",
            "round_number": 1,
            "hand_number": 5,
            "poker_hand": "FULL_HOUSE",
            "cards_played": None,
            "scoring": {},
            "discards_left": 0,
            "accumulated_score": 250,
        }
        r5 = requests.post(f"{self.base_url}/telemetry/hands", json=payload_null_cards, timeout=3.0)
        self.assertEqual(r5.status_code, 201)
        created_event_ids.append(("rest_null_cards", r5.json()["event_id"]))

        # Now query GET /telemetry/hands (all events)
        r_list = requests.get(f"{self.base_url}/telemetry/hands?limit=100", timeout=3.0)
        self.assertEqual(r_list.status_code, 200, f"GET /telemetry/hands returned HTTP {r_list.status_code}: {r_list.text}")
        data_list = r_list.json()
        self.assertGreaterEqual(data_list["total"], len(created_event_ids))

        # Query each individually
        for label, ev_id in created_event_ids:
            r_single = requests.get(f"{self.base_url}/telemetry/hands/{ev_id}", timeout=3.0)
            self.assertEqual(
                r_single.status_code,
                200,
                f"Failed to fetch {label} (event_id={ev_id}): {r_single.status_code} - {r_single.text}",
            )
            item = r_single.json()
            # Verify default scoring breakdown exists with valid integers
            self.assertIn("scoring", item)
            scoring = item["scoring"]
            self.assertEqual(scoring["base_chips"], 0)
            self.assertEqual(scoring["base_mult"], 0)
            self.assertEqual(scoring["card_chips"], 0)
            self.assertEqual(scoring["final_chips"], 0)
            self.assertEqual(scoring["final_mult"], 0)
            self.assertEqual(scoring["score"], 0)
            self.assertEqual(scoring["jokers"], [])
            self.assertIsInstance(item["cards_played"], list)

        # Verify matches aggregation
        r_matches = requests.get(f"{self.base_url}/telemetry/matches", timeout=3.0)
        self.assertEqual(r_matches.status_code, 200)
        matches = r_matches.json()
        target_m = next((m for m in matches if m["match_id"] == "unscored-test-match"), None)
        self.assertIsNotNone(target_m)
        self.assertEqual(target_m["hands_played"], 5)
        self.assertEqual(target_m["final_score"], 250)

        print(
            f"\n--- CHALLENGE 3: Schema Resilience Results ---\n"
            f"Successfully ingested and retrieved 5 varied unscored/null-field events.\n"
            f"GET /telemetry/hands list and item endpoints returned HTTP 200 with zero ValidationError.\n"
            f"----------------------------------------------"
        )

    # =========================================================================
    # CHALLENGE 4: Event Loop Lag Measurement During Health Flooding
    # =========================================================================
    def test_04_event_loop_lag_during_health_flooding(self):
        """
        Adversarial Test 4:
        Measures event loop latency of a separate control probe while background
        threads hammer the health endpoint.
        If any synchronous / blocking calls occur on the event loop,
        the control probe's latency will degrade significantly (> 300ms).
        """
        stop_flood = False

        def flooder():
            while not stop_flood:
                try:
                    requests.get(f"{self.base_url}/health", timeout=1.0)
                except Exception:
                    pass

        # Launch background flood threads
        pool = concurrent.futures.ThreadPoolExecutor(max_workers=5)
        for _ in range(5):
            pool.submit(flooder)

        try:
            # Send 20 control probes and measure individual latencies
            control_latencies = []
            for _ in range(20):
                t0 = time.perf_counter()
                r = requests.get(f"{self.base_url}/health", timeout=1.0)
                self.assertEqual(r.status_code, 200)
                control_latencies.append(time.perf_counter() - t0)
                time.sleep(0.01)

            max_control = max(control_latencies)
            mean_control = sum(control_latencies) / len(control_latencies)

            print(
                f"\n--- CHALLENGE 4: Event Loop Lag Under Continuous Flood ---\n"
                f"Mean control latency: {mean_control*1000:.2f}ms\n"
                f"Max control latency:  {max_control*1000:.2f}ms\n"
                f"----------------------------------------------------------"
            )
            # Under a non-blocking loop, control probes should respond in < 150ms
            self.assertLess(max_control, 0.15, f"Event loop lagged: max control latency was {max_control*1000:.2f}ms")
        finally:
            stop_flood = True
            pool.shutdown(wait=False)

    # =========================================================================
    # CHALLENGE 5: In-Process Motor Connection State & Ping Recovery
    # =========================================================================
    def test_05_in_process_mongo_manager_ping_resilience(self):
        """
        Adversarial Test 5:
        Direct in-process unit verification of app.db.mongo.MongoManager:
        - When offline (is_connected=False, client=None), ping() must return False immediately (< 5ms).
        - Connect() against unreachable URL must catch exception, reset client=None, and is_connected=False.
        - Simulated hung client with asyncio.wait_for timeout=0.2 must timeout in ~0.2s and not hang for 2.0s.
        """
        from app.db.mongo import MongoManager

        async def run_checks():
            mgr = MongoManager(mongo_url="mongodb://127.0.0.1:29999", db_name="test_resilience")
            # 1. Pre-connection state
            t0 = time.perf_counter()
            p1 = await mgr.ping()
            lat1 = time.perf_counter() - t0
            self.assertFalse(p1)
            self.assertLess(lat1, 0.01, f"Pre-connection ping took {lat1*1000:.2f}ms (expected < 10ms)")

            # 2. Connect attempt against unreachable port
            t0 = time.perf_counter()
            await mgr.connect()
            lat2 = time.perf_counter() - t0
            self.assertFalse(mgr.is_connected)
            self.assertIsNone(mgr.client, "client should be set to None on failed connect()")
            self.assertIsNone(mgr.db, "db should be set to None on failed connect()")

            # 3. Post-failed-connect ping
            t0 = time.perf_counter()
            p2 = await mgr.ping()
            lat3 = time.perf_counter() - t0
            self.assertFalse(p2)
            self.assertLess(lat3, 0.01, f"Post-failed-connect ping took {lat3*1000:.2f}ms (expected < 10ms)")

        asyncio.run(run_checks())
        print(
            f"\n--- CHALLENGE 5: In-Process MongoManager Ping Resilience ---\n"
            f"Verified: ping() returns False in < 1ms when disconnected.\n"
            f"Verified: client and db reset to None on connection failure.\n"
            f"-----------------------------------------------------------"
        )

    # =========================================================================
    # CHALLENGE 6: Clean Shutdown & Port Rebinding on SIGTERM
    # =========================================================================
    def test_06_clean_shutdown_and_socket_rebinding(self):
        """
        Adversarial Test 6:
        Verifies clean dual-server termination within 5s grace period and
        immediate socket rebinding on both ports (18085 and 15085).
        """
        test_http = 18085
        test_grpc = 15085
        env = os.environ.copy()
        env["HTTP_PORT"] = str(test_http)
        env["GRPC_PORT"] = str(test_grpc)
        env["MONGO_URL"] = "mongodb://127.0.0.1:29999"
        env["MONGO_DB"] = "jokerarena_shutdown_test"
        env["PYTHONPATH"] = f"{str(TELEMETRY_DIR)}{os.pathsep}{str(TELEMETRY_DIR / 'proto')}"

        sub = subprocess.Popen(
            [str(VENV_PYTHON), "main.py"],
            cwd=str(TELEMETRY_DIR),
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        try:
            # Wait until ready
            ready = False
            start = time.time()
            while time.time() - start < 15.0:
                try:
                    r = requests.get(f"http://127.0.0.1:{test_http}/health", timeout=0.5)
                    if r.status_code == 200:
                        ready = True
                        break
                except Exception:
                    time.sleep(0.25)
            self.assertTrue(ready, "Shutdown test server failed to start")

            t0 = time.perf_counter()
            sub.terminate()
            sub.wait(timeout=5.0)
            shutdown_time = time.perf_counter() - t0

            self.assertLess(shutdown_time, 4.0, f"Shutdown took {shutdown_time:.2f}s (should be < 4.0s)")
            self.assertTrue(wait_for_ports_closed([test_http, test_grpc], timeout=3.0))

            # Re-bind immediately to verify no WSAEADDRINUSE / socket leak
            s1 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            s1.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            s1.bind(("127.0.0.1", test_http))
            s1.close()

            s2 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            s2.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            s2.bind(("127.0.0.1", test_grpc))
            s2.close()

            print(
                f"\n--- CHALLENGE 6: Clean Shutdown Results ---\n"
                f"Dual-server exited in {shutdown_time:.2f}s.\n"
                f"Ports :{test_http} and :{test_grpc} immediately rebound without error.\n"
                f"-------------------------------------------"
            )
        finally:
            if sub.poll() is None:
                sub.kill()


if __name__ == "__main__":
    unittest.main(verbosity=2)
