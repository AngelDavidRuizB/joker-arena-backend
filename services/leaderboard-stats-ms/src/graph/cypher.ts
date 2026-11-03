/**
 * Consultas Cypher del servicio. Están separadas para poder revisarlas y
 * probarlas sin una base Neo4j real. Las restricciones también están en
 * db/neo4j/constraints.cypher (mismo contenido) como documentación del esquema.
 */

export const CONSTRAINTS = [
  "CREATE CONSTRAINT player_id_unique IF NOT EXISTS FOR (p:Player) REQUIRE p.player_id IS UNIQUE",
  "CREATE CONSTRAINT match_id_unique IF NOT EXISTS FOR (m:Match) REQUIRE m.match_id IS UNIQUE",
];

export const UPSERT_PLAYER = `
MERGE (p:Player {player_id: $player_id})
  ON CREATE SET p.created_at = datetime(), p.is_bot = false
SET p.alias = $alias`;

export const MATCH_EXISTS = `
OPTIONAL MATCH (m:Match {match_id: $match_id})
RETURN m IS NOT NULL AS exists`;

/** Crea la partida y una relación PLAYED por jugador (y nodos Player si aún no existen). */
export const CREATE_MATCH = `
CREATE (m:Match {
  match_id: $match_id,
  room_code: $room_code,
  is_bot_match: $is_bot_match,
  ended_at: datetime($ended_at)
})
WITH m
UNWIND $results AS r
MERGE (p:Player {player_id: r.player_id})
  ON CREATE SET p.created_at = datetime()
SET p.is_bot = coalesce(r.is_bot, false),
    p.alias = coalesce(p.alias, r.alias)
CREATE (p)-[:PLAYED {score: r.score, won: r.won}]->(m)`;

/** Una relación FACED por cada par de jugadores que compartieron la partida. */
export const CREATE_FACED = `
MATCH (a:Player)-[:PLAYED]->(m:Match {match_id: $match_id})<-[:PLAYED]-(b:Player)
WHERE a.player_id < b.player_id
CREATE (a)-[:FACED {match_id: $match_id}]->(b)`;

/** Ranking global: el bot no compite. Orden: victorias, mejor puntaje, menos partidas. */
export const TOP = `
MATCH (p:Player)-[r:PLAYED]->(:Match)
WHERE coalesce(p.is_bot, false) = false
WITH p,
     count(r) AS matches,
     sum(CASE WHEN r.won THEN 1 ELSE 0 END) AS wins,
     max(r.score) AS best_score,
     sum(r.score) AS total_score
RETURN p.player_id AS player_id, p.alias AS alias, matches, wins, best_score, total_score
ORDER BY wins DESC, best_score DESC, matches ASC, player_id ASC
LIMIT $limit`;

export const PLAYER_STATS = `
MATCH (p:Player {player_id: $player_id})
OPTIONAL MATCH (p)-[r:PLAYED]->(:Match)
WITH p,
     count(r) AS matches,
     sum(CASE WHEN r.won THEN 1 ELSE 0 END) AS wins,
     coalesce(max(r.score), 0) AS best_score,
     coalesce(sum(r.score), 0) AS total_score
OPTIONAL MATCH (p)-[f:FACED]-(o:Player)
WITH p, matches, wins, best_score, total_score, o, count(f) AS times
ORDER BY times DESC, o.player_id ASC
WITH p, matches, wins, best_score, total_score,
     collect(CASE WHEN o IS NULL THEN NULL ELSE {
       player_id: o.player_id,
       alias: o.alias,
       is_bot: coalesce(o.is_bot, false),
       matches: times
     } END) AS rivals
RETURN p.player_id AS player_id, p.alias AS alias, matches, wins, best_score, total_score,
       rivals[0..$max_rivals] AS rivals`;
