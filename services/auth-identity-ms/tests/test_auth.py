import time
import uuid

import jwt
import pytest
from fastapi.testclient import TestClient

from app.config import Settings
from app.keys import SigningKey
from app.main import Services, create_app
from app.store import AliasTaken, Player, utcnow

PASSWORD = "secreta123"
ADMIN_PASSWORD = "secreto-admin"


class FakePlayers:
    def __init__(self):
        self.rows = {}
        self.down = False
        self.schema_ready = False

    def _guard(self):
        if self.down:
            raise ConnectionError("pg down")

    async def ensure_schema(self):
        self._guard()
        self.schema_ready = True

    async def create(self, alias, role, password_hash=None):
        self._guard()
        if any(p.alias.lower() == alias.lower() for p in self.rows.values()):
            raise AliasTaken(alias)
        p = Player(str(uuid.uuid4()), alias, role, utcnow(), password_hash)
        self.rows[p.player_id] = p
        return p

    async def attach_password(self, alias, password_hash):
        self._guard()
        p = next((p for p in self.rows.values() if p.alias.lower() == alias.lower()), None)
        if p is None or p.password_hash:
            return None
        p.password_hash = password_hash
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
    cfg = Settings(admin_password=ADMIN_PASSWORD, token_ttl_seconds=3600)
    players, sessions, pub = FakePlayers(), FakeSessions(), FakePublisher()
    app = create_app(Services(players, sessions, pub, KEY, cfg), cfg)
    with TestClient(app) as client:
        yield client, players, sessions, pub


def register(client, user, password=PASSWORD, repeat=None):
    body = {"user": user, "password": password, "repeat_password": password if repeat is None else repeat}
    return client.post("/auth/register", json=body)


def login(client, user, password=PASSWORD):
    return client.post("/auth/login", json={"user": user, "password": password})


def test_register_issues_a_verifiable_rs256_token_and_publishes_the_event(ctx):
    client, _, sessions, pub = ctx
    r = register(client, "Ana")
    assert r.status_code == 201
    body = r.json()
    assert body["player"]["alias"] == "Ana" and body["player"]["user"] == "Ana"
    assert body["player"]["role"] == "player"
    jwks = client.get("/.well-known/jwks.json").json()["keys"][0]
    header = jwt.get_unverified_header(body["token"])
    assert header["alg"] == "RS256" and header["kid"] == jwks["kid"]
    pub_key = jwt.algorithms.RSAAlgorithm.from_jwk(jwks)
    claims = jwt.decode(body["token"], pub_key, algorithms=["RS256"], issuer="jokerarena-auth")
    assert claims["sub"] == body["player"]["player_id"] and claims["alias"] == "Ana"
    assert claims["exp"] > time.time()
    assert pub.events == [("auth.player.registered", {"player_id": claims["sub"], "alias": "Ana"})]
    assert claims["jti"] in sessions.active


def test_the_password_is_stored_as_a_hash_and_never_in_clear(ctx):
    client, players, *_ = ctx
    register(client, "Hugo")
    player = next(p for p in players.rows.values() if p.alias == "Hugo")
    assert player.password_hash and PASSWORD not in player.password_hash
    assert player.password_hash.startswith("pbkdf2_sha256$")


def test_register_twice_is_a_conflict_and_login_checks_the_password(ctx):
    client, *_ = ctx
    assert register(client, "Beto").status_code == 201
    assert register(client, "beto").json()["error"] == "alias_taken"
    ok = login(client, "BETO")
    assert ok.status_code == 200 and ok.json()["player"]["alias"] == "Beto"
    wrong = login(client, "Beto", "otra-clave")
    assert wrong.status_code == 401 and wrong.json()["error"] == "invalid_credentials"
    assert login(client, "Nadie").status_code == 404


def test_register_requires_password_policy_and_a_matching_repetition(ctx):
    client, *_ = ctx
    missing = client.post("/auth/register", json={"user": "Ivo"})
    assert missing.status_code == 400 and missing.json()["error"] == "invalid_password"
    short = register(client, "Ivo", "corta")
    assert short.status_code == 400 and "al menos 8" in short.json()["message"]
    mismatch = register(client, "Ivo", PASSWORD, repeat="otra-distinta")
    assert mismatch.status_code == 400 and mismatch.json()["error"] == "password_mismatch"
    assert login(client, "Ivo").status_code == 404  # no se creó ninguna cuenta


def test_legacy_account_without_password_adopts_it_on_register(ctx):
    client, players, *_ = ctx
    legacy = client.portal.call(lambda: players.create("Viejo", "player", None))
    assert legacy.password_hash is None
    assert login(client, "Viejo").json()["error"] == "password_not_set"
    adopted = register(client, "viejo")
    assert adopted.status_code == 201 and adopted.json()["player"]["player_id"] == legacy.player_id
    assert login(client, "viejo").status_code == 200
    assert register(client, "viejo", "otra-clave-larga").status_code == 409



@pytest.mark.parametrize("alias", ["a", "x" * 31, "con espacio", "<script>", "bot", "Bot-JokerBot"])
def test_invalid_users_are_rejected(ctx, alias):
    client, *_ = ctx
    r = register(client, alias)
    assert r.status_code == 400 and r.json()["error"] in ("invalid_alias", "reserved_alias")


def test_me_and_logout_revoke_the_session(ctx):
    client, *_ = ctx
    token = register(client, "Carla").json()["token"]
    h = {"Authorization": f"Bearer {token}"}
    me = client.get("/auth/me", headers=h).json()
    assert me["alias"] == "Carla" and me["user"] == "Carla"
    assert client.post("/auth/logout", headers=h).status_code == 204
    assert client.get("/auth/me", headers=h).status_code == 401
    assert client.get("/auth/me").json()["error"] == "unauthorized"


def test_admin_requires_the_admin_password(ctx):
    client, *_ = ctx
    assert register(client, "admin", "otra-clave-larga").status_code == 401
    r = register(client, "admin", ADMIN_PASSWORD)
    assert r.status_code == 201 and r.json()["player"]["role"] == "admin"
    assert login(client, "admin", "otra").status_code == 401
    assert login(client, "admin", ADMIN_PASSWORD).status_code == 200


def test_database_outage_answers_503_and_health_degrades(ctx):
    client, players, *_ = ctx
    players.down = True
    r = register(client, "Dani")
    assert r.status_code == 503 and r.json()["error"] == "store_unavailable"
    h = client.get("/health").json()
    assert h["status"] == "degraded" and h["deps"]["postgres"] == "down" and h["service"] == "auth-identity-ms"


def test_bad_json_uses_the_common_error_shape(ctx):
    client, *_ = ctx
    for body in ({"nombre": "x"}, {"user": "x"}, {"password": PASSWORD}):
        r = client.post("/auth/register", json=body)
        assert r.status_code == 400 and set(r.json()) == {"error", "message"}
    # el campo `alias` sigue aceptándose por compatibilidad
    assert client.post("/auth/register", json={"alias": "AliasLegado", "password": PASSWORD}).status_code == 201


def test_signing_key_is_persisted(tmp_path):
    path = str(tmp_path / "k.pem")
    a = SigningKey.load_or_create(path)
    b = SigningKey.load_or_create(path)
    assert a.kid == b.kid


def test_verify_endpoint_for_the_gateway(ctx):
    client, *_ = ctx
    player = register(client, "Eva").json()
    admin = register(client, "admin", ADMIN_PASSWORD).json()
    ok = client.get("/auth/verify", headers={"Authorization": f"Bearer {player['token']}"})
    assert ok.status_code == 204 and ok.headers["X-Player-Alias"] == "Eva" and ok.headers["X-Player-Role"] == "player"
    # WebSocket: el token llega en la query de la URI original
    ws = client.get("/auth/verify", headers={"X-Original-URI": f"/admin/cluster/metrics?token={admin['token']}", "X-Required-Role": "admin"})
    assert ws.status_code == 204 and ws.headers["X-Player-Role"] == "admin"
    forbidden = client.get("/auth/verify", headers={"Authorization": f"Bearer {player['token']}", "X-Required-Role": "admin"})
    assert forbidden.status_code == 403
    assert client.get("/auth/verify").status_code == 401

