using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace RoomsLobby;

public static class RoomStatus
{
    public const string Open = "open";
    public const string InProgress = "in_progress";
    public const string Closed = "closed";
}

public sealed record PlayerRef(
    [property: JsonPropertyName("player_id")] string PlayerId,
    [property: JsonPropertyName("alias")] string Alias);

public sealed record Room(
    [property: JsonPropertyName("room_code")] string RoomCode,
    [property: JsonPropertyName("mode")] string Mode,
    [property: JsonPropertyName("difficulty")] string Difficulty,
    [property: JsonPropertyName("target_score")] int TargetScore,
    [property: JsonPropertyName("status")] string Status,
    [property: JsonPropertyName("created_by")] string CreatedBy,
    [property: JsonPropertyName("created_at")] DateTime CreatedAt,
    [property: JsonPropertyName("players")] IReadOnlyList<PlayerRef> Players);

public sealed record CreateRoomRequest(
    [property: JsonPropertyName("mode")] string? Mode,
    [property: JsonPropertyName("difficulty")] string? Difficulty,
    [property: JsonPropertyName("target_score")] int? TargetScore);

/// <summary>Reglas del lobby, sin dependencias de infraestructura (se prueban con xUnit).</summary>
public static class RoomRules
{
    public const int DefaultTargetScore = 22000;
    public const int MaxTargetScore = 100000;
    public const int MaxPlayers = 4;
    private const string Alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"; // sin 0/O ni 1/I/L
    public static readonly string[] Modes = { "bot", "pvp" };
    public static readonly string[] Difficulties = { "easy", "normal", "hard" };

    public static string GenerateCode()
    {
        var sb = new StringBuilder("JKR-");
        for (var i = 0; i < 4; i++) sb.Append(Alphabet[RandomNumberGenerator.GetInt32(Alphabet.Length)]);
        return sb.ToString();
    }

    public static bool IsValidCode(string code) =>
        code.Length == 8 && code.StartsWith("JKR-") && code[4..].All(c => Alphabet.Contains(c));

    /// <summary>Valida y normaliza la petición. Devuelve un mensaje de error o null.</summary>
    public static string? Normalize(CreateRoomRequest req, out string mode, out string difficulty, out int target)
    {
        mode = (req.Mode ?? "bot").Trim().ToLowerInvariant();
        difficulty = (req.Difficulty ?? "normal").Trim().ToLowerInvariant();
        target = req.TargetScore ?? DefaultTargetScore;
        if (!Modes.Contains(mode)) return "mode debe ser 'bot' o 'pvp'.";
        if (!Difficulties.Contains(difficulty)) return "difficulty debe ser 'easy', 'normal' o 'hard'.";
        if (target < 1 || target > MaxTargetScore) return $"target_score debe estar entre 1 y {MaxTargetScore}.";
        return null;
    }

    /// <summary>¿Puede este jugador unirse? Devuelve un código de error o null.</summary>
    public static string? CanJoin(Room room, string playerId)
    {
        if (room.Players.Any(p => p.PlayerId == playerId)) return null; // ya está: idempotente
        if (room.Status != RoomStatus.Open) return "room_not_open";
        if (room.Mode == "bot") return "room_full"; // contra el bot juega solo el creador
        if (room.Players.Count >= MaxPlayers) return "room_full";
        return null;
    }

    /// <summary>Transición de estado provocada por un evento del motor (nunca retrocede).</summary>
    public static string? NextStatus(string current, string eventType) => (current, eventType) switch
    {
        (RoomStatus.Open, "game.match.started") => RoomStatus.InProgress,
        (RoomStatus.Open or RoomStatus.InProgress, "game.match.ended") => RoomStatus.Closed,
        _ => null,
    };
}

/// <summary>Identidad del jugador tomada del JWT que el gateway ya validó y reenvía.</summary>
public static class Identity
{
    public static PlayerRef? FromAuthorization(string? header)
    {
        if (string.IsNullOrWhiteSpace(header) || !header.StartsWith("Bearer ", StringComparison.OrdinalIgnoreCase)) return null;
        var parts = header[7..].Trim().Split('.');
        if (parts.Length != 3) return null;
        try
        {
            var payload = parts[1].Replace('-', '+').Replace('_', '/');
            payload = payload.PadRight(payload.Length + (4 - payload.Length % 4) % 4, '=');
            using var doc = JsonDocument.Parse(Convert.FromBase64String(payload));
            var root = doc.RootElement;
            var sub = root.TryGetProperty("sub", out var s) ? s.GetString() : null;
            var alias = root.TryGetProperty("alias", out var a) ? a.GetString() : null;
            if (string.IsNullOrEmpty(sub)) return null;
            if (root.TryGetProperty("exp", out var exp) && exp.TryGetInt64(out var e) && DateTimeOffset.UtcNow.ToUnixTimeSeconds() >= e) return null;
            return new PlayerRef(sub, alias ?? "");
        }
        catch (Exception) { return null; }
    }
}
