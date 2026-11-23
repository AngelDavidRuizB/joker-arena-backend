// Borra los datos de prueba creados por sembrar_ranking.cypher (prefijo "perf-").
//   docker exec -i jokerarena-neo4j sh -c 'cypher-shell -u neo4j -p "${NEO4J_AUTH#neo4j/}"' < perf/http/seed/limpiar_ranking.cypher
//   docker exec jokerarena-redis redis-cli DEL leaderboard:top
MATCH (m:Match) WHERE m.match_id STARTS WITH 'perf-' DETACH DELETE m;
MATCH (p:Player) WHERE p.player_id STARTS WITH 'perf-' DETACH DELETE p;
