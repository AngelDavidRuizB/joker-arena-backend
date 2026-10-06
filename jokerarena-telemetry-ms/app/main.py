from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.db.mongo import mongo_manager
from app.routes.health import router as health_router
from app.routes.telemetry import router as telemetry_router


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup: Connect to MongoDB and ensure indexes
    await mongo_manager.connect()
    yield
    # Shutdown: Close database connection pool
    await mongo_manager.close()


def create_app() -> FastAPI:
    app = FastAPI(
        title="JokerArena Telemetry & Analytics Service",
        description="Immutable audit ledger and analytics for JokerArena poker events",
        version="1.0.0",
        docs_url="/docs",
        redoc_url="/redoc",
        openapi_url="/openapi.json",
        lifespan=lifespan,
    )

    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    app.include_router(health_router)
    app.include_router(telemetry_router)

    return app


app = create_app()
