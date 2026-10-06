#!/usr/bin/env python3
"""
JokerArena E2E Acceptance Test Runner
Orchestrates test suites across Tiers 1-4 for 40% Prototype 1.

Usage:
  python tests/e2e_runner.py --tier all
  python tests/e2e_runner.py --tier 1
  python tests/e2e_runner.py --tier 2
  python tests/e2e_runner.py --tier 3
  python tests/e2e_runner.py --tier 4
  python tests/e2e_runner.py --mock          # Test contract invariants in mock mode
  python tests/e2e_runner.py --verify-syntax # Validate syntax and suite assembly
"""
import argparse
import os
import sys
import time
import unittest
from pathlib import Path
from typing import Dict, List

# Ensure project root is in sys.path
PROJECT_ROOT = Path(__file__).resolve().parent.parent
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from tests.test_tier1_features import TestTier1Features
from tests.test_tier2_boundaries import TestTier2Boundaries
from tests.test_tier3_integration import TestTier3Integration
from tests.test_tier4_scenarios import TestTier4Scenarios

TIER_MAP = {
    "1": ("Tier 1: Feature Coverage (Happy Path)", TestTier1Features),
    "2": ("Tier 2: Boundary & Corner Cases", TestTier2Boundaries),
    "3": ("Tier 3: Cross-Feature Combinations", TestTier3Integration),
    "4": ("Tier 4: Real-World Workload Scenarios", TestTier4Scenarios),
}


class ColoredTestResult(unittest.TextTestResult):
    """Custom test result collector that tracks stats per tier."""

    def __init__(self, stream, descriptions, verbosity):
        super().__init__(stream, descriptions, verbosity)
        self.success_count = 0

    def addSuccess(self, test):
        super().addSuccess(test)
        self.success_count += 1


def run_tier(tier_key: str, verbosity: int = 1) -> Dict[str, any]:
    tier_name, test_class = TIER_MAP[tier_key]
    suite = unittest.TestLoader().loadTestsFromTestCase(test_class)

    stream = open(os.devnull, "w") if verbosity == 0 else sys.stdout
    runner = unittest.TextTestRunner(stream=stream, resultclass=ColoredTestResult, verbosity=verbosity)

    start_time = time.time()
    result = runner.run(suite)
    elapsed = time.time() - start_time

    return {
        "tier": tier_key,
        "name": tier_name,
        "total": result.testsRun,
        "passed": result.success_count,
        "failed": len(result.failures),
        "errors": len(result.errors),
        "skipped": len(result.skipped),
        "elapsed": elapsed,
        "success": result.wasSuccessful(),
    }


def print_summary_table(results: List[Dict[str, any]]):
    print("\n" + "=" * 80)
    print("                     JOKERARENA E2E ACCEPTANCE TEST SUMMARY")
    print("=" * 80)
    print(f"{'Tier':<8} | {'Description':<38} | {'Pass':<6} | {'Fail':<6} | {'Skip':<6} | {'Time':<7}")
    print("-" * 80)

    total_pass = 0
    total_fail = 0
    total_skip = 0
    total_time = 0.0

    for r in results:
        total_pass += r["passed"]
        total_fail += r["failed"] + r["errors"]
        total_skip += r["skipped"]
        total_time += r["elapsed"]
        status_flag = "PASS" if r["success"] else "FAIL"
        print(f"Tier {r['tier']:<3} | {r['name'][:38]:<38} | {r['passed']:<6} | {r['failed'] + r['errors']:<6} | {r['skipped']:<6} | {r['elapsed']:<6.2f}s")

    print("-" * 80)
    print(f"{'TOTAL':<8} | {'All Configured Tiers':<38} | {total_pass:<6} | {total_fail:<6} | {total_skip:<6} | {total_time:<6.2f}s")
    print("=" * 80)

    all_passed = all(r["success"] for r in results)
    if all_passed:
        print("[SUCCESS] All executed tiers passed acceptance criteria successfully.\n")
    else:
        print("[FAILURE] One or more tiers failed acceptance criteria.\n")

    return all_passed


def main():
    parser = argparse.ArgumentParser(description="JokerArena E2E Acceptance Test Runner")
    parser.add_argument(
        "--tier",
        choices=["1", "2", "3", "4", "all"],
        default="all",
        help="Specific tier to execute (default: all)",
    )
    parser.add_argument(
        "--mock",
        action="store_true",
        help="Run tests in mock/contract simulation mode (no live containers required)",
    )
    parser.add_argument(
        "--verify-syntax",
        action="store_true",
        help="Verify test suite assembly and syntax without executing network calls",
    )
    parser.add_argument(
        "-v", "--verbose",
        action="count",
        default=1,
        help="Verbosity level (-v or -vv)",
    )
    args = parser.parse_args()

    if args.mock:
        os.environ["E2E_MOCK_MODE"] = "true"

    if args.verify_syntax:
        print("[VERIFY] Validating test suite assembly...")
        for tier_key, (name, cls_obj) in TIER_MAP.items():
            loader = unittest.TestLoader()
            suite = loader.loadTestsFromTestCase(cls_obj)
            print(f"  [OK] {name}: {suite.countTestCases()} test cases assembled.")
        print("[VERIFY] All test suites assembled with zero syntax errors.")
        return 0

    tiers_to_run = ["1", "2", "3", "4"] if args.tier == "all" else [args.tier]
    results = []

    print(f"\n[RUNNING] Executing JokerArena E2E Test Suite (Tier: {args.tier}, Mode: {'MOCK' if args.mock else 'LIVE'})...")

    for t in tiers_to_run:
        res = run_tier(t, verbosity=args.verbose)
        results.append(res)

    all_passed = print_summary_table(results)
    return 0 if all_passed else 1


if __name__ == "__main__":
    sys.exit(main())
