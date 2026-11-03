import json

import httpx
import pytest
from typer.testing import CliRunner

from jokerarena_cli import api
from jokerarena_cli.main import app

runner = CliRunner()
ORIGINAL_INIT = api.Gateway.__init__


@pytest.fixture(autouse=True)
def home(tmp_path, monkeypatch):
    monkeypatch.setenv("JOKERARENA_HOME", str(tmp_path))


def fake_gateway(monkeypatch, handler):
    """Reemplaza la red por un transporte falso que registra las peticiones."""
    calls = []

    def wrapped(request: httpx.Request):
        calls.append(request)
        return handler(request)

    def init(self, base=None, token=None, transport=None):
        ORIGINAL_INIT(self, base, token, httpx.MockTransport(wrapped))

    monkeypatch.setattr(api.Gateway, "__init__", init)
    return calls


def test_login_registers_or_falls_back_to_login(monkeypatch):
    def handler(req):
        if req.url.path == "/api/auth/register":
            return httpx.Response(409, json={"error": "alias_taken", "message": "Ese alias ya existe: use /auth/login."})
        return httpx.Response(200, json={"token": "T", "player": {"player_id": "p1", "alias": "Ana", "role": "player"}})

    calls = fake_gateway(monkeypatch, handler)
    res = runner.invoke(app, ["login", "Ana"])
    assert res.exit_code == 0 and "Ana" in res.output
    assert [c.url.path for c in calls] == ["/api/auth/register", "/api/auth/login"]
    assert json.loads(api.session_file().read_text())["token"] == "T"


def test_rooms_require_a_session_and_send_the_token(monkeypatch):
    calls = fake_gateway(monkeypatch, lambda req: httpx.Response(200, json={"rooms": [
        {"room_code": "JKR-AB23", "mode": "bot", "difficulty": "normal", "target_score": 22000, "status": "open", "players": [{"alias": "Ana"}]}]}))
    assert runner.invoke(app, ["rooms", "list"]).exit_code == 1
    api.save_session({"token": "T", "player_id": "p1", "alias": "Ana", "role": "player"})
    res = runner.invoke(app, ["rooms", "list"])
    assert res.exit_code == 0 and "JKR-AB23" in res.output
    assert calls[-1].headers["Authorization"] == "Bearer T" and calls[-1].url.host == "localhost"


def test_create_room_body(monkeypatch):
    api.save_session({"token": "T", "player_id": "p1", "alias": "Ana", "role": "player"})
    calls = fake_gateway(monkeypatch, lambda req: httpx.Response(201, json={"room_code": "JKR-XY23", "mode": "bot", "difficulty": "hard", "target_score": 26000}))
    res = runner.invoke(app, ["rooms", "create", "--target", "26000", "--difficulty", "hard"])
    assert res.exit_code == 0 and "JKR-XY23" in res.output
    assert json.loads(calls[-1].content) == {"mode": "bot", "target_score": 26000, "difficulty": "hard"}


def test_leaderboard_is_public_and_errors_are_friendly(monkeypatch):
    fake_gateway(monkeypatch, lambda req: httpx.Response(200, json={"cached": True, "players": [
        {"rank": 1, "player_id": "p1", "alias": "Ana", "wins": 3, "matches": 4, "best_score": 23000}]}))
    res = runner.invoke(app, ["leaderboard"])
    assert res.exit_code == 0 and "Ana" in res.output and "Redis" in res.output
    fake_gateway(monkeypatch, lambda req: httpx.Response(503, json={"error": "service_unavailable", "message": "El servicio no está disponible en este momento."}))
    res = runner.invoke(app, ["leaderboard"])
    assert res.exit_code == 1 and "no está disponible" in res.output


def test_gateway_down(monkeypatch):
    def boom(req):
        raise httpx.ConnectError("refused")
    fake_gateway(monkeypatch, boom)
    res = runner.invoke(app, ["leaderboard"])
    assert res.exit_code == 1 and "No se pudo contactar el gateway" in res.output
