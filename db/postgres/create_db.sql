-- ==============================================================================
-- JokerArena: Multiverse Poker Engine
-- Esquema relacional oficial del equipo (PostgreSQL 16) - BD: appdb
-- Reconstruido a partir del diagrama ER: players 1--n matches n--1 rooms.
-- Es idempotente: puede ejecutarse varias veces sin error.
-- ==============================================================================

CREATE TABLE IF NOT EXISTS players (
    id_player  SERIAL PRIMARY KEY,
    username   VARCHAR(255),
    email      VARCHAR(255),
    password   VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rooms (
    id_room     SERIAL PRIMARY KEY,
    name        VARCHAR(255),
    description TEXT,
    capacity    INTEGER,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS matches (
    id_match   SERIAL PRIMARY KEY,
    id_player  INTEGER,
    id_room    INTEGER,
    started_at TIMESTAMP,
    ended_at   TIMESTAMP,
    result     VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_matches_id_player_players FOREIGN KEY (id_player) REFERENCES players (id_player),
    CONSTRAINT fk_matches_id_room_rooms     FOREIGN KEY (id_room)   REFERENCES rooms (id_room)
);

-- Índices de apoyo (no alteran el modelo lógico; aceleran las consultas del Game Engine)
CREATE INDEX IF NOT EXISTS idx_players_username ON players (username);
CREATE INDEX IF NOT EXISTS idx_rooms_name       ON rooms (name);
CREATE INDEX IF NOT EXISTS idx_matches_id_room  ON matches (id_room);
CREATE INDEX IF NOT EXISTS idx_matches_id_player ON matches (id_player);
