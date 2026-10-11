import os
from urllib.parse import quote_plus


def _build_mongo_url() -> str:
    """Builds the MongoDB URL.

    Priority: explicit MONGO_URL / TELEMETRY_MONGO_URL, otherwise composed from
    MONGO_HOST / MONGO_PORT / MONGO_USER / MONGO_PASSWORD / MONGO_AUTH_SOURCE.
    User and password are percent-encoded so characters such as '@' are safe.
    """
    explicit = os.getenv("MONGO_URL") or os.getenv("TELEMETRY_MONGO_URL")
    if explicit:
        return explicit

    host = os.getenv("MONGO_HOST", "localhost")
    port = os.getenv("MONGO_PORT", "27017")
    user = os.getenv("MONGO_USER", "")
    password = os.getenv("MONGO_PASSWORD", "")
    auth_source = os.getenv("MONGO_AUTH_SOURCE", "admin")

    if user:
        creds = f"{quote_plus(user)}:{quote_plus(password)}@"
        return f"mongodb://{creds}{host}:{port}/?authSource={quote_plus(auth_source)}"
    return f"mongodb://{host}:{port}"


class Settings:
    HTTP_PORT: int = int(os.getenv("HTTP_PORT", os.getenv("TELEMETRY_HTTP_PORT", "8000")))
    GRPC_PORT: int = int(os.getenv("GRPC_PORT", os.getenv("TELEMETRY_GRPC_PORT", "50051")))
    MONGO_URL: str = _build_mongo_url()
    MONGO_DB: str = os.getenv("MONGO_DB", os.getenv("MONGO_INITDB_DATABASE", "casino_games"))
    MONGO_COLLECTION: str = os.getenv("MONGO_COLLECTION", "hands")
    SERVICE_NAME: str = "jokerarena-telemetry-ms"


settings = Settings()
