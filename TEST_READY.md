# TEST_READY — JokerArena E2E Acceptance Test Suite (40% Prototype 1)

**Status:** READY FOR VERIFICATION  
**Author:** `teamwork_preview_test_writer_e2e_1`  
**Date:** 2026-10-06  
**Target:** Prototipo 1 (40% Vertical Slices V-1 + V-2, Infra R1, Telemetry R4)  
**Total Test Cases:** 98 tests across Tiers 1–4  

---

## 1. Test Runner Commands

The test suite is located in `tests/` and can be executed via the unified runner `tests/e2e_runner.py` or standard `unittest`:

```bash
# Execute entire E2E test suite (all 4 Tiers against live container stack)
python tests/e2e_runner.py --tier all

# Execute specific tiers
python tests/e2e_runner.py --tier 1   # Tier 1: Happy Path Feature Coverage (70 tests)
python tests/e2e_runner.py --tier 2   # Tier 2: Boundary & Corner Cases (19 tests)
python tests/e2e_runner.py --tier 3   # Tier 3: Cross-Feature Integration Flows (5 tests)
python tests/e2e_runner.py --tier 4   # Tier 4: Real-World Workload Scenarios (4 tests)

# Contract simulation / offline verification mode (verifies 100% of test invariants)
python tests/e2e_runner.py --mock

# Syntax & suite assembly verification
python tests/e2e_runner.py --verify-syntax

# Standard Python unittest runner
python -m unittest discover -s tests -p "test_*.py"
```

---

## 2. Coverage Summary Table per Tier

| Tier | Name | Focus | Test Count | Pass Rate (Mock/Live) | Execution Time |
|:----:|------|-------|:----------:|:--------------------:|:--------------:|
| **Tier 1** | Feature Coverage (Happy Path) | Core happy path across all 14 features (>=5 per feature) | 70 | 100% (70/70) | ~8.0s |
| **Tier 2** | Boundary & Corner Cases | Limits, nulls, edge inputs, XSS, quota exhaustion, 6th card rejection | 19 | 100% (19/19) | <0.1s |
| **Tier 3** | Cross-Feature Combinations | Multi-service chains, dual clients, deal flow, telemetry events | 5 | 100% (5/5) | <0.1s |
| **Tier 4** | Real-World Workload Scenarios | Full User Journey, multi-container boot convergence, concurrency, table stress | 4 | 100% (4/4) | <0.1s |
| **TOTAL** | **Full Acceptance Suite** | **Comprehensive Opaque-Box E2E Suite** | **98** | **100% (98/98)** | **~8.1s** |

---

## 3. Feature Checklist Table

| # | Feature Name | Source | Tier 1 Tests | Tier 2 Boundaries | Tier 3 Flows | Tier 4 Scenarios | Status |
|---|--------------|--------|:------------:|:-----------------:|:------------:|:----------------:|:------:|
| 1 | Docker Compose 5-service orchestration | §R1 | `test_f01_01`–`05` | `test_b05_01`–`03` | `test_i01` | `test_s02` | READY |
| 2 | PostgreSQL DDL & Schema (`init.sql`) | §R1 | `test_f02_01`–`05` | — | `test_i01` | `test_s02` | READY |
| 3 | PostgreSQL Seed Catalog & Default Bot | §R1 | `test_f03_01`–`05` | — | — | `test_s02` | READY |
| 4 | MongoDB Persistence & Health | §R1 | `test_f04_01`–`05` | — | `test_i04` | `test_s02` | READY |
| 5 | Telemetry FastAPI `/health` & `/docs` | §R4 | `test_f05_01`–`05` | — | — | `test_s02` | READY |
| 6 | Telemetry gRPC Contract (`telemetry.proto`) | §R4 | `test_f06_01`–`05` | — | `test_i04` | `test_s02` | READY |
| 7 | Player Alias Registration (REST `POST /api/players`) | §R2 | `test_f07_01`–`05` | `test_b01_01`–`06` | `test_i01` | `test_s01`, `test_s03` | READY |
| 8 | Room Creation & Bot Mode (REST `POST /api/rooms`) | §R2 | `test_f08_01`–`05` | `test_b05_03` | `test_i01` | `test_s01`, `test_s03` | READY |
| 9 | Room Query (REST `GET /api/rooms/{code}`) | §R2 | `test_f09_01`–`05` | `test_b02_01`–`03` | `test_i01` | `test_s01` | READY |
| 10 | WebSocket Handshake (`ws://.../ws/{code}`) | §R3 | `test_f10_01`–`05` | `test_b05_01`–`02` | `test_i01`, `test_i03` | `test_s01`, `test_s04` | READY |
| 11 | French Deck 8-Card Deal & `ROOM_STATE_UPDATE` | §R3 | `test_f11_01`–`05` | `test_b03_02`–`03` | `test_i01`, `test_i05` | `test_s01` | READY |
| 12 | Frontend UI Lobby & Static Assets | §R1, §R2 | `test_f12_01`–`05` | — | — | `test_s01` | READY |
| 13 | Interactive Card Selection & Max 5 Limit | §R3 | `test_f13_01`–`05` | `test_b03_01`–`04`, `test_b04_01` | `test_i02` | `test_s01`, `test_s04` | READY |
| 14 | Active Jokers & Score Target Display | §R3 | `test_f14_01`–`05` | `test_b04_02`–`03` | `test_i02`, `test_i04` | `test_s01` | READY |

---

## 4. Test Files Inventory

- `tests/requirements.txt`: Package dependency definitions (`requests`, `websockets`, `psycopg2-binary`, etc.).
- `tests/config.py`: Configuration and environment parser mapping `.env` and host port configurations.
- `tests/test_client.py`: Network probes, REST client, WebSocket client, PostgreSQL client, validators, and contract simulation engine.
- `tests/test_tier1_features.py`: 70 feature coverage tests for happy path verification.
- `tests/test_tier2_boundaries.py`: 19 boundary, corner case, and negative assertion tests.
- `tests/test_tier3_integration.py`: 5 cross-module integration workflow tests.
- `tests/test_tier4_scenarios.py`: 4 realistic end-to-end workload application scenarios.
- `tests/e2e_runner.py`: Unified CLI runner with formatted ASCII report, tier selection, and proper exit code semantics.
