"""
Slice V-2/V-3 acceptance test: plays complete matches over the real WebSocket (connector C2)
against a running Game Engine (docker compose up -d).

The test is skipped automatically when the Game Engine is not reachable.

Run:  python -m pytest tests/test_ws_game_flow.py -v
"""
import json
import sys
import unittest
import uuid
from pathlib import Path

import requests

PROJECT_ROOT = Path(__file__).resolve().parent.parent
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from tests.config import GAME_ENGINE_URL, GAME_ENGINE_WS_URL, INITIAL_HANDS  # noqa: E402

try:
    from websockets.sync.client import connect as ws_connect
except ImportError:  # pragma: no cover
    ws_connect = None


def _engine_up() -> bool:
    try:
        return requests.get(f"{GAME_ENGINE_URL}/health", timeout=2).status_code == 200
    except requests.RequestException:
        return False


def _register_and_create_room(target_score: int, with_bot: bool = True):
    alias = f"Ws_{uuid.uuid4().hex[:8]}"
    res = requests.post(f"{GAME_ENGINE_URL}/api/players", json={"alias": alias}, timeout=5)
    assert res.status_code in (200, 201), res.text
    player_id = res.json()["id"]
    res = requests.post(
        f"{GAME_ENGINE_URL}/api/rooms",
        json={"player_id": player_id, "target_score": target_score, "with_bot": with_bot},
        timeout=5,
    )
    assert res.status_code == 201, res.text
    return player_id, res.json()["room_code"]


class Session:
    """Thin helper around a WebSocket connection speaking the JokerArena envelope."""

    def __init__(self, room_code: str, player_id: str):
        self.ws = ws_connect(f"{GAME_ENGINE_WS_URL}/ws/{room_code}", open_timeout=5)
        self.ws.send(json.dumps({"type": "JOIN_ROOM", "payload": {"room_code": room_code, "player_id": player_id}}))

    def recv(self):
        msg = json.loads(self.ws.recv(timeout=5))
        return msg["type"], msg["payload"]

    def expect(self, wanted: str):
        typ, payload = self.recv()
        assert typ == wanted, f"expected {wanted}, got {typ}: {payload}"
        return payload

    def send(self, typ: str, indices):
        self.ws.send(json.dumps({"type": typ, "payload": {"card_indices": indices}}))

    def close(self):
        self.ws.close()


@unittest.skipIf(ws_connect is None, "websockets>=12 is required")
@unittest.skipUnless(_engine_up(), "Game Engine is not running (docker compose up -d)")
class TestWebSocketGameFlow(unittest.TestCase):
    def test_01_initial_state_has_eight_cards_from_a_52_card_deck(self):
        player_id, code = _register_and_create_room(300)
        s = Session(code, player_id)
        try:
            state = s.expect("ROOM_STATE_UPDATE")
            self.assertEqual(state["room_code"], code)
            self.assertEqual(len(state["cards_in_hand"]), 8)
            self.assertEqual(state["deck_count"], 44)
            self.assertEqual((state["hands_left"], state["discards_left"], state["current_score"]), (INITIAL_HANDS, 3, 0))
            self.assertEqual(state["active_jokers"], ["banner", "joker", "the_duo"])
            keys = {(c["rank"], c["suit"]) for c in state["cards_in_hand"]}
            self.assertEqual(len(keys), 8, "cards must be unique")
        finally:
            s.close()

    def test_02_play_hand_scores_with_joker_cascade(self):
        player_id, code = _register_and_create_room(100000)
        s = Session(code, player_id)
        try:
            before = s.expect("ROOM_STATE_UPDATE")
            s.send("PLAY_HAND", [0, 1, 2, 3, 4])
            scored = s.expect("HAND_SCORED")
            after = s.expect("ROOM_STATE_UPDATE")

            # Server-authoritative maths: (base + cards + 30*discards) x ((mult + 4) x duo)
            self.assertEqual([j["id"] for j in scored["jokers"]], ["banner", "joker", "the_duo"])
            self.assertEqual(scored["jokers"][0]["chips_added"], 30 * before["discards_left"])
            chips = scored["base_chips"] + scored["card_chips"] + scored["jokers"][0]["chips_added"]
            mult = (scored["base_mult"] + 4) * (2 if scored["jokers"][2]["triggered"] else 1)
            self.assertEqual(scored["final_chips"], chips)
            self.assertEqual(scored["final_mult"], mult)
            self.assertEqual(scored["score"], chips * mult)

            self.assertEqual(after["current_score"], scored["score"])
            self.assertEqual(after["hands_left"], INITIAL_HANDS - 1)
            self.assertEqual(len(after["cards_in_hand"]), 8)
            self.assertEqual(after["deck_count"], 39)
        finally:
            s.close()

    def test_03_discard_replaces_cards_and_spends_a_discard(self):
        player_id, code = _register_and_create_room(300)
        s = Session(code, player_id)
        try:
            before = s.expect("ROOM_STATE_UPDATE")
            s.send("DISCARD_CARDS", [0, 1])
            after = s.expect("ROOM_STATE_UPDATE")
            self.assertEqual(after["discards_left"], before["discards_left"] - 1)
            self.assertEqual(after["hands_left"], before["hands_left"])
            self.assertEqual(after["current_score"], 0)
            self.assertEqual(len(after["cards_in_hand"]), 8)
            self.assertEqual(after["deck_count"], before["deck_count"] - 2)
        finally:
            s.close()

    def test_04_invalid_selection_is_rejected(self):
        player_id, code = _register_and_create_room(300)
        s = Session(code, player_id)
        try:
            s.expect("ROOM_STATE_UPDATE")
            for bad in ([], [0, 0], [99], [0, 1, 2, 3, 4, 5]):
                s.send("PLAY_HAND", bad)
                err = s.expect("ERROR")
                self.assertEqual(err["code"], "INVALID_SELECTION", bad)
        finally:
            s.close()

    def test_05_win_by_reaching_target_and_persistence(self):
        player_id, code = _register_and_create_room(1)  # any scoring hand wins
        s = Session(code, player_id)
        try:
            s.expect("ROOM_STATE_UPDATE")
            s.send("PLAY_HAND", [0])
            s.expect("HAND_SCORED")
            self.assertEqual(s.expect("ROOM_STATE_UPDATE")["status"], "FINISHED")
            fin = s.expect("MATCH_FINISHED")
            self.assertEqual((fin["result"], fin["reason"], fin["winner_id"]), ("WIN", "TARGET_SCORE_REACHED", player_id))
        finally:
            s.close()
        # REST view of a finished room reports FINISHED.
        details = requests.get(f"{GAME_ENGINE_URL}/api/rooms/{code}", timeout=5).json()
        self.assertEqual(details["status"], "FINISHED")

    def test_06_lose_when_hands_are_exhausted(self):
        player_id, code = _register_and_create_room(100_000)  # unreachable in 20 hands
        s = Session(code, player_id)
        try:
            s.expect("ROOM_STATE_UPDATE")
            for _ in range(INITIAL_HANDS):
                s.send("PLAY_HAND", [0, 1, 2, 3, 4])
                s.expect("HAND_SCORED")
                s.expect("ROOM_STATE_UPDATE")
            fin = s.expect("MATCH_FINISHED")
            self.assertEqual((fin["result"], fin["reason"]), ("LOSE", "HANDS_EXHAUSTED"))
            self.assertFalse(fin["target_reached"])
            s.send("PLAY_HAND", [0])
            self.assertEqual(s.expect("ERROR")["code"], "MATCH_FINISHED")
        finally:
            s.close()

    def test_07_state_survives_reconnect(self):
        player_id, code = _register_and_create_room(100000)
        s1 = Session(code, player_id)
        s1.expect("ROOM_STATE_UPDATE")
        s1.send("DISCARD_CARDS", [0])
        first = s1.expect("ROOM_STATE_UPDATE")
        s1.close()
        s2 = Session(code, player_id)
        try:
            again = s2.expect("ROOM_STATE_UPDATE")
            self.assertEqual(again["cards_in_hand"], first["cards_in_hand"])
            self.assertEqual(again["discards_left"], 2)
        finally:
            s2.close()

    def test_08_unknown_room_and_player_errors(self):
        player_id, code = _register_and_create_room(300)
        s = Session("JKR-ZZZZ", player_id)
        try:
            self.assertEqual(s.expect("ERROR")["code"], "ROOM_NOT_FOUND")
        finally:
            s.close()
        s = Session(code, "999999999")
        try:
            self.assertEqual(s.expect("ERROR")["code"], "UNKNOWN_PLAYER")
        finally:
            s.close()


if __name__ == "__main__":
    unittest.main()
