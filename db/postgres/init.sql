-- ==============================================================================
-- JokerArena: Multiverse Poker Engine
-- Script de Inicialización DDL y Datos Semilla para PostgreSQL 16
-- Base de Datos: jokerarena_db
-- ==============================================================================

-- Extensión para generación de UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Tabla de Jugadores (Players)
CREATE TABLE IF NOT EXISTS players (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    alias      VARCHAR(50) UNIQUE NOT NULL,
    total_wins INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. Catálogo Estático de Jokers
CREATE TABLE IF NOT EXISTS jokers (
    id            VARCHAR(30) PRIMARY KEY,          -- 'banner', 'joker', 'the_duo'
    name          VARCHAR(50) NOT NULL,
    description   TEXT NOT NULL,
    modifier_type VARCHAR(20) NOT NULL CHECK (modifier_type IN ('ADD_CHIPS', 'ADD_MULT', 'X_MULT')),
    default_val   NUMERIC(6,2) NOT NULL,
    apply_order   SMALLINT NOT NULL
);

-- 3. Registro Maestro de Partidas (Matches)
CREATE TABLE IF NOT EXISTS matches (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_code    VARCHAR(10) UNIQUE NOT NULL,
    target_score INT NOT NULL DEFAULT 300,
    status       VARCHAR(20) NOT NULL DEFAULT 'WAITING' CHECK (status IN ('WAITING', 'IN_PROGRESS', 'FINISHED')),
    winner_id    UUID REFERENCES players(id) ON DELETE SET NULL,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at  TIMESTAMPTZ
);

-- 4. Participantes de Partida (Match Players)
CREATE TABLE IF NOT EXISTS match_players (
    match_id      UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    player_id     UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    is_bot        BOOLEAN NOT NULL DEFAULT false,
    final_score   INT NOT NULL DEFAULT 0,
    hands_left    INT NOT NULL DEFAULT 4,
    discards_left INT NOT NULL DEFAULT 3,
    PRIMARY KEY (match_id, player_id)
);

-- 5. Índices para Optimización de Consultas Relacionales
CREATE INDEX IF NOT EXISTS idx_match_players_player_id ON match_players(player_id);
CREATE INDEX IF NOT EXISTS idx_matches_winner_id ON matches(winner_id);
CREATE INDEX IF NOT EXISTS idx_matches_status ON matches(status);

-- ==============================================================================
-- SEED DATA: Catálogo Oficial de Jokers del Prototipo 1
-- ==============================================================================
INSERT INTO jokers (id, name, description, modifier_type, default_val, apply_order) VALUES
('banner', 'Banner', '+30 Chips por cada descarte restante en la mano', 'ADD_CHIPS', 30.00, 1),
('joker', 'Joker Clásico', '+4 Mult fijo a toda mano jugada', 'ADD_MULT', 4.00, 2),
('the_duo', 'The Duo', 'x2 Mult exponencial si la mano contiene al menos un Par', 'X_MULT', 2.00, 3)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    modifier_type = EXCLUDED.modifier_type,
    default_val = EXCLUDED.default_val,
    apply_order = EXCLUDED.apply_order;

-- Usuario del Bot por defecto (Solo vs Bot Mode)
INSERT INTO players (id, alias, total_wins) VALUES
('00000000-0000-0000-0000-000000000001', 'Bot-JokerBot', 0)
ON CONFLICT (id) DO NOTHING;
