"""Cliente HTTP del gateway. La CLI nunca habla con un microservicio directamente."""
import json
import os
import ssl
from pathlib import Path
from typing import Any, Optional, Union
from urllib.parse import quote

import httpx

DEFAULT_GATEWAY = "https://localhost"


class CliError(Exception):
    """Error mostrado al usuario tal cual (en español). ``code`` trae el código del gateway."""

    def __init__(self, message: str, code: str = ""):
        super().__init__(message)
        self.code = code


def gateway_url() -> str:
    return os.getenv("JOKERARENA_GATEWAY", DEFAULT_GATEWAY).rstrip("/")


def session_file() -> Path:
    return Path(os.getenv("JOKERARENA_HOME", Path.home() / ".jokerarena")) / "session.json"


def ca_bundle_path() -> Optional[Path]:
    """Resuelve el certificado de la CA local para verificar HTTPS/WSS.

    Si JOKERARENA_CA está definida, exige que exista en esa ruta. Si no, busca
    edge/certs/jokerarena-ca.crt en el directorio actual, en la raíz del repo o en
    JOKERARENA_HOME. Si no existe ninguno, usa el almacén de confianza del sistema.
    """
    env_ca = os.getenv("JOKERARENA_CA", "").strip()
    if env_ca:
        p = Path(env_ca).expanduser()
        if not p.is_file():
            raise CliError(f"No se encontró el certificado CA en {p}. Ejecuta: bash scripts/gen_certs.sh")
        return p

    home_dir = Path(os.getenv("JOKERARENA_HOME", Path.home() / ".jokerarena"))
    candidates = [
        Path.cwd() / "edge" / "certs" / "jokerarena-ca.crt",
        home_dir / "jokerarena-ca.crt",
    ]
    resolved = Path(__file__).resolve()
    if len(resolved.parents) > 3:
        candidates.insert(1, resolved.parents[3] / "edge" / "certs" / "jokerarena-ca.crt")

    for candidate in candidates:
        if candidate.is_file():
            return candidate
    return None


def build_ssl_context(ca_path: Optional[Path] = None) -> Union[ssl.SSLContext, bool]:
    """Construye el contexto SSL con la CA local o mantiene la verificación estricta del sistema."""
    ca = ca_path if ca_path is not None else ca_bundle_path()
    if ca is not None:
        return ssl.create_default_context(cafile=str(ca))
    return True


def ws_url(room_code: str, token: str, base: Optional[str] = None) -> str:
    """Construye la URL WebSocket (wss:// para https://, ws:// para http://)."""
    raw = (base or gateway_url()).rstrip("/")
    if raw.startswith("https://"):
        ws_base = "wss://" + raw[len("https://"):]
    elif raw.startswith("http://"):
        ws_base = "ws://" + raw[len("http://"):]
    else:
        ws_base = raw
    return f"{ws_base}/ws/{quote(room_code.upper(), safe='')}?token={quote(token, safe='')}"


def ws_ssl_context(url: str, ca_path: Optional[Path] = None) -> Optional[ssl.SSLContext]:
    """Devuelve SSLContext para wss:// y None para ws:// (websockets rechaza ssl en ws://)."""
    if not url.startswith("wss://"):
        return None
    ctx = build_ssl_context(ca_path)
    return ctx if isinstance(ctx, ssl.SSLContext) else ssl.create_default_context()


def _is_tls_error(exc: BaseException) -> bool:
    seen = set()
    cur: Optional[BaseException] = exc
    while cur is not None and id(cur) not in seen:
        seen.add(id(cur))
        if isinstance(cur, ssl.SSLError):
            return True
        if "CERTIFICATE_VERIFY_FAILED" in str(cur):
            return True
        cur = cur.__cause__ or cur.__context__
    return False


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
    def __init__(
        self,
        base: Optional[str] = None,
        token: Optional[str] = None,
        transport: Optional[httpx.BaseTransport] = None,
        verify: Optional[Union[bool, str, ssl.SSLContext]] = None,
    ):
        self.base = (base or gateway_url()).rstrip("/")
        self.token = token
        if verify is not None:
            ssl_verify: Union[bool, str, ssl.SSLContext] = verify
        elif transport is not None:
            ssl_verify = True
        else:
            ssl_verify = build_ssl_context()
        self.client = httpx.Client(base_url=self.base, timeout=5.0, transport=transport, verify=ssl_verify)

    def _call(self, method: str, path: str, auth: bool = False, **kw) -> Any:
        headers = {}
        if auth:
            if not self.token:
                raise CliError("No hay sesión. Ejecuta primero: jokerarena login <alias>")
            headers["Authorization"] = f"Bearer {self.token}"
        try:
            r = self.client.request(method, path, headers=headers, **kw)
        except httpx.HTTPError as exc:
            if _is_tls_error(exc):
                raise CliError(
                    f"No se pudo verificar el certificado TLS de {self.base}. "
                    "Genera la CA con 'bash scripts/gen_certs.sh' o indica su ruta en JOKERARENA_CA."
                ) from exc
            raise CliError(f"No se pudo contactar el gateway en {self.base} ({exc.__class__.__name__}).") from exc
        try:
            body = r.json()
        except ValueError:
            body = {}
        if r.status_code >= 400:
            message = body.get("message") if isinstance(body, dict) else None
            code = body.get("error") if isinstance(body, dict) else None
            raise CliError(message or f"Error {r.status_code}", code or "")
        return body

    def enter(self, user: str, password: str) -> dict:
        """Registra la cuenta (usuario + contraseña) o inicia sesión si ya existe."""
        payload = {"user": user, "password": password}
        try:
            return self._call("POST", "/api/auth/register", json=payload)
        except CliError as exc:
            if exc.code != "alias_taken":
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
