import logging
from typing import Optional

import grpc

try:
    from proto import telemetry_pb2_grpc
except ImportError:
    from app.grpc_server import telemetry_pb2_grpc

from app.grpc_server.service import TelemetryServiceServicer

logger = logging.getLogger("telemetry.grpc")


async def create_grpc_server(
    servicer: Optional[TelemetryServiceServicer] = None,
    host: str = "0.0.0.0",
    port: int = 50051,
) -> grpc.aio.Server:
    """Instantiates and registers the async gRPC server on the event loop."""
    server = grpc.aio.server()
    if servicer is None:
        servicer = TelemetryServiceServicer()
    telemetry_pb2_grpc.add_TelemetryServiceServicer_to_server(servicer, server)
    listen_addr = f"{host}:{port}"
    server.add_insecure_port(listen_addr)
    logger.info(f"Configured gRPC server on {listen_addr}")
    return server
