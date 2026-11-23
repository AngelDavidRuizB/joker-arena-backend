"""Hash y política de contraseñas (biblioteca estándar: sin dependencias nuevas).

Formato almacenado: ``pbkdf2_sha256$<iteraciones>$<salt-b64>$<hash-b64>``.
Cada contraseña usa una sal aleatoria de 16 bytes; la comparación es en tiempo constante.
"""
import base64
import hashlib
import hmac
import secrets

ALGORITHM = "pbkdf2_sha256"
ITERATIONS = 240_000
SALT_BYTES = 16
MIN_LENGTH = 8
MAX_LENGTH = 72


def _b64(raw: bytes) -> str:
    return base64.b64encode(raw).decode("ascii")


def _unb64(text: str) -> bytes:
    return base64.b64decode(text.encode("ascii"), validate=True)


def hash_password(password: str) -> str:
    """Devuelve el hash con sal para guardar en ``auth.players.password_hash``."""
    salt = secrets.token_bytes(SALT_BYTES)
    digest = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, ITERATIONS)
    return f"{ALGORITHM}${ITERATIONS}${_b64(salt)}${_b64(digest)}"


def verify_password(password: str, stored: str | None) -> bool:
    """True si ``password`` corresponde al hash ``stored``. Nunca lanza."""
    if not password or not stored:
        return False
    try:
        algorithm, iterations, salt_b64, digest_b64 = stored.split("$", 3)
        if algorithm != ALGORITHM:
            return False
        rounds = int(iterations)
        salt, expected = _unb64(salt_b64), _unb64(digest_b64)
    except (ValueError, TypeError):
        return False
    digest = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, rounds)
    return hmac.compare_digest(digest, expected)


def password_policy_error(password: str) -> str | None:
    """Mensaje de error en español, o ``None`` si la contraseña cumple la política."""
    if not password or not password.strip():
        return "Escribe una contraseña."
    if len(password) < MIN_LENGTH:
        return f"La contraseña necesita al menos {MIN_LENGTH} caracteres."
    if len(password) > MAX_LENGTH:
        return f"La contraseña admite máximo {MAX_LENGTH} caracteres."
    return None
