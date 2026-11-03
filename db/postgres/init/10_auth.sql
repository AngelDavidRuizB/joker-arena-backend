-- ==============================================================================
-- auth-identity-ms · esquema "auth" (dueño: svc_auth)
-- ==============================================================================
SET ROLE svc_auth;

CREATE TABLE IF NOT EXISTS auth.players (
    player_id   UUID PRIMARY KEY,
    alias       VARCHAR(30) NOT NULL,
    role        VARCHAR(10) NOT NULL DEFAULT 'player' CHECK (role IN ('player', 'admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- El alias es único sin importar mayúsculas/minúsculas.
CREATE UNIQUE INDEX IF NOT EXISTS ux_auth_players_alias ON auth.players (lower(alias));

RESET ROLE;
