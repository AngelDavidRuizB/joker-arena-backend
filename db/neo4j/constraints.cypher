// ==============================================================================
// JokerArena P2 - Esquema de Neo4j (dueño: leaderboard-stats-ms)
// leaderboard-stats-ms ejecuta estas mismas sentencias al arrancar
// (src/graph/cypher.ts -> CONSTRAINTS), así que no hace falta correrlas a mano.
//
// Modelo:
//   (:Player {player_id, alias, is_bot, created_at})
//   (:Match  {match_id, room_code, is_bot_match, ended_at})
//   (:Player)-[:PLAYED {score, won}]->(:Match)
//   (:Player)-[:FACED {match_id}]->(:Player)   // un par por partida compartida
// ==============================================================================
CREATE CONSTRAINT player_id_unique IF NOT EXISTS FOR (p:Player) REQUIRE p.player_id IS UNIQUE;
CREATE CONSTRAINT match_id_unique IF NOT EXISTS FOR (m:Match) REQUIRE m.match_id IS UNIQUE;
