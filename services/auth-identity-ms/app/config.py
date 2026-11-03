"""Configuración por variables de entorno (sección 0.4 de la guía)."""
import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    port: int = int(os.getenv("PORT", "8082"))
    postgres_url: str = os.getenv("POSTGRES_URL", "")
    redis_url: str = os.getenv("REDIS_URL", "redis://jokerarena-redis:6379/0")
    rabbitmq_url: str = os.getenv("RABBITMQ_URL", "")
    exchange: str = os.getenv("RABBITMQ_EXCHANGE", "jokerarena.events")
    key_file: str = os.getenv("JWT_KEY_FILE", "/data/jwt_private_key.pem")
    issuer: str = os.getenv("JWT_ISSUER", "jokerarena-auth")
    token_ttl_seconds: int = int(os.getenv("JWT_TTL_SECONDS", str(12 * 3600)))
    admin_password: str = os.getenv("ADMIN_PASSWORD", "")


settings = Settings()
