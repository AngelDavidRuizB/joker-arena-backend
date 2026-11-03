"""Llave RSA para firmar los JWT (RS256) y su publicación como JWKS.

La llave privada se guarda en un volumen para que los tokens sigan siendo válidos si el
contenedor se reinicia. Si el archivo no existe, se genera una nueva.
"""
import base64
import hashlib
import os

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa


def _b64(n: int) -> str:
    raw = n.to_bytes((n.bit_length() + 7) // 8, "big")
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode()


class SigningKey:
    def __init__(self, private_key: rsa.RSAPrivateKey):
        self.private_key = private_key
        pub = private_key.public_key().public_numbers()
        self.kid = hashlib.sha256(f"{pub.n}:{pub.e}".encode()).hexdigest()[:16]
        self.private_pem = private_key.private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        )

    @classmethod
    def generate(cls) -> "SigningKey":
        return cls(rsa.generate_private_key(public_exponent=65537, key_size=2048))

    @classmethod
    def load_or_create(cls, path: str) -> "SigningKey":
        if path and os.path.exists(path):
            with open(path, "rb") as fh:
                return cls(serialization.load_pem_private_key(fh.read(), password=None))
        key = cls.generate()
        if path:
            os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
            with open(path, "wb") as fh:
                fh.write(key.private_pem)
            os.chmod(path, 0o600)
        return key

    def jwks(self) -> dict:
        pub = self.private_key.public_key().public_numbers()
        return {"keys": [{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": self.kid, "n": _b64(pub.n), "e": _b64(pub.e)}]}
