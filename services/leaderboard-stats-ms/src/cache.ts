import Redis from "ioredis";

/** Caché del ranking (patrón cache-aside). Un fallo de caché nunca debe tumbar una petición. */
export interface Cache {
  get(key: string): Promise<string | null>;
  set(key: string, value: string, ttlSeconds: number): Promise<void>;
  del(key: string): Promise<void>;
  ping(): Promise<void>;
  close(): Promise<void>;
}

export class RedisCache implements Cache {
  private readonly client: Redis;

  constructor(url: string) {
    this.client = new Redis(url, {
      lazyConnect: false,
      // Si Redis no está disponible, fallar rápido en vez de encolar comandos.
      enableOfflineQueue: false,
      maxRetriesPerRequest: 1,
      connectTimeout: 3_000,
      retryStrategy: (times) => Math.min(times * 500, 5_000),
    });
    // Sin este listener ioredis imprime "Unhandled error event" en cada reintento.
    this.client.on("error", () => undefined);
  }

  async get(key: string): Promise<string | null> {
    return this.client.get(key);
  }

  async set(key: string, value: string, ttlSeconds: number): Promise<void> {
    await this.client.set(key, value, "EX", ttlSeconds);
  }

  async del(key: string): Promise<void> {
    await this.client.del(key);
  }

  async ping(): Promise<void> {
    await this.client.ping();
  }

  async close(): Promise<void> {
    this.client.disconnect();
  }
}
