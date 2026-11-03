using System.Text;
using System.Text.Json;
using RabbitMQ.Client;
using RabbitMQ.Client.Events;

namespace RoomsLobby;

/// <summary>
/// Publica lobby.room.created y consume game.match.started / game.match.ended
/// (cola rooms-lobby-ms.game). Se reconecta solo; sin broker el lobby sigue respondiendo HTTP.
/// </summary>
public sealed class EventBus : BackgroundService
{
    public const string Exchange = "jokerarena.events";
    public const string Queue = "rooms-lobby-ms.game";
    private readonly string _url;
    private readonly IServiceProvider _sp;
    private readonly ILogger<EventBus> _log;
    private IConnection? _conn;
    private IModel? _pub;
    private readonly object _lock = new();

    public EventBus(IConfiguration cfg, IServiceProvider sp, ILogger<EventBus> log)
    {
        _url = cfg["RABBITMQ_URL"] ?? "";
        _sp = sp;
        _log = log;
    }

    public bool Connected => _conn is { IsOpen: true };

    public void Publish(string routingKey, object payload)
    {
        var envelope = new Dictionary<string, object>
        {
            ["event_id"] = Guid.NewGuid().ToString(),
            ["type"] = routingKey,
            ["version"] = 1,
            ["occurred_at"] = DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ss.fffZ"),
            ["producer"] = "rooms-lobby-ms",
            ["payload"] = payload,
        };
        try
        {
            lock (_lock)
            {
                if (_pub is not { IsOpen: true }) { _log.LogWarning("evento {Key} no publicado: broker no conectado", routingKey); return; }
                var props = _pub.CreateBasicProperties();
                props.ContentType = "application/json";
                props.DeliveryMode = 2;
                _pub.BasicPublish(Exchange, routingKey, props, JsonSerializer.SerializeToUtf8Bytes(envelope));
            }
        }
        catch (Exception ex) { _log.LogError(ex, "no se pudo publicar {Key}", routingKey); }
    }

    protected override async Task ExecuteAsync(CancellationToken stop)
    {
        if (string.IsNullOrEmpty(_url)) { _log.LogWarning("RABBITMQ_URL vacío: eventos deshabilitados"); return; }
        var delay = 1;
        while (!stop.IsCancellationRequested)
        {
            try
            {
                var factory = new ConnectionFactory { Uri = new Uri(_url), DispatchConsumersAsync = true, AutomaticRecoveryEnabled = true };
                _conn = factory.CreateConnection("rooms-lobby-ms");
                lock (_lock)
                {
                    _pub = _conn.CreateModel();
                    _pub.ExchangeDeclare(Exchange, ExchangeType.Topic, durable: true);
                }
                var sub = _conn.CreateModel();
                sub.QueueDeclare(Queue, durable: true, exclusive: false, autoDelete: false);
                sub.QueueBind(Queue, Exchange, "game.match.started");
                sub.QueueBind(Queue, Exchange, "game.match.ended");
                sub.BasicQos(0, 10, false);
                var consumer = new AsyncEventingBasicConsumer(sub);
                consumer.Received += async (_, ea) => await Handle(sub, ea);
                sub.BasicConsume(Queue, autoAck: false, consumer);
                _log.LogInformation("RabbitMQ conectado: consumiendo {Queue}", Queue);
                delay = 1;
                while (!stop.IsCancellationRequested && _conn.IsOpen) await Task.Delay(1000, stop);
            }
            catch (OperationCanceledException) { break; }
            catch (Exception ex)
            {
                _log.LogWarning("RabbitMQ no disponible ({Msg}); reintento en {Delay}s", ex.Message, delay);
                try { await Task.Delay(TimeSpan.FromSeconds(delay), stop); } catch (OperationCanceledException) { break; }
                delay = Math.Min(delay * 2, 15);
            }
        }
    }

    private async Task Handle(IModel ch, BasicDeliverEventArgs ea)
    {
        try
        {
            using var doc = JsonDocument.Parse(Encoding.UTF8.GetString(ea.Body.ToArray()));
            var type = doc.RootElement.GetProperty("type").GetString() ?? "";
            var code = doc.RootElement.GetProperty("payload").GetProperty("room_code").GetString() ?? "";
            using var scope = _sp.CreateScope();
            var repo = scope.ServiceProvider.GetRequiredService<IRoomRepository>();
            var room = await repo.GetAsync(code, CancellationToken.None);
            var next = room is null ? null : RoomRules.NextStatus(room.Status, type);
            if (next is not null)
            {
                await repo.UpdateStatusAsync(code, next, CancellationToken.None);
                _log.LogInformation("sala {Code}: {From} -> {To} ({Type})", code, room!.Status, next, type);
            }
            ch.BasicAck(ea.DeliveryTag, false);
        }
        catch (Exception ex) when (ex is JsonException or KeyNotFoundException or InvalidOperationException)
        {
            _log.LogError("mensaje descartado (fuera de contrato): {Msg}", ex.Message);
            ch.BasicNack(ea.DeliveryTag, false, false);
        }
        catch (Exception ex)
        {
            _log.LogWarning("fallo transitorio procesando evento: {Msg}", ex.Message);
            await Task.Delay(2000);
            ch.BasicNack(ea.DeliveryTag, false, true);
        }
    }

    public override void Dispose()
    {
        try { _pub?.Close(); _conn?.Close(); } catch { /* ya cerrado */ }
        base.Dispose();
    }
}
