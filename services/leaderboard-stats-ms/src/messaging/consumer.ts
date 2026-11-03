import amqp, { type Channel, type Connection, type ConsumeMessage } from "amqplib";
import { InvalidEventError, ROUTING_KEYS } from "../events";
import type { Logger } from "../logger";
import type { LeaderboardService } from "../service";

/** Colas propias del servicio (convención <servicio>.<propósito>, sección 0.7). */
export const QUEUES = [
  { name: "leaderboard-stats-ms.players", routingKey: ROUTING_KEYS.playerRegistered },
  { name: "leaderboard-stats-ms.matches", routingKey: ROUTING_KEYS.matchEnded },
] as const;

export type DeliveryOutcome = "ack" | "reject" | "retry";
export type ConsumerStatus = "connecting" | "connected" | "disconnected";

/**
 * Decide qué hacer con un mensaje. Separado de amqplib para poder probarlo:
 * - ack: procesado (o duplicado/ignorado).
 * - reject: mensaje inválido; reintentarlo no lo arreglaría.
 * - retry: fallo transitorio (p. ej. Neo4j caído); se devuelve a la cola.
 */
export async function handleDelivery(
  content: Buffer,
  service: LeaderboardService,
  logger: Logger,
): Promise<DeliveryOutcome> {
  let raw: unknown;
  try {
    raw = JSON.parse(content.toString("utf8"));
  } catch {
    logger.error({ body: content.toString("utf8").slice(0, 200) }, "mensaje descartado: no es JSON válido");
    return "reject";
  }
  try {
    await service.handleEvent(raw);
    return "ack";
  } catch (err) {
    if (err instanceof InvalidEventError) {
      logger.error({ err: err.message }, "mensaje descartado: no cumple el contrato de eventos");
      return "reject";
    }
    logger.warn({ err: (err as Error).message }, "fallo transitorio procesando el evento: se reintentará");
    return "retry";
  }
}

const RETRY_DELAY_MS = 2_000;
const MAX_RECONNECT_DELAY_MS = 15_000;

/** Consumidor RabbitMQ con reconexión automática; el servicio sigue respondiendo HTTP sin broker. */
export class EventConsumer {
  private connection: Connection | null = null;
  private channel: Channel | null = null;
  private stopped = false;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private attempt = 0;
  status: ConsumerStatus = "disconnected";

  constructor(
    private readonly url: string,
    private readonly exchange: string,
    private readonly service: LeaderboardService,
    private readonly logger: Logger,
  ) {}

  start(): void {
    this.stopped = false;
    void this.connect();
  }

  private async connect(): Promise<void> {
    if (this.stopped) return;
    this.status = "connecting";
    try {
      const connection = await amqp.connect(this.url);
      connection.on("error", (err: Error) => this.logger.warn({ err: err.message }, "error en la conexión AMQP"));
      connection.on("close", () => this.onDisconnect());
      const channel = await connection.createChannel();
      await channel.assertExchange(this.exchange, "topic", { durable: true });
      await channel.prefetch(10);
      for (const q of QUEUES) {
        await channel.assertQueue(q.name, { durable: true });
        await channel.bindQueue(q.name, this.exchange, q.routingKey);
        await channel.consume(q.name, (msg) => void this.onMessage(channel, msg), { noAck: false });
      }
      this.connection = connection;
      this.channel = channel;
      this.attempt = 0;
      this.status = "connected";
      this.logger.info({ exchange: this.exchange, queues: QUEUES.map((q) => q.name) }, "consumidor AMQP conectado");
    } catch (err) {
      this.logger.warn({ err: (err as Error).message }, "no se pudo conectar a RabbitMQ");
      this.onDisconnect();
    }
  }

  private async onMessage(channel: Channel, msg: ConsumeMessage | null): Promise<void> {
    if (!msg) return; // la cola fue cancelada por el broker
    const outcome = await handleDelivery(msg.content, this.service, this.logger);
    try {
      if (outcome === "ack") channel.ack(msg);
      else if (outcome === "reject") channel.nack(msg, false, false);
      else setTimeout(() => {
        try {
          channel.nack(msg, false, true);
        } catch {
          // El canal se cerró mientras esperábamos: RabbitMQ reentrega el mensaje solo.
        }
      }, RETRY_DELAY_MS);
    } catch (err) {
      this.logger.warn({ err: (err as Error).message }, "no se pudo confirmar el mensaje (canal cerrado)");
    }
  }

  private onDisconnect(): void {
    this.connection = null;
    this.channel = null;
    this.status = "disconnected";
    if (this.stopped || this.reconnectTimer) return;
    const delay = Math.min(1_000 * 2 ** this.attempt, MAX_RECONNECT_DELAY_MS);
    this.attempt += 1;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      void this.connect();
    }, delay);
  }

  async stop(): Promise<void> {
    this.stopped = true;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
    try {
      await this.channel?.close();
      await this.connection?.close();
    } catch {
      // ya estaba cerrada
    }
    this.status = "disconnected";
  }
}
