using System.Text;
using RoomsLobby;
using Xunit;

public class RoomRulesTests
{
    private static Room MakeRoom(string mode = "pvp", string status = RoomStatus.Open, int players = 1) =>
        new("JKR-AB23", mode, "normal", 22000, status, "p0", DateTime.UtcNow,
            Enumerable.Range(0, players).Select(i => new PlayerRef($"p{i}", $"P{i}")).ToList());

    [Fact]
    public void GeneratedCodesHaveTheJkrFormatWithoutAmbiguousCharacters()
    {
        for (var i = 0; i < 500; i++)
        {
            var code = RoomRules.GenerateCode();
            Assert.True(RoomRules.IsValidCode(code), code);
            Assert.DoesNotContain(code[4..], c => "01OIL".Contains(c));
        }
    }

    [Theory]
    [InlineData("JKR-AB23", true)]
    [InlineData("JKR-AB2", false)]
    [InlineData("ABC-AB23", false)]
    [InlineData("JKR-AB0O", false)]
    public void ValidatesCodes(string code, bool valid) => Assert.Equal(valid, RoomRules.IsValidCode(code));

    [Fact]
    public void NormalizeAppliesDefaults()
    {
        Assert.Null(RoomRules.Normalize(new CreateRoomRequest(null, null, null), out var m, out var d, out var t));
        Assert.Equal(("bot", "normal", 22000), (m, d, t));
    }

    [Theory]
    [InlineData("solo", "normal", 100)]
    [InlineData("bot", "imposible", 100)]
    [InlineData("bot", "easy", 0)]
    [InlineData("bot", "easy", 100001)]
    public void NormalizeRejectsInvalidValues(string mode, string diff, int target) =>
        Assert.NotNull(RoomRules.Normalize(new CreateRoomRequest(mode, diff, target), out _, out _, out _));

    [Fact]
    public void JoinRules()
    {
        Assert.Null(RoomRules.CanJoin(MakeRoom(), "nuevo"));
        Assert.Null(RoomRules.CanJoin(MakeRoom(), "p0")); // ya está: idempotente
        Assert.Equal("room_full", RoomRules.CanJoin(MakeRoom("bot"), "nuevo"));
        Assert.Equal("room_full", RoomRules.CanJoin(MakeRoom(players: 4), "nuevo"));
        Assert.Equal("room_not_open", RoomRules.CanJoin(MakeRoom(status: RoomStatus.Closed), "nuevo"));
    }

    [Theory]
    [InlineData(RoomStatus.Open, "game.match.started", RoomStatus.InProgress)]
    [InlineData(RoomStatus.Open, "game.match.ended", RoomStatus.Closed)]
    [InlineData(RoomStatus.InProgress, "game.match.ended", RoomStatus.Closed)]
    [InlineData(RoomStatus.Closed, "game.match.started", null)]
    [InlineData(RoomStatus.InProgress, "game.match.started", null)]
    public void StatusOnlyMovesForward(string from, string evt, string? to) =>
        Assert.Equal(to, RoomRules.NextStatus(from, evt));

    private static string Token(string json) =>
        "Bearer x." + Convert.ToBase64String(Encoding.UTF8.GetBytes(json)).TrimEnd('=').Replace('+', '-').Replace('/', '_') + ".sig";

    [Fact]
    public void ReadsTheIdentityFromTheForwardedToken()
    {
        var exp = DateTimeOffset.UtcNow.AddHours(1).ToUnixTimeSeconds();
        var p = Identity.FromAuthorization(Token($"{{\"sub\":\"abc\",\"alias\":\"Ana\",\"exp\":{exp}}}"));
        Assert.Equal(new PlayerRef("abc", "Ana"), p);
        Assert.Null(Identity.FromAuthorization(null));
        Assert.Null(Identity.FromAuthorization("Bearer roto"));
        Assert.Null(Identity.FromAuthorization(Token("{\"alias\":\"sin sub\"}")));
        Assert.Null(Identity.FromAuthorization(Token("{\"sub\":\"abc\",\"exp\":1}"))); // vencido
    }
}
