using Npgsql;

namespace RoomsLobby;

public interface IRoomRepository
{
    Task<Room?> CreateAsync(string code, string mode, string difficulty, int target, PlayerRef creator, CancellationToken ct);
    Task<Room?> GetAsync(string code, CancellationToken ct);
    Task<IReadOnlyList<Room>> ListAsync(string? status, int limit, CancellationToken ct);
    Task AddPlayerAsync(string code, PlayerRef player, CancellationToken ct);
    Task<bool> UpdateStatusAsync(string code, string status, CancellationToken ct);
    Task PingAsync(CancellationToken ct);
}

/// <summary>Esquema "lobby" en PostgreSQL (dueño: svc_lobby).</summary>
public sealed class PostgresRoomRepository : IRoomRepository
{
    private readonly NpgsqlDataSource _ds;
    public PostgresRoomRepository(NpgsqlDataSource ds) => _ds = ds;

    public async Task<Room?> CreateAsync(string code, string mode, string difficulty, int target, PlayerRef creator, CancellationToken ct)
    {
        await using var conn = await _ds.OpenConnectionAsync(ct);
        await using var tx = await conn.BeginTransactionAsync(ct);
        await using (var cmd = new NpgsqlCommand(
            "INSERT INTO lobby.rooms (room_code, mode, difficulty, target_score, status, created_by) " +
            "VALUES (@c, @m, @d, @t, 'open', @by) ON CONFLICT (room_code) DO NOTHING", conn, tx))
        {
            cmd.Parameters.AddWithValue("c", code);
            cmd.Parameters.AddWithValue("m", mode);
            cmd.Parameters.AddWithValue("d", difficulty);
            cmd.Parameters.AddWithValue("t", target);
            cmd.Parameters.AddWithValue("by", creator.PlayerId);
            if (await cmd.ExecuteNonQueryAsync(ct) == 0) return null; // código repetido
        }
        await InsertPlayer(conn, tx, code, creator, ct);
        await tx.CommitAsync(ct);
        return await GetAsync(code, ct);
    }

    private static async Task InsertPlayer(NpgsqlConnection conn, NpgsqlTransaction? tx, string code, PlayerRef p, CancellationToken ct)
    {
        await using var cmd = new NpgsqlCommand(
            "INSERT INTO lobby.room_players (room_code, player_id, alias) VALUES (@c, @p, @a) ON CONFLICT DO NOTHING", conn, tx);
        cmd.Parameters.AddWithValue("c", code);
        cmd.Parameters.AddWithValue("p", p.PlayerId);
        cmd.Parameters.AddWithValue("a", p.Alias);
        await cmd.ExecuteNonQueryAsync(ct);
    }

    public async Task AddPlayerAsync(string code, PlayerRef player, CancellationToken ct)
    {
        await using var conn = await _ds.OpenConnectionAsync(ct);
        await InsertPlayer(conn, null, code, player, ct);
    }

    public async Task<Room?> GetAsync(string code, CancellationToken ct)
    {
        var rooms = await Query("WHERE r.room_code = @c", cmd => cmd.Parameters.AddWithValue("c", code), 1, ct);
        return rooms.FirstOrDefault();
    }

    public Task<IReadOnlyList<Room>> ListAsync(string? status, int limit, CancellationToken ct) =>
        status is null
            ? Query("", _ => { }, limit, ct)
            : Query("WHERE r.status = @s", cmd => cmd.Parameters.AddWithValue("s", status), limit, ct);

    private async Task<IReadOnlyList<Room>> Query(string where, Action<NpgsqlCommand> bind, int limit, CancellationToken ct)
    {
        await using var conn = await _ds.OpenConnectionAsync(ct);
        var sql = "SELECT r.room_code, r.mode, r.difficulty, r.target_score, r.status, r.created_by, r.created_at " +
                  $"FROM lobby.rooms r {where} ORDER BY r.created_at DESC LIMIT @lim";
        var rooms = new List<(string code, string mode, string diff, int target, string status, string by, DateTime at)>();
        await using (var cmd = new NpgsqlCommand(sql, conn))
        {
            bind(cmd);
            cmd.Parameters.AddWithValue("lim", limit);
            await using var rd = await cmd.ExecuteReaderAsync(ct);
            while (await rd.ReadAsync(ct))
                rooms.Add((rd.GetString(0), rd.GetString(1), rd.GetString(2), rd.GetInt32(3), rd.GetString(4), rd.GetString(5), rd.GetDateTime(6)));
        }
        if (rooms.Count == 0) return Array.Empty<Room>();
        var players = new Dictionary<string, List<PlayerRef>>();
        await using (var cmd = new NpgsqlCommand(
            "SELECT room_code, player_id, alias FROM lobby.room_players WHERE room_code = ANY(@codes) ORDER BY joined_at", conn))
        {
            cmd.Parameters.AddWithValue("codes", rooms.Select(r => r.code).ToArray());
            await using var rd = await cmd.ExecuteReaderAsync(ct);
            while (await rd.ReadAsync(ct))
            {
                var code = rd.GetString(0);
                if (!players.TryGetValue(code, out var list)) players[code] = list = new List<PlayerRef>();
                list.Add(new PlayerRef(rd.GetString(1), rd.GetString(2)));
            }
        }
        return rooms.Select(r => new Room(r.code, r.mode, r.diff, r.target, r.status, r.by,
            DateTime.SpecifyKind(r.at, DateTimeKind.Utc),
            players.TryGetValue(r.code, out var l) ? l : new List<PlayerRef>())).ToList();
    }

    public async Task<bool> UpdateStatusAsync(string code, string status, CancellationToken ct)
    {
        await using var conn = await _ds.OpenConnectionAsync(ct);
        await using var cmd = new NpgsqlCommand(
            "UPDATE lobby.rooms SET status = @s, updated_at = now() WHERE room_code = @c AND status <> 'closed'", conn);
        cmd.Parameters.AddWithValue("s", status);
        cmd.Parameters.AddWithValue("c", code);
        return await cmd.ExecuteNonQueryAsync(ct) > 0;
    }

    public async Task PingAsync(CancellationToken ct)
    {
        await using var conn = await _ds.OpenConnectionAsync(ct);
        await using var cmd = new NpgsqlCommand("SELECT 1", conn);
        await cmd.ExecuteScalarAsync(ct);
    }
}
