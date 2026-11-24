"""Pruebas del hash y la política de contraseñas."""
from app.passwords import hash_password, password_policy_error, verify_password


def test_hash_is_salted_and_verifiable():
    a = hash_password("secreta123")
    b = hash_password("secreta123")
    assert a != b, "dos hashes de la misma contraseña deben diferir (sal aleatoria)"
    assert a.startswith("pbkdf2_sha256$") and verify_password("secreta123", a)
    assert verify_password("secreta124", a) is False


def test_verify_never_raises_with_bad_input():
    for stored in (None, "", "basura", "pbkdf2_sha256$x$y$z", "md5$1$2$3"):
        assert verify_password("secreta123", stored) is False
    assert verify_password("", hash_password("secreta123")) is False


def test_password_policy():
    assert password_policy_error("secreta123") is None
    assert "escribe" in password_policy_error("").lower()
    assert "al menos 8" in password_policy_error("corta")
    assert "máximo 72" in password_policy_error("x" * 73)
