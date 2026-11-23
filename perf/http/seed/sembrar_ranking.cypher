// =============================================================================
// JokerArena P3 · D1 · Datos de prueba para el ranking (Neo4j)
// 1.000 jugadores y 4.000 partidas de 2 jugadores (8.000 relaciones PLAYED),
// para que la consulta del top recorra un grafo de tamaño realista.
// Todo lo creado lleva el prefijo "perf-" y se borra con limpiar_ranking.cypher.
//
// Uso (desde la raíz del repo, con el sistema levantado):
//   docker exec -i jokerarena-neo4j sh -c 'cypher-shell -u neo4j -p "${NEO4J_AUTH#neo4j/}"' < perf/http/seed/sembrar_ranking.cypher
// =============================================================================
UNWIND range(1, 1000) AS i
MERGE (p:Player {player_id: 'perf-p-' + toString(i)})
  ON CREATE SET p.created_at = datetime()
SET p.alias = 'perf' + toString(i), p.is_bot = false;

UNWIND range(1, 4000) AS j
WITH j, 1 + (j * 7) % 1000 AS a, 1 + (j * 13 + 5) % 1000 AS b
WHERE a <> b
MATCH (pa:Player {player_id: 'perf-p-' + toString(a)})
MATCH (pb:Player {player_id: 'perf-p-' + toString(b)})
MERGE (m:Match {match_id: 'perf-m-' + toString(j)})
  ON CREATE SET m.room_code = 'JKR-PERF', m.is_bot_match = false, m.ended_at = datetime()
MERGE (pa)-[:PLAYED {score: (j * 37) % 5000 + 100, won: true}]->(m)
MERGE (pb)-[:PLAYED {score: (j * 53) % 5000 + 100, won: false}]->(m);

MATCH (p:Player) WHERE p.player_id STARTS WITH 'perf-'
RETURN count(p) AS jugadores_perf;
