import json
import ssl

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
    monkeypatch.delenv("JOKERARENA_GATEWAY", raising=False)
    monkeypatch.delenv("JOKERARENA_CA", raising=False)


def fake_gateway(monkeypatch, handler):
    """Reemplaza la red por un transporte falso que registra las peticiones."""
    calls = []

    def wrapped(request: httpx.Request):
        calls.append(request)
        return handler(request)

    def init(self, base=None, token=None, transport=None, verify=None):
        ORIGINAL_INIT(self, base, token, httpx.MockTransport(wrapped), verify=verify)

    monkeypatch.setattr(api.Gateway, "__init__", init)
    return calls


def test_login_registers_or_falls_back_to_login(monkeypatch):
    def handler(req):
        if req.url.path == "/api/auth/register":
            return httpx.Response(409, json={"error": "alias_taken", "message": "Ese usuario ya existe: inicia sesión con tu contraseña."})
        return httpx.Response(200, json={"token": "T", "player": {"player_id": "p1", "alias": "Ana", "user": "Ana", "role": "player"}})

    calls = fake_gateway(monkeypatch, handler)
    res = runner.invoke(app, ["login", "Ana", "--password", "secreta123"])
    assert res.exit_code == 0 and "Ana" in res.output
    assert [c.url.path for c in calls] == ["/api/auth/register", "/api/auth/login"]
    assert json.loads(calls[0].content) == {"user": "Ana", "password": "secreta123"}
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


def test_default_gateway_and_ws_url_use_https_and_wss(monkeypatch):
    assert api.gateway_url() == "https://localhost"
    assert api.ws_url("jkr-ab23", "t0k") == "wss://localhost/ws/JKR-AB23?token=t0k"

    monkeypatch.setenv("JOKERARENA_GATEWAY", "http://localhost:8080/")
    assert api.gateway_url() == "http://localhost:8080"
    assert api.ws_url("jkr-ab23", "a b") == "ws://localhost:8080/ws/JKR-AB23?token=a%20b"


def test_ca_bundle_env_and_missing_file(tmp_path, monkeypatch):
    missing = tmp_path / "no-existe.crt"
    monkeypatch.setenv("JOKERARENA_CA", str(missing))
    with pytest.raises(api.CliError, match="gen_certs.sh"):
        api.ca_bundle_path()

    ca_file = tmp_path / "jokerarena-ca.crt"
    ca_file.write_text("dummy", encoding="utf-8")
    monkeypatch.setenv("JOKERARENA_CA", str(ca_file))
    assert api.ca_bundle_path() == ca_file

    # build_ssl_context intenta cargar el PEM en ssl.SSLContext; verificamos el rechazo de PEM inválido
    with pytest.raises(ssl.SSLError):
        api.build_ssl_context(ca_file)


def test_ws_ssl_context_only_enabled_for_wss(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    assert api.ws_ssl_context("ws://localhost:8080/ws/JKR-ABCD?token=T") is None
    ctx = api.ws_ssl_context("wss://localhost/ws/JKR-ABCD?token=T")
    assert isinstance(ctx, ssl.SSLContext)


def test_rooms_create_prints_https_web_url(monkeypatch):
    api.save_session({"token": "T", "player_id": "p1", "alias": "Ana", "role": "player"})
    fake_gateway(monkeypatch, lambda req: httpx.Response(201, json={"room_code": "JKR-XY23", "mode": "bot", "difficulty": "normal", "target_score": 22000}))
    res = runner.invoke(app, ["rooms", "create"])
    assert res.exit_code == 0
    assert "https://localhost/room/JKR-XY23" in res.output
    assert ":3000" not in res.output


def test_tls_verification_error_shows_ca_hint(monkeypatch):
    def tls_fail(req):
        try:
            raise ssl.SSLCertVerificationError(1, "[SSL: CERTIFICATE_VERIFY_FAILED] certificate verify failed")
        except ssl.SSLCertVerificationError as ssl_exc:
            raise httpx.ConnectError("TLS failed") from ssl_exc

    fake_gateway(monkeypatch, tls_fail)
    res = runner.invoke(app, ["leaderboard"])
    assert res.exit_code == 1
    assert "gen_certs.sh" in res.output
    assert "JOKERARENA_CA" in res.output
