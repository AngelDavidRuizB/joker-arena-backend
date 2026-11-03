"""Cliente HTTP del gateway. La CLI nunca habla con un microservicio directamente."""
import json
import os
from pathlib import Path
from typing import Any, Optional

import httpx

DEFAULT_GATEWAY = "http://localhost:8080"


class CliError(Exception):
    """Error mostrado al usuario tal cual (en español)."""


def gateway_url() -> str:
    return os.getenv("JOKERARENA_GATEWAY", DEFAULT_GATEWAY).rstrip("/")


def session_file() -> Path:
    return Path(os.getenv("JOKERARENA_HOME", Path.home() / ".jokerarena")) / "session.json"


def load_session() -> Optional[dict]:
    try:
        return json.loads(session_file().read_text())
    except (OSError, ValueError):
        return None


def save_session(data: dict) -> None:
    f = session_file()
    f.parent.mkdir(parents=True, exist_ok=True)
    f.write_text(json.dumps(data))
    try:
        f.chmod(0o600)
    except OSError:
        pass


def clear_session() -> None:
    try:
        session_file().unlink()
    except OSError:
        pass


class Gateway:
    def __init__(self, base: Optional[str] = None, token: Optional[str] = None, transport: Optional[httpx.BaseTransport] = None):
        self.base = (base or gateway_url()).rstrip("/")
        self.token = token
        self.client = httpx.Client(base_url=self.base, timeout=5.0, transport=transport)

    def _call(self, method: str, path: str, auth: bool = False, **kw) -> Any:
        headers = {}
        if auth:
            if not self.token:
                raise CliError("No hay sesión. Ejecuta primero: jokerarena login <alias>")
            headers["Authorization"] = f"Bearer {self.token}"
        try:
            r = self.client.request(method, path, headers=headers, **kw)
        except httpx.HTTPError as exc:
            raise CliError(f"No se pudo contactar el gateway en {self.base} ({exc.__class__.__name__}).") from exc
        try:
            body = r.json()
        except ValueError:
            body = {}
        if r.status_code >= 400:
            message = body.get("message") if isinstance(body, dict) else None
            raise CliError(message or f"Error {r.status_code}")
        return body

    def enter(self, alias: str, password: Optional[str] = None) -> dict:
        payload = {"alias": alias, **({"password": password} if password else {})}
        try:
            return self._call("POST", "/api/auth/register", json=payload)
        except CliError as exc:
            if "ya existe" not in str(exc):
                raise
            return self._call("POST", "/api/auth/login", json=payload)

    def rooms(self, status: str = "open") -> list:
        return self._call("GET", "/api/rooms", auth=True, params={"status": status})["rooms"]

    def create_room(self, bot: bool, target: int, difficulty: str) -> dict:
        return self._call("POST", "/api/rooms", auth=True, json={"mode": "bot" if bot else "pvp", "target_score": target, "difficulty": difficulty})

    def leaderboard(self, limit: int = 10) -> dict:
        return self._call("GET", "/api/leaderboard/top", params={"limit": limit})

    def player(self, player_id: str) -> dict:
        return self._call("GET", f"/api/leaderboard/players/{player_id}")

    def match(self, match_id: str) -> dict:
        return self._call("GET", f"/api/telemetry/matches/{match_id}", auth=True)

    def matches(self, limit: int = 10) -> list:
        return self._call("GET", "/api/telemetry/matches", auth=True, params={"limit": limit})["matches"]

    def ruleset(self) -> dict:
        return self._call("GET", "/api/game/ruleset")
