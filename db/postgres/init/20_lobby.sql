-- ==============================================================================
-- rooms-lobby-ms · esquema "lobby" (dueño: svc_lobby)
-- ==============================================================================
SET ROLE svc_lobby;

CREATE TABLE IF NOT EXISTS lobby.rooms (
    room_code     VARCHAR(8)  PRIMARY KEY CHECK (room_code ~ '^JKR-[A-Z2-9]{4}$'),
    mode          VARCHAR(8)  NOT NULL CHECK (mode IN ('bot', 'pvp')),
    difficulty    VARCHAR(8)  NOT NULL DEFAULT 'normal' CHECK (difficulty IN ('easy', 'normal', 'hard')),
    target_score  INTEGER     NOT NULL CHECK (target_score BETWEEN 1 AND 100000),
    status        VARCHAR(12) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'closed')),
    created_by    VARCHAR(64) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_lobby_rooms_status ON lobby.rooms (status, created_at DESC);

CREATE TABLE IF NOT EXISTS lobby.room_players (
    room_code  VARCHAR(8)  NOT NULL REFERENCES lobby.rooms (room_code) ON DELETE CASCADE,
    player_id  VARCHAR(64) NOT NULL,
    alias      VARCHAR(64) NOT NULL DEFAULT '',
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_code, player_id)
);

RESET ROLE;
