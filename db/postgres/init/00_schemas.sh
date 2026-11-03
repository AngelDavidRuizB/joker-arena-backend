#!/bin/sh
# ==============================================================================
# JokerArena P2 - Crea un usuario y un esquema por servicio (Database per service).
# Lo ejecuta la imagen oficial de PostgreSQL solo la primera vez (volumen vacío).
# Las contraseñas llegan por variables de entorno (docker-compose.p2.yml).
# ==============================================================================
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  -v auth_pw="$AUTH_DB_PASSWORD" \
  -v lobby_pw="$LOBBY_DB_PASSWORD" \
  -v game_pw="$GAME_DB_PASSWORD" \
  -v liveops_pw="$LIVEOPS_DB_PASSWORD" \
  -v monitor_pw="$MONITOR_DB_PASSWORD" <<'SQL'
CREATE ROLE svc_auth    LOGIN PASSWORD :'auth_pw';
CREATE ROLE svc_lobby   LOGIN PASSWORD :'lobby_pw';
CREATE ROLE svc_game    LOGIN PASSWORD :'game_pw';
CREATE ROLE svc_liveops LOGIN PASSWORD :'liveops_pw';

CREATE SCHEMA auth    AUTHORIZATION svc_auth;
CREATE SCHEMA lobby   AUTHORIZATION svc_lobby;
CREATE SCHEMA game    AUTHORIZATION svc_game;
CREATE SCHEMA liveops AUTHORIZATION svc_liveops;

-- Cada usuario solo ve su propio esquema.
REVOKE ALL ON SCHEMA public FROM PUBLIC;
ALTER ROLE svc_auth    SET search_path = auth;
ALTER ROLE svc_lobby   SET search_path = lobby;
ALTER ROLE svc_game    SET search_path = game;
ALTER ROLE svc_liveops SET search_path = liveops;

-- Usuario de monitorización de solo lectura (cluster-monitor-ms): estadísticas, sin datos.
CREATE ROLE svc_monitor LOGIN PASSWORD :'monitor_pw';
GRANT pg_monitor TO svc_monitor;
SQL
