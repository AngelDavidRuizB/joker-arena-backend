namespace RoomsLobby;

public static class Endpoints
{
    private static IResult Error(int status, string code, string message) =>
        Results.Json(new { error = code, message }, statusCode: status);

    private static async Task<IResult> Guard(Func<Task<IResult>> action, ILogger log)
    {
        try { return await action(); }
        catch (Exception ex) when (ex is Npgsql.NpgsqlException or System.Net.Sockets.SocketException or TimeoutException)
        {
            log.LogWarning("PostgreSQL no disponible: {Msg}", ex.Message);
            return Error(503, "store_unavailable", "La base de datos del lobby no está disponible.");
        }
    }

    public static void MapLobbyEndpoints(this WebApplication app)
    {
        var log = app.Logger;

        app.MapGet("/health", async (IRoomRepository repo, EventBus bus) =>
        {
            string pg;
            try { await repo.PingAsync(CancellationToken.None); pg = "ok"; } catch { pg = "down"; }
            var deps = new Dictionary<string, string> { ["postgres"] = pg, ["rabbitmq"] = bus.Connected ? "ok" : "down" };
            return Results.Json(new { status = deps.Values.All(v => v == "ok") ? "ok" : "degraded", service = "rooms-lobby-ms", deps });
        });

        app.MapPost("/rooms", (HttpRequest req, CreateRoomRequest? body, IRoomRepository repo, EventBus bus) => Guard(async () =>
        {
            var player = Identity.FromAuthorization(req.Headers.Authorization);
            if (player is null) return Error(401, "unauthorized", "Se requiere un token válido.");
            var problem = RoomRules.Normalize(body ?? new CreateRoomRequest(null, null, null), out var mode, out var diff, out var target);
            if (problem is not null) return Error(400, "invalid_request", problem);
            for (var attempt = 0; attempt < 10; attempt++)
            {
                var room = await repo.CreateAsync(RoomRules.GenerateCode(), mode, diff, target, player, CancellationToken.None);
                if (room is null) continue; // código repetido: otro intento
                bus.Publish("lobby.room.created", new
                {
                    room_code = room.RoomCode, mode = room.Mode, difficulty = room.Difficulty,
                    target_score = room.TargetScore, created_by = room.CreatedBy,
                });
                return Results.Json(room, statusCode: 201);
            }
            return Error(503, "code_exhausted", "No se pudo generar un código de sala único.");
        }, log));

        app.MapGet("/rooms", (string? status, int? limit, IRoomRepository repo) => Guard(async () =>
        {
            var st = string.IsNullOrWhiteSpace(status) ? RoomStatus.Open : status.Trim().ToLowerInvariant();
            if (st == "all") st = null!;
            else if (st is not (RoomStatus.Open or RoomStatus.InProgress or RoomStatus.Closed))
                return Error(400, "invalid_status", "status debe ser open, in_progress, closed o all.");
            var rooms = await repo.ListAsync(st, Math.Clamp(limit ?? 50, 1, 200), CancellationToken.None);
            return Results.Json(new { rooms });
        }, log));

        app.MapGet("/rooms/{code}", (string code, IRoomRepository repo) => Guard(async () =>
        {
            code = code.Trim().ToUpperInvariant();
            if (!RoomRules.IsValidCode(code)) return Error(404, "room_not_found", "Sala no encontrada.");
            var room = await repo.GetAsync(code, CancellationToken.None);
            return room is null ? Error(404, "room_not_found", "Sala no encontrada.") : Results.Json(room);
        }, log));

        app.MapPost("/rooms/{code}/join", (string code, HttpRequest req, IRoomRepository repo) => Guard(async () =>
        {
            var player = Identity.FromAuthorization(req.Headers.Authorization);
            if (player is null) return Error(401, "unauthorized", "Se requiere un token válido.");
            code = code.Trim().ToUpperInvariant();
            var room = RoomRules.IsValidCode(code) ? await repo.GetAsync(code, CancellationToken.None) : null;
            if (room is null) return Error(404, "room_not_found", "Sala no encontrada.");
            var problem = RoomRules.CanJoin(room, player.PlayerId);
            if (problem is not null) return Error(409, problem, problem == "room_full" ? "La sala está llena." : "La sala ya no acepta jugadores.");
            await repo.AddPlayerAsync(code, player, CancellationToken.None);
            return Results.Json(await repo.GetAsync(code, CancellationToken.None));
        }, log));
    }
}
