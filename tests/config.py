"""
JokerArena E2E Test Suite - Configuration Module
Loads configuration from .env and environment variables.
"""
import os
from pathlib import Path


def _load_env_file(project_root: Path) -> dict:
    """Lightweight .env parser that doesn't strictly require python-dotenv."""
    env_vars = {}
    env_file = project_root / ".env"
    if env_file.exists():
        with open(env_file, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                if "=" in line:
                    key, val = line.split("=", 1)
                    env_vars[key.strip()] = val.strip()
    return env_vars


PROJECT_ROOT = Path(__file__).resolve().parent.parent
_ENV = _load_env_file(PROJECT_ROOT)


def get_conf(key: str, default: str) -> str:
    return os.environ.get(key, _ENV.get(key, default))


# Host Configuration
TARGET_HOST = get_conf("E2E_TARGET_HOST", "localhost")

# Ports
FRONTEND_PORT = int(get_conf("FRONTEND_PORT", "3000"))
GAME_ENGINE_PORT = int(get_conf("GAME_ENGINE_PORT", "8080"))
TELEMETRY_HTTP_PORT = int(get_conf("TELEMETRY_HTTP_PORT", "8000"))
TELEMETRY_GRPC_PORT = int(get_conf("TELEMETRY_GRPC_PORT", "50051"))
POSTGRES_PORT = int(get_conf("POSTGRES_PORT", "5432"))
MONGO_PORT = int(get_conf("MONGO_PORT", "27017"))

# Service URLs
FRONTEND_URL = f"http://{TARGET_HOST}:{FRONTEND_PORT}"
GAME_ENGINE_URL = f"http://{TARGET_HOST}:{GAME_ENGINE_PORT}"
GAME_ENGINE_WS_URL = f"ws://{TARGET_HOST}:{GAME_ENGINE_PORT}"
TELEMETRY_HTTP_URL = f"http://{TARGET_HOST}:{TELEMETRY_HTTP_PORT}"
TELEMETRY_GRPC_ADDR = f"{TARGET_HOST}:{TELEMETRY_GRPC_PORT}"

# Database Settings (credentials come from .env, never hard-coded)
POSTGRES_USER = get_conf("POSTGRES_USER", "appuser")
POSTGRES_PASSWORD = get_conf("POSTGRES_PASSWORD", "")
POSTGRES_DB = get_conf("POSTGRES_DB", "appdb")

MONGO_DB = get_conf("MONGO_DB", "casino_games")
MONGO_ROOT_USER = get_conf("MONGO_ROOT_USER", "admin")
MONGO_ROOT_PASSWORD = get_conf("MONGO_ROOT_PASSWORD", "")

# Test Run Mode
def is_mock_mode() -> bool:
    return os.environ.get("E2E_MOCK_MODE", "false").lower() in ("1", "true", "yes")

TIMEOUT_SECONDS = float(get_conf("E2E_TIMEOUT_SECONDS", "3.0"))
