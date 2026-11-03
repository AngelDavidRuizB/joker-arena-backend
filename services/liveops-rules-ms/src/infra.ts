import amqp, { type Channel, type Connection } from "amqplib";
import Redis from "ioredis";
import { randomUUID } from "node:crypto";
import type { Logger } from "./logger";
import type { Ruleset } from "./ruleset";

export const CACHE_KEY = "liveops:ruleset:current";
export const ROUTING_KEY = "liveops.ruleset.updated";

export interface RulesetCache {
  get(): Promise<Ruleset | null>;
  set(rs: Ruleset): Promise<void>;
  ping(): Promise<void>;
  close(): Promise<void>;
}

export class RedisRulesetCache implements RulesetCache {
  private readonly client: Redis;
  constructor(url: string) {
    this.client = new Redis(url, { enableOfflineQueue: false, maxRetriesPerRequest: 1, connectTimeout: 3_000 });
    this.client.on("error", () => undefined);
  }
  async get(): Promise<Ruleset | null> {
    const raw = await this.client.get(CACHE_KEY);
    return raw ? (JSON.parse(raw) as Ruleset) : null;
  }
  async set(rs: Ruleset): Promise<void> {
    await this.client.set(CACHE_KEY, JSON.stringify(rs));
  }
  async ping(): Promise<void> {
    await this.client.ping();
  }
  async close(): Promise<void> {
    this.client.disconnect();
  }
}

export interface EventPublisher {
  publish(routingKey: string, payload: unknown): Promise<boolean>;
  readonly connected: boolean;
}

/** Publicador con confirmaciones del broker y reconexión en segundo plano. */
export class AmqpPublisher implements EventPublisher {
  private conn: Connection | null = null;
  private channel: Channel | null = null;
  private stopped = false;

  constructor(private readonly url: string, private readonly exchange: string, private readonly logger: Logger) {}

  get connected(): boolean {
    return this.channel !== null;
  }

  start(): void {
    void this.connect(1_000);
  }

  private async connect(delay: number): Promise<void> {
    if (this.stopped) return;
    try {
      const conn = await amqp.connect(this.url);
      conn.on("error", () => undefined);
      conn.on("close", () => {
        this.conn = null;
        this.channel = null;
        if (!this.stopped) setTimeout(() => void this.connect(1_000), 1_000);
      });
      const ch = await conn.createConfirmChannel();
      await ch.assertExchange(this.exchange, "topic", { durable: true });
      this.conn = conn;
      this.channel = ch;
      this.logger.info({ exchange: this.exchange }, "publicador AMQP conectado");
    } catch (err) {
      this.logger.warn({ err: (err as Error).message }, "RabbitMQ no disponible; reintentando");
      setTimeout(() => void this.connect(Math.min(delay * 2, 15_000)), delay);
    }
  }

  async publish(routingKey: string, payload: unknown): Promise<boolean> {
    const ch = this.channel as (Channel & { waitForConfirms(): Promise<void> }) | null;
    if (!ch) return false;
    const envelope = {
      event_id: randomUUID(),
      type: routingKey,
      version: 1,
      occurred_at: new Date().toISOString(),
      producer: "liveops-rules-ms",
      payload,
    };
    try {
      ch.publish(this.exchange, routingKey, Buffer.from(JSON.stringify(envelope)), { contentType: "application/json", persistent: true });
      await ch.waitForConfirms();
      return true;
    } catch (err) {
      this.logger.error({ err: (err as Error).message, routingKey }, "no se pudo publicar el evento");
      return false;
    }
  }

  async close(): Promise<void> {
    this.stopped = true;
    try {
      await this.conn?.close();
    } catch {
      // ya cerrada
    }
  }
}
