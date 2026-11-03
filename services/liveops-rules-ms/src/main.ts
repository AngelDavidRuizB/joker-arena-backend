import pino from "pino";
import { buildApp, type DepStatus } from "./app";
import { AmqpPublisher, RedisRulesetCache } from "./infra";
import { LiveopsService } from "./service";
import { PostgresRulesetRepository } from "./store";

async function main(): Promise<void> {
  const logger = pino({ name: "liveops-rules-ms", level: process.env.LOG_LEVEL ?? "info" });
  const repo = new PostgresRulesetRepository(process.env.POSTGRES_URL ?? "");
  const cache = new RedisRulesetCache(process.env.REDIS_URL ?? "redis://jokerarena-redis:6379/0");
  const publisher = new AmqpPublisher(process.env.RABBITMQ_URL ?? "", process.env.RABBITMQ_EXCHANGE ?? "jokerarena.events", logger);
  if (process.env.RABBITMQ_URL) publisher.start();
  const service = new LiveopsService(repo, cache, publisher, logger);
  const ok = async (f: () => Promise<void>): Promise<DepStatus> => {
    try {
      await f();
      return "ok";
    } catch {
      return "down";
    }
  };
  const app = buildApp(service, {
    postgres: () => ok(() => repo.ping()),
    redis: () => ok(() => cache.ping()),
    rabbitmq: async () => (publisher.connected ? "ok" : "down"),
  }, logger);
  await app.listen({ host: "0.0.0.0", port: Number(process.env.PORT ?? 8087) });

  const shutdown = async () => {
    await app.close();
    await publisher.close();
    await cache.close();
    await repo.close();
    process.exit(0);
  };
  process.on("SIGTERM", () => void shutdown());
  process.on("SIGINT", () => void shutdown());
}

main().catch((e) => {
  // eslint-disable-next-line no-console
  console.error("No se pudo iniciar liveops-rules-ms:", e);
  process.exit(1);
});
