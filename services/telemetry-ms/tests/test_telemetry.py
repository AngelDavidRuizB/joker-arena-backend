import asyncio
import uuid

import pytest
from fastapi.testclient import TestClient

from app.main import create_app
from app.projector import InvalidEvent, project


class FakeStore:
    def __init__(self):
        self.raw, self.hands, self.matches = {}, {}, {}
        self.down = False
        self.fail_next_hand = False

    def _g(self):
        if self.down:
            raise ConnectionError("mongo down")

    async def seen(self, event_id):
        self._g()
        return event_id in self.raw

    async def save_raw(self, env):
        self._g()
        if env["event_id"] in self.raw:
            return False
        self.raw[env["event_id"]] = env
        return True

    async def upsert_hand(self, doc):
        if self.fail_next_hand:
            self.fail_next_hand = False
            raise ConnectionError("mongo down")
        self.hands[(doc["match_id"], doc["player_id"], doc["hand_number"])] = doc

    async def match_started(self, doc):
        self.matches.setdefault(doc["match_id"], {"status": "in_progress"}).update(doc)

    async def match_ended(self, doc):
        self.matches.setdefault(doc["match_id"], {}).update({**doc, "status": "finished"})

    async def list_hands(self, match_id, player_id, limit):
        self._g()
        out = [h for h in self.hands.values() if (not match_id or h["match_id"] == match_id) and (not player_id or h["player_id"] == player_id)]
        return sorted(out, key=lambda h: h["hand_number"])[:limit]

    async def list_matches(self, limit):
        return list(self.matches.values())[:limit]

    async def get_match(self, mid):
        self._g()
        return self.matches.get(mid)

    async def stats(self):
        return {"matches": len(self.matches), "hands": len(self.hands)}

    async def ping(self):
        self._g()


def env(etype, payload, event_id=None):
    return {"event_id": event_id or str(uuid.uuid4()), "type": etype, "version": 1,
            "occurred_at": "2026-11-03T18:00:00Z", "producer": "test", "payload": payload}


def test_a_failed_projection_is_not_lost_as_a_duplicate():
    store = FakeStore()
    event = env("game.hand.scored", {"match_id": "m1", "player_id": "p1", "hand_number": 1, "hand_type": "PAIR", "score": 10})
    store.fail_next_hand = True
    with pytest.raises(ConnectionError):
        asyncio.run(project(store, event))
    assert asyncio.run(project(store, event)) == "processed"
    assert ("m1", "p1", 1) in store.hands
    assert asyncio.run(project(store, event)) == "duplicate"


def run(coro):
    return asyncio.run(coro)


def hand(n, score=100, player="p1"):
    return env("game.hand.scored", {"match_id": "m1", "room_code": "JKR-AB23", "player_id": player, "is_bot": player == "bot",
                                     "hand_number": n, "hand_type": "PAIR", "score": score, "ruleset_version": 1, "jokers": []})


def test_projects_a_full_match():
    s = FakeStore()
    assert run(project(s, env("game.match.started", {"match_id": "m1", "room_code": "JKR-AB23", "player_ids": ["p1", "bot"]}))) == "processed"
    run(project(s, hand(1)))
    run(project(s, hand(1, player="bot")))
    run(project(s, hand(2, 300)))
    run(project(s, env("game.match.ended", {"match_id": "m1", "room_code": "JKR-AB23", "results": [{"player_id": "p1", "won": True}]})))
    assert len(s.hands) == 3
    assert s.matches["m1"]["status"] == "finished" and s.matches["m1"]["player_ids"] == ["p1", "bot"]


def test_duplicates_and_foreign_events():
    s = FakeStore()
    e = hand(1)
    assert run(project(s, e)) == "processed"
    assert run(project(s, e)) == "duplicate"
    assert run(project(s, env("auth.player.registered", {"player_id": "x"}))) == "ignored"
    assert run(project(s, env("lobby.room.created", {"room_code": "JKR-AB23"}))) == "processed"


@pytest.mark.parametrize("bad", [
    {"type": "game.hand.scored"},
    env("game.hand.scored", {"match_id": "m1"}),
    env("game.hand.scored", "no es objeto"),
    {**hand(1), "occurred_at": "ayer"},
    env("game.match.ended", {"match_id": "m1"}),
])
def test_out_of_contract_events_raise(bad):
    with pytest.raises(InvalidEvent):
        run(project(FakeStore(), bad))


@pytest.fixture
def client():
    s = FakeStore()
    run(project(s, env("game.match.started", {"match_id": "m1", "room_code": "JKR-AB23"})))
    run(project(s, hand(2, 300)))
    run(project(s, hand(1)))
    with TestClient(create_app(store=s, consumer=None), raise_server_exceptions=False) as c:
        yield c, s


def test_match_audit_endpoint_orders_hands(client):
    c, _ = client
    body = c.get("/matches/m1").json()
    assert body["match"]["room_code"] == "JKR-AB23"
    assert [h["hand_number"] for h in body["hands"]] == [1, 2]
    assert c.get("/matches/nada").status_code == 404


def test_hands_filters_and_limits(client):
    c, _ = client
    assert len(c.get("/hands?match_id=m1").json()["hands"]) == 2
    r = c.get("/hands?limit=0")
    assert r.status_code == 400 and r.json()["error"] == "invalid_request"


def test_health_and_outage(client):
    c, s = client
    h = c.get("/health").json()
    assert h["service"] == "telemetry-ms" and h["deps"]["mongo"] == "ok" and h["status"] == "degraded"  # sin broker en tests
    s.down = True
    r = c.get("/matches/m1")
    assert r.status_code == 503 and r.json()["error"] == "store_unavailable"
    assert c.get("/health").json()["deps"]["mongo"] == "down"
