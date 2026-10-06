import os


class Settings:
    HTTP_PORT: int = int(os.getenv("HTTP_PORT", os.getenv("TELEMETRY_HTTP_PORT", "8000")))
    GRPC_PORT: int = int(os.getenv("GRPC_PORT", os.getenv("TELEMETRY_GRPC_PORT", "50051")))
    MONGO_URL: str = os.getenv(
        "MONGO_URL",
        os.getenv("TELEMETRY_MONGO_URL", "mongodb://localhost:27017"),
    )
    MONGO_DB: str = os.getenv(
        "MONGO_DB",
        os.getenv("MONGO_INITDB_DATABASE", "jokerarena_telemetry"),
    )
    SERVICE_NAME: str = "jokerarena-telemetry-ms"


settings = Settings()
