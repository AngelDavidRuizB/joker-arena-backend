import sys
from pathlib import Path

# Ensure proto directory is accessible for generated stubs
_proto_dir = str(Path(__file__).resolve().parent.parent.parent / "proto")
if _proto_dir not in sys.path:
    sys.path.insert(0, _proto_dir)

try:
    from proto import telemetry_pb2, telemetry_pb2_grpc
except ImportError:
    import telemetry_pb2
    import telemetry_pb2_grpc

__all__ = ["telemetry_pb2", "telemetry_pb2_grpc"]
