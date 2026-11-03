using Npgsql;
using RoomsLobby;

var port = Environment.GetEnvironmentVariable("PORT") ?? "8083";

// La imagen aspnet no trae curl/wget: el HEALTHCHECK de Docker ejecuta este mismo binario.
if (args.Contains("--healthcheck"))
{
    using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(2) };
    try { return (await http.GetAsync($"http://127.0.0.1:{port}/health")).IsSuccessStatusCode ? 0 : 1; }
    catch { return 1; }
}

var builder = WebApplication.CreateBuilder(args);
builder.WebHost.UseUrls($"http://0.0.0.0:{port}");
builder.Logging.AddSimpleConsole(o => { o.SingleLine = true; o.TimestampFormat = "HH:mm:ss "; });

var pgUrl = builder.Configuration["POSTGRES_URL"] ?? "";
builder.Services.AddSingleton(_ => new NpgsqlDataSourceBuilder(ToNpgsql(pgUrl)).Build());
builder.Services.AddSingleton<IRoomRepository, PostgresRoomRepository>();
builder.Services.AddSingleton<EventBus>();
builder.Services.AddHostedService(sp => sp.GetRequiredService<EventBus>());

var app = builder.Build();
app.MapLobbyEndpoints();
app.Run();
return 0;

// Acepta la URL postgres://user:pass@host:port/db y la convierte al formato de Npgsql.
static string ToNpgsql(string url)
{
    if (!url.StartsWith("postgres")) return url;
    var u = new Uri(url);
    var userInfo = u.UserInfo.Split(':', 2);
    return new NpgsqlConnectionStringBuilder
    {
        Host = u.Host,
        Port = u.Port > 0 ? u.Port : 5432,
        Username = Uri.UnescapeDataString(userInfo[0]),
        Password = userInfo.Length > 1 ? Uri.UnescapeDataString(userInfo[1]) : "",
        Database = u.AbsolutePath.TrimStart('/'),
        Timeout = 3,
    }.ConnectionString;
}

public partial class Program { }
