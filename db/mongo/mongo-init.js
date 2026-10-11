// ==============================================================================
// JokerArena - Inicialización de MongoDB 7 (base casino_games)
// Basado en el mongo-init.js entregado por el equipo. Se omiten los documentos de
// ejemplo para que las métricas/telemetría reflejen solo partidas reales.
// ==============================================================================
db = db.getSiblingDB('casino_games');

db.createCollection('hands');
db.createCollection('players');
db.createCollection('rooms');

// Índices de consulta para el servicio de telemetría
db.hands.createIndex({ roomId: 1, handNumber: 1 });
db.hands.createIndex({ playerId: 1 });
db.hands.createIndex({ timestamp: -1 });

print('Base de datos casino_games inicializada correctamente');
