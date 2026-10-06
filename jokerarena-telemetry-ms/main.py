import asyncio
import logging
import os
import signal
import sys
from typing import Optional

import uvicorn

from app.config import settings
from app.db.mongo import mongo_manager
from app.grpc_server.server import create_grpc_server
from app.grpc_server.service import TelemetryServiceServicer
from app.main import app, create_app

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("telemetry.main")


async def run_fastapi(app_instance, host: str, port: int, stop_event: asyncio.Event):
    """Runs Uvicorn server cooperatively on the active asyncio event loop."""
    config = uvicorn.Config(
        app=app_instance,
        host=host,
        port=port,
        log_level="info",
        loop="asyncio",
    )
    server = uvicorn.Server(config)

    async def watch_stop():
        await stop_event.wait()
        server.should_exit = True

    watcher = asyncio.create_task(watch_stop())
    try:
        logger.info(f"Starting FastAPI HTTP server on http://{host}:{port}")
        await server.serve()
    finally:
        watcher.cancel()


async def run_grpc(servicer: TelemetryServiceServicer, port: int, stop_event: asyncio.Event):
    """Runs async gRPC server on the same event loop."""
    server = await create_grpc_server(servicer=servicer, host="0.0.0.0", port=port)
    await server.start()
    logger.info(f"gRPC server running on port {port}")

    await stop_event.wait()
    logger.info("Stopping gRPC server with 5s grace period...")
    await server.stop(grace=5.0)
    logger.info("gRPC server stopped.")


async def main():
    http_port = settings.HTTP_PORT
    grpc_port = settings.GRPC_PORT
    stop_event = asyncio.Event()

    def handle_signal():
        logger.info("Termination signal received. Initiating graceful shutdown...")
        stop_event.set()

    if sys.platform != "win32":
        loop = asyncio.get_running_loop()
        for sig in (signal.SIGTERM, signal.SIGINT):
            try:
                loop.add_signal_handler(sig, handle_signal)
            except NotImplementedError:
                pass
    else:
        signal.signal(signal.SIGINT, lambda s, f: handle_signal())
        signal.signal(signal.SIGTERM, lambda s, f: handle_signal())

    await mongo_manager.connect()
    servicer = TelemetryServiceServicer(manager=mongo_manager)

    fastapi_task = asyncio.create_task(run_fastapi(app, "0.0.0.0", http_port, stop_event))
    grpc_task = asyncio.create_task(run_grpc(servicer, grpc_port, stop_event))

    done, pending = await asyncio.wait(
        [fastapi_task, grpc_task, asyncio.create_task(stop_event.wait())],
        return_when=asyncio.FIRST_COMPLETED,
    )

    stop_event.set()
    await asyncio.gather(fastapi_task, grpc_task, return_exceptions=True)
    await mongo_manager.close()
    logger.info("Dual server terminated cleanly.")


if __name__ == "__main__":
    asyncio.run(main())
