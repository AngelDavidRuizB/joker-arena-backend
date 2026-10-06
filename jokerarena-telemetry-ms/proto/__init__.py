import sys
from pathlib import Path

# Ensure proto directory is in sys.path so generated stubs resolve imports cleanly
_proto_dir = str(Path(__file__).resolve().parent)
if _proto_dir not in sys.path:
    sys.path.insert(0, _proto_dir)

try:
    from . import telemetry_pb2, telemetry_pb2_grpc
except ImportError:
    import telemetry_pb2
    import telemetry_pb2_grpc

__all__ = ["telemetry_pb2", "telemetry_pb2_grpc"]
