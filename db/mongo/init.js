// JokerArena P2 - Índices de la base "telemetry" (dueño: telemetry-ms).
// telemetry-ms también los crea al arrancar; este script deja la base lista desde el inicio.
const tdb = db.getSiblingDB("telemetry");
tdb.createCollection("events");
tdb.events.createIndex({ event_id: 1 }, { unique: true });
tdb.createCollection("hand_events");
tdb.hand_events.createIndex({ match_id: 1, player_id: 1, hand_number: 1 }, { unique: true });
tdb.hand_events.createIndex({ occurred_at: -1 });
tdb.createCollection("match_summaries");
tdb.match_summaries.createIndex({ match_id: 1 }, { unique: true });
tdb.match_summaries.createIndex({ ended_at: -1 });
