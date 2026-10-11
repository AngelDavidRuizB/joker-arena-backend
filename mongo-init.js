db = db.getSiblingDB('casino_games');

db.createCollection('hands');
db.createCollection('players');
db.createCollection('rooms');

// Datos de ejemplo del JSON
db.players.insertMany([
    {
        id_player: "789",
        username: "player1",
        email: "player1@example.com",
        password: "hashed_password",
        created_at: new Date("2024-10-26T12:00:00Z"),
        updated_at: new Date("2024-10-27T10:15:00Z")
    },
    {
        id_player: "123",
        username: "player2",
        email: "player2@example.com",
        password: "hashed_password",
        created_at: new Date("2024-10-26T12:15:00Z"),
        updated_at: new Date("2024-10-27T10:20:00Z")
    }
]);

db.rooms.insertMany([
    {
        id_room: "123",
        name: "Texas Hold'em",
        description: "Standard Texas Hold'em game",
        capacity: 6,
        created_at: new Date("2024-10-26T12:30:00Z"),
        updated_at: new Date("2024-10-27T10:30:00Z")
    }
]);

db.hands.insertMany([
    {
        roomId: "room123",
        playerId: "player456",
        handNumber: 1,
        timestamp: new Date("2024-10-27T10:00:00Z"),
        cardsDealt: [
            { rank: "Ace", suit: "Hearts" },
            { rank: "King", suit: "Diamonds" }
        ],
        playerBets: [
            { playerId: "player456", amount: 10.00, currency: "USD" },
            { playerId: "123", amount: 5.00, currency: "USD" }
        ],
        dealerBet: { playerId: "123", amount: 25.00, currency: "USD" },
        result: "player1_win",
        finalHandCards: [
            { rank: "Queen", suit: "Clubs" },
            { rank: "Jack", suit: "Spades" }
        ]
    }
]);

print('Base de datos casino_games inicializada correctamente');