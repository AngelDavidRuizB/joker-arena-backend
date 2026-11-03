"""auth-identity-ms: identidad de jugadores, emisión de JWT (RS256) y JWKS."""
import hmac
import logging
import re
import time
import uuid
from contextlib import asynccontextmanager
from typing import Optional

import jwt
from fastapi import Depends, FastAPI, Header, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse, Response
from pydantic import BaseModel

from .config import Settings, settings as default_settings
from .events import Publisher, RabbitPublisher
from .keys import SigningKey
from .store import AliasTaken, PlayerStore, PostgresPlayerStore, RedisSessionStore, SessionStore

log = logging.getLogger("auth")
SERVICE = "auth-identity-ms"
ALIAS_RE = re.compile(r"^[A-Za-z0-9_-]{2,30}$")
RESERVED = {"bot", "bot-jokerbot"}


class ApiError(Exception):
    def __init__(self, status: int, code: str, message: str):
        self.status, self.code, self.message = status, code, message


class AliasBody(BaseModel):
    alias: str
    password: Optional[str] = None


class Services:
    def __init__(self, players: PlayerStore, sessions: SessionStore, publisher: Publisher, key: SigningKey, cfg: Settings):
        self.players, self.sessions, self.publisher, self.key, self.cfg = players, sessions, publisher, key, cfg


def validate_alias(raw: str) -> str:
    alias = (raw or "").strip()
    if not ALIAS_RE.match(alias):
        raise ApiError(400, "invalid_alias", "El alias debe tener 2 a 30 caracteres: letras, números, '_' o '-'.")
    if alias.lower() in RESERVED:
        raise ApiError(400, "reserved_alias", "Ese alias está reservado.")
    return alias


def create_app(services: Optional[Services] = None, cfg: Settings = default_settings) -> FastAPI:
    @asynccontextmanager
    async def lifespan(app: FastAPI):
        if app.state.svc is None:
            publisher = RabbitPublisher(cfg.rabbitmq_url, cfg.exchange)
            if cfg.rabbitmq_url:
                publisher.start()
            app.state.svc = Services(
                PostgresPlayerStore(cfg.postgres_url), RedisSessionStore(cfg.redis_url),
                publisher, SigningKey.load_or_create(cfg.key_file), cfg,
            )
        yield
        svc = app.state.svc
        for closer in (getattr(svc.players, "close", None), getattr(svc.sessions, "close", None), getattr(svc.publisher, "close", None)):
            if closer:
                try:
                    await closer()
                except Exception:  # noqa: BLE001
                    pass

    app = FastAPI(title="auth-identity-ms", version="1.0.0", lifespan=lifespan)
    app.state.svc = services

    @app.exception_handler(ApiError)
    async def api_error(_: Request, exc: ApiError):
        return JSONResponse(status_code=exc.status, content={"error": exc.code, "message": exc.message})

    @app.exception_handler(RequestValidationError)
    async def validation_error(_: Request, exc: RequestValidationError):
        return JSONResponse(status_code=400, content={"error": "invalid_request", "message": "Cuerpo inválido: se espera {\"alias\": \"...\"}."})

    def svc(request: Request) -> Services:
        return request.app.state.svc

    def issue(s: Services, player) -> dict:
        now = int(time.time())
        jti = str(uuid.uuid4())
        claims = {"iss": s.cfg.issuer, "sub": player.player_id, "alias": player.alias, "role": player.role,
                  "iat": now, "exp": now + s.cfg.token_ttl_seconds, "jti": jti}
        token = jwt.encode(claims, s.key.private_pem, algorithm="RS256", headers={"kid": s.key.kid})
        return {"token": token, "token_type": "Bearer", "expires_in": s.cfg.token_ttl_seconds, "jti": jti,
                "player": {"player_id": player.player_id, "alias": player.alias, "role": player.role}}

    async def start_session(s: Services, out: dict) -> None:
        try:
            await s.sessions.start(out["jti"], out["player"]["player_id"], s.cfg.token_ttl_seconds)
        except Exception as exc:  # noqa: BLE001 - Redis es auxiliar: el token sigue siendo válido
            log.warning("no se pudo registrar la sesión en Redis: %s", exc)
        out.pop("jti", None)

    async def store_call(coro):
        try:
            return await coro
        except (ApiError, AliasTaken):
            raise
        except Exception as exc:  # noqa: BLE001
            log.error("PostgreSQL no disponible: %s", exc)
            raise ApiError(503, "store_unavailable", "La base de datos de identidad no está disponible.") from exc

    def check_admin(s: Services, alias: str, password: Optional[str]) -> str:
        if alias.lower() != "admin":
            return "player"
        if not s.cfg.admin_password or not hmac.compare_digest(password or "", s.cfg.admin_password):
            raise ApiError(401, "invalid_credentials", "El usuario admin requiere la contraseña de administración.")
        return "admin"

    @app.post("/auth/register", status_code=201)
    async def register(body: AliasBody, s: Services = Depends(svc)):
        alias = validate_alias(body.alias)
        role = check_admin(s, alias, body.password)
        try:
            player = await store_call(s.players.create(alias, role))
        except AliasTaken:
            raise ApiError(409, "alias_taken", "Ese alias ya existe: use /auth/login.")
        await s.publisher.publish("auth.player.registered", {"player_id": player.player_id, "alias": player.alias})
        out = issue(s, player)
        await start_session(s, out)
        return out

    @app.post("/auth/login")
    async def login(body: AliasBody, s: Services = Depends(svc)):
        alias = validate_alias(body.alias)
        check_admin(s, alias, body.password)
        player = await store_call(s.players.by_alias(alias))
        if player is None:
            raise ApiError(404, "player_not_found", "No existe un jugador con ese alias: use /auth/register.")
        out = issue(s, player)
        await start_session(s, out)
        return out

    async def current_claims(s: Services, authorization: Optional[str]) -> dict:
        token = (authorization or "").removeprefix("Bearer ").strip()
        if not token:
            raise ApiError(401, "unauthorized", "Falta el token (Authorization: Bearer ...).")
        try:
            claims = jwt.decode(token, s.key.private_key.public_key(), algorithms=["RS256"], issuer=s.cfg.issuer)
        except jwt.PyJWTError:
            raise ApiError(401, "unauthorized", "Token inválido o vencido.")
        try:
            if await s.sessions.is_revoked(claims["jti"]):
                raise ApiError(401, "unauthorized", "La sesión fue cerrada.")
        except ApiError:
            raise
        except Exception:  # noqa: BLE001 - sin Redis no se puede comprobar la revocación
            pass
        return claims

    @app.get("/auth/me")
    async def me(authorization: Optional[str] = Header(default=None), s: Services = Depends(svc)):
        claims = await current_claims(s, authorization)
        return {"player_id": claims["sub"], "alias": claims["alias"], "role": claims["role"], "expires_at": claims["exp"]}

    @app.post("/auth/logout", status_code=204)
    async def logout(authorization: Optional[str] = Header(default=None), s: Services = Depends(svc)):
        claims = await current_claims(s, authorization)
        try:
            await s.sessions.revoke(claims["jti"], max(claims["exp"] - int(time.time()), 1))
        except Exception as exc:  # noqa: BLE001
            raise ApiError(503, "session_store_unavailable", "No se pudo cerrar la sesión.") from exc

    @app.get("/auth/verify", status_code=204)
    async def verify(request: Request, s: Services = Depends(svc)):
        """Subpetición del gateway (nginx auth_request) antes de enrutar una ruta protegida.

        Acepta el token en Authorization o, para WebSockets, en ?token= de la URI original
        (cabecera X-Original-URI). Con X-Required-Role: admin exige ese rol. Devuelve la
        identidad en cabeceras para que el gateway la reenvíe al servicio.
        """
        from urllib.parse import parse_qs, urlsplit
        auth_header = request.headers.get("authorization")
        if not auth_header:
            query = parse_qs(urlsplit(request.headers.get("x-original-uri", "")).query)
            if query.get("token"):
                auth_header = f"Bearer {query['token'][0]}"
        claims = await current_claims(s, auth_header)
        required = request.headers.get("x-required-role")
        if required and claims.get("role") != required:
            raise ApiError(403, "forbidden", f"Se requiere el rol {required}.")
        return Response(status_code=204, headers={
            "X-Player-Id": claims["sub"], "X-Player-Alias": claims.get("alias", ""), "X-Player-Role": claims.get("role", ""),
        })

    @app.get("/.well-known/jwks.json")
    async def jwks(s: Services = Depends(svc)):
        return s.key.jwks()

    @app.get("/health")
    async def health(s: Services = Depends(svc)):
        deps = {}
        for name, check in (("postgres", s.players.ping), ("redis", s.sessions.ping)):
            try:
                await check()
                deps[name] = "ok"
            except Exception:  # noqa: BLE001
                deps[name] = "down"
        deps["rabbitmq"] = "ok" if s.publisher.connected else "down"
        status = "ok" if all(v == "ok" for v in deps.values()) else "degraded"
        return {"status": status, "service": SERVICE, "deps": deps}

    return app


app = create_app()
