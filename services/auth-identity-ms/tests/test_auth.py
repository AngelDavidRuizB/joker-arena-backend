import time
import uuid

import jwt
import pytest
from fastapi.testclient import TestClient

from app.config import Settings
from app.keys import SigningKey
from app.main import Services, create_app
from app.store import AliasTaken, Player, utcnow


class FakePlayers:
    def __init__(self):
        self.rows = {}
        self.down = False

    def _guard(self):
        if self.down:
            raise ConnectionError("pg down")

    async def create(self, alias, role):
        self._guard()
        if any(p.alias.lower() == alias.lower() for p in self.rows.values()):
            raise AliasTaken(alias)
        p = Player(str(uuid.uuid4()), alias, role, utcnow())
        self.rows[p.player_id] = p
        return p

    async def by_alias(self, alias):
        self._guard()
        return next((p for p in self.rows.values() if p.alias.lower() == alias.lower()), None)

    async def by_id(self, pid):
        return self.rows.get(pid)

    async def ping(self):
        self._guard()


class FakeSessions:
    def __init__(self):
        self.active, self.revoked = {}, set()

    async def start(self, jti, pid, ttl):
        self.active[jti] = pid

    async def revoke(self, jti, ttl):
        self.active.pop(jti, None)
        self.revoked.add(jti)

    async def is_revoked(self, jti):
        return jti in self.revoked

    async def ping(self):
        pass


class FakePublisher:
    def __init__(self):
        self.events = []

    connected = True

    async def publish(self, key, payload):
        self.events.append((key, payload))


KEY = SigningKey.generate()


@pytest.fixture
def ctx():
    cfg = Settings(admin_password="secreto-admin", token_ttl_seconds=3600)
    players, sessions, pub = FakePlayers(), FakeSessions(), FakePublisher()
    app = create_app(Services(players, sessions, pub, KEY, cfg), cfg)
    with TestClient(app) as client:
        yield client, players, sessions, pub


def test_register_issues_a_verifiable_rs256_token_and_publishes_the_event(ctx):
    client, _, sessions, pub = ctx
    r = client.post("/auth/register", json={"alias": "Ana"})
    assert r.status_code == 201
    body = r.json()
    assert body["player"]["alias"] == "Ana" and body["player"]["role"] == "player"
    jwks = client.get("/.well-known/jwks.json").json()["keys"][0]
    header = jwt.get_unverified_header(body["token"])
    assert header["alg"] == "RS256" and header["kid"] == jwks["kid"]
    pub_key = jwt.algorithms.RSAAlgorithm.from_jwk(jwks)
    claims = jwt.decode(body["token"], pub_key, algorithms=["RS256"], issuer="jokerarena-auth")
    assert claims["sub"] == body["player"]["player_id"] and claims["alias"] == "Ana"
    assert claims["exp"] > time.time()
    assert pub.events == [("auth.player.registered", {"player_id": claims["sub"], "alias": "Ana"})]
    assert claims["jti"] in sessions.active


def test_register_twice_is_a_conflict_and_login_works(ctx):
    client, *_ = ctx
    client.post("/auth/register", json={"alias": "Beto"})
    assert client.post("/auth/register", json={"alias": "beto"}).json()["error"] == "alias_taken"
    r = client.post("/auth/login", json={"alias": "BETO"})
    assert r.status_code == 200 and r.json()["player"]["alias"] == "Beto"
    assert client.post("/auth/login", json={"alias": "Nadie"}).status_code == 404


@pytest.mark.parametrize("alias", ["a", "x" * 31, "con espacio", "<script>", "bot", "Bot-JokerBot"])
def test_invalid_aliases_are_rejected(ctx, alias):
    client, *_ = ctx
    r = client.post("/auth/register", json={"alias": alias})
    assert r.status_code == 400 and r.json()["error"] in ("invalid_alias", "reserved_alias")


def test_me_and_logout_revoke_the_session(ctx):
    client, *_ = ctx
    token = client.post("/auth/register", json={"alias": "Carla"}).json()["token"]
    h = {"Authorization": f"Bearer {token}"}
    assert client.get("/auth/me", headers=h).json()["alias"] == "Carla"
    assert client.post("/auth/logout", headers=h).status_code == 204
    assert client.get("/auth/me", headers=h).status_code == 401
    assert client.get("/auth/me").json()["error"] == "unauthorized"


def test_admin_requires_the_admin_password(ctx):
    client, *_ = ctx
    assert client.post("/auth/register", json={"alias": "admin"}).status_code == 401
    r = client.post("/auth/register", json={"alias": "admin", "password": "secreto-admin"})
    assert r.status_code == 201 and r.json()["player"]["role"] == "admin"
    assert client.post("/auth/login", json={"alias": "admin", "password": "otra"}).status_code == 401


def test_database_outage_answers_503_and_health_degrades(ctx):
    client, players, *_ = ctx
    players.down = True
    r = client.post("/auth/register", json={"alias": "Dani"})
    assert r.status_code == 503 and r.json()["error"] == "store_unavailable"
    h = client.get("/health").json()
    assert h["status"] == "degraded" and h["deps"]["postgres"] == "down" and h["service"] == "auth-identity-ms"


def test_bad_json_uses_the_common_error_shape(ctx):
    client, *_ = ctx
    r = client.post("/auth/register", json={"nombre": "x"})
    assert r.status_code == 400 and set(r.json()) == {"error", "message"}


def test_signing_key_is_persisted(tmp_path):
    path = str(tmp_path / "k.pem")
    a = SigningKey.load_or_create(path)
    b = SigningKey.load_or_create(path)
    assert a.kid == b.kid


def test_verify_endpoint_for_the_gateway(ctx):
    client, *_ = ctx
    player = client.post("/auth/register", json={"alias": "Eva"}).json()
    admin = client.post("/auth/register", json={"alias": "admin", "password": "secreto-admin"}).json()
    ok = client.get("/auth/verify", headers={"Authorization": f"Bearer {player['token']}"})
    assert ok.status_code == 204 and ok.headers["X-Player-Alias"] == "Eva" and ok.headers["X-Player-Role"] == "player"
    # WebSocket: el token llega en la query de la URI original
    ws = client.get("/auth/verify", headers={"X-Original-URI": f"/admin/cluster/metrics?token={admin['token']}", "X-Required-Role": "admin"})
    assert ws.status_code == 204 and ws.headers["X-Player-Role"] == "admin"
    forbidden = client.get("/auth/verify", headers={"Authorization": f"Bearer {player['token']}", "X-Required-Role": "admin"})
    assert forbidden.status_code == 403
    assert client.get("/auth/verify").status_code == 401
