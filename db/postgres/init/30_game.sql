-- ==============================================================================
-- game-engine-ms · esquema "game" (dueño: svc_game)
-- Jugadores y salas viven en auth y lobby: aquí solo se guardan sus identificadores.
-- ==============================================================================
SET ROLE svc_game;

CREATE TABLE IF NOT EXISTS game.matches (
    match_id      UUID PRIMARY KEY,
    room_code     VARCHAR(16)  NOT NULL,
    host_id       VARCHAR(64)  NOT NULL,
    host_alias    VARCHAR(64)  NOT NULL DEFAULT '',
    with_bot      BOOLEAN      NOT NULL DEFAULT TRUE,
    difficulty    VARCHAR(16)  NOT NULL DEFAULT 'normal',
    target_score  INTEGER      NOT NULL CHECK (target_score > 0),
    status        VARCHAR(16)  NOT NULL DEFAULT 'WAITING'
                  CHECK (status IN ('WAITING', 'IN_PROGRESS', 'FINISHED')),
    host_score    INTEGER,
    bot_score     INTEGER,
    hands_played  INTEGER,
    host_won      BOOLEAN,
    end_reason    VARCHAR(32),
    progress      JSONB,       -- estado reanudable tras reiniciar el motor (room.MatchProgress)
    progress_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    ended_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_game_matches_room ON game.matches (room_code, created_at DESC);

RESET ROLE;
