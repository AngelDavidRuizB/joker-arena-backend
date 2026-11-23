-- ==============================================================================
-- auth-identity-ms · esquema "auth" (dueño: svc_auth)
-- ==============================================================================
SET ROLE svc_auth;

CREATE TABLE IF NOT EXISTS auth.players (
    player_id     UUID PRIMARY KEY,
    alias         VARCHAR(30) NOT NULL,
    role          VARCHAR(10) NOT NULL DEFAULT 'player' CHECK (role IN ('player', 'admin')),
    -- Hash PBKDF2-SHA256 de la contraseña ("pbkdf2_sha256$iteraciones$sal$hash").
    -- NULL solo en cuentas creadas antes del registro con contraseña (se migran al
    -- volver a registrarse con el mismo usuario).
    password_hash TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- El alias es único sin importar mayúsculas/minúsculas.
CREATE UNIQUE INDEX IF NOT EXISTS ux_auth_players_alias ON auth.players (lower(alias));

-- Migración idempotente para volúmenes creados antes de existir las contraseñas
-- (el servicio también la aplica al arrancar, ver PostgresPlayerStore.ensure_schema).
ALTER TABLE auth.players ADD COLUMN IF NOT EXISTS password_hash TEXT;

RESET ROLE;

