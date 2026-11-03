"""telemetry-ms: auditoría mano a mano y estadísticas, alimentadas por eventos de RabbitMQ."""
import logging
import os
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI, Query, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from .consumer import Consumer
from .store import MongoStore

logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s %(levelname)s %(name)s: %(message)s")
log = logging.getLogger("telemetry")
SERVICE = "telemetry-ms"


def create_app(store=None, consumer=None) -> FastAPI:
    @asynccontextmanager
    async def lifespan(app: FastAPI):
        if app.state.store is None:
            app.state.store = MongoStore(os.getenv("MONGO_URL", "mongodb://jokerarena-mongo:27017"))
            try:
                await app.state.store.ensure_indexes()
            except Exception as exc:  # noqa: BLE001 - se crearán cuando Mongo responda (init.js)
                log.warning("no se pudieron crear los índices: %s", exc)
        if app.state.consumer is None and os.getenv("RABBITMQ_URL"):
            app.state.consumer = Consumer(os.environ["RABBITMQ_URL"], os.getenv("RABBITMQ_EXCHANGE", "jokerarena.events"), app.state.store)
            app.state.consumer.start()
        yield
        if app.state.consumer is not None and hasattr(app.state.consumer, "close"):
            await app.state.consumer.close()

    app = FastAPI(title="telemetry-ms", version="2.0.0", lifespan=lifespan)
    app.state.store, app.state.consumer = store, consumer

    @app.exception_handler(RequestValidationError)
    async def invalid(_: Request, exc: RequestValidationError):
        return JSONResponse(status_code=400, content={"error": "invalid_request", "message": str(exc.errors()[0].get("msg", "parámetro inválido"))})

    @app.exception_handler(Exception)
    async def unavailable(_: Request, exc: Exception):
        log.warning("MongoDB no disponible: %s", exc)
        return JSONResponse(status_code=503, content={"error": "store_unavailable", "message": "La base de telemetría no está disponible."})

    @app.get("/hands")
    async def hands(request: Request, match_id: Optional[str] = None, player_id: Optional[str] = None,
                    limit: int = Query(50, ge=1, le=500)):
        return {"hands": await request.app.state.store.list_hands(match_id, player_id, limit)}

    @app.get("/matches")
    async def matches(request: Request, limit: int = Query(20, ge=1, le=200)):
        return {"matches": await request.app.state.store.list_matches(limit)}

    @app.get("/matches/{match_id}")
    async def match(request: Request, match_id: str):
        store = request.app.state.store
        summary = await store.get_match(match_id)
        hands = await store.list_hands(match_id, None, 500)
        if summary is None and not hands:
            return JSONResponse(status_code=404, content={"error": "match_not_found", "message": f"No hay telemetría de la partida {match_id}."})
        return {"match": summary, "hands": hands}

    @app.get("/stats")
    async def stats(request: Request):
        return await request.app.state.store.stats()

    @app.get("/health")
    async def health(request: Request):
        deps = {}
        try:
            await request.app.state.store.ping()
            deps["mongo"] = "ok"
        except Exception:  # noqa: BLE001
            deps["mongo"] = "down"
        c = request.app.state.consumer
        deps["rabbitmq"] = "ok" if c is not None and c.connected else "down"
        return {"status": "ok" if all(v == "ok" for v in deps.values()) else "degraded", "service": SERVICE, "deps": deps}

    return app


app = create_app()
