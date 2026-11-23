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
    # --- Bloqueo por intentos fallidos (login y registro) -----------------------
    # Intentos fallidos permitidos antes de bloquear la cuenta.
    max_failed_attempts: int = int(os.getenv("AUTH_MAX_FAILED_ATTEMPTS", "5"))
    # Duración del bloqueo en MINUTOS (aquí se cambia el tiempo).
    lockout_minutes: int = int(os.getenv("AUTH_LOCKOUT_MINUTES", "15"))
    # Vigencia del token del rol admin, en segundos (30 min por defecto).
    admin_token_ttl_seconds: int = int(os.getenv("AUTH_ADMIN_TOKEN_TTL_SECONDS", str(30 * 60)))

    @property
    def lockout_seconds(self) -> int:
        return max(self.lockout_minutes, 1) * 60


settings = Settings()

