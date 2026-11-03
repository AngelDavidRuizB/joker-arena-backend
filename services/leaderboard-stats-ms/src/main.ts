import pino from "pino";
import { RedisCache } from "./cache";
import { loadConfig } from "./config";
import { Neo4jLeaderboardRepository, createNeo4jDriver } from "./graph/neo4j.repository";
import { SERVICE_NAME, buildApp, type DepStatus } from "./http/app";
import { EventConsumer } from "./messaging/consumer";
import { LeaderboardService } from "./service";

const SCHEMA_RETRY_MS = 5_000;

async function main(): Promise<void> {
  const config = loadConfig();
  const logger = pino({ name: SERVICE_NAME, level: config.logLevel });

  const repo = new Neo4jLeaderboardRepository(
    createNeo4jDriver(config.neo4jUrl, config.neo4jUser, config.neo4jPassword),
  );
  const cache = new RedisCache(config.redisUrl);
  const service = new LeaderboardService(repo, cache, logger, config.cacheTtlSeconds);
  const consumer = new EventConsumer(config.rabbitmqUrl, config.exchange, service, logger);

  const ok = async (check: () => Promise<void>): Promise<DepStatus> => {
    try {
      await check();
      return "ok";
    } catch {
      return "down";
    }
  };

  const app = buildApp({
    service,
    logger,
    health: {
      neo4j: () => ok(() => repo.ping()),
      redis: () => ok(() => cache.ping()),
      rabbitmq: async () => (consumer.status === "connected" ? "ok" : "down"),
    },
  });

  // El esquema se crea en segundo plano y se reintenta: el servicio arranca aunque Neo4j tarde.
  let schemaTimer: NodeJS.Timeout | null = null;
  const ensureSchema = async (): Promise<void> => {
    try {
      await repo.ensureSchema();
      logger.info("restricciones de Neo4j verificadas");
    } catch (err) {
      logger.warn({ err: (err as Error).message }, "Neo4j aún no responde: se reintenta la creación del esquema");
      schemaTimer = setTimeout(() => void ensureSchema(), SCHEMA_RETRY_MS);
    }
  };
  void ensureSchema();
  consumer.start();

  await app.listen({ host: "0.0.0.0", port: config.port });

  let shuttingDown = false;
  const shutdown = async (signal: string): Promise<void> => {
    if (shuttingDown) return;
    shuttingDown = true;
    logger.info({ signal }, "apagando el servicio");
    if (schemaTimer) clearTimeout(schemaTimer);
    await consumer.stop();
    await app.close();
    await cache.close();
    await repo.close();
    process.exit(0);
  };
  process.on("SIGTERM", () => void shutdown("SIGTERM"));
  process.on("SIGINT", () => void shutdown("SIGINT"));
}

main().catch((err) => {
  // eslint-disable-next-line no-console
  console.error("No se pudo iniciar leaderboard-stats-ms:", err);
  process.exit(1);
});
