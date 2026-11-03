/**
 * Configuración del servicio, leída solo de variables de entorno
 * (nombres definidos en la sección 0.4 de la guía del equipo).
 */
export interface Config {
  port: number;
  neo4jUrl: string;
  neo4jUser: string;
  neo4jPassword: string;
  redisUrl: string;
  rabbitmqUrl: string;
  exchange: string;
  cacheTtlSeconds: number;
  logLevel: string;
}

function int(value: string | undefined, fallback: number, name: string): number {
  if (value === undefined || value === "") return fallback;
  const n = Number(value);
  if (!Number.isInteger(n) || n <= 0) {
    throw new Error(`La variable ${name} debe ser un entero positivo (valor recibido: "${value}")`);
  }
  return n;
}

export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  return {
    port: int(env.PORT, 8086, "PORT"),
    neo4jUrl: env.NEO4J_URL ?? "bolt://jokerarena-neo4j:7687",
    neo4jUser: env.NEO4J_USER ?? "neo4j",
    neo4jPassword: env.NEO4J_PASSWORD ?? "",
    redisUrl: env.REDIS_URL ?? "redis://jokerarena-redis:6379/0",
    rabbitmqUrl: env.RABBITMQ_URL ?? "amqp://guest:guest@jokerarena-rabbitmq:5672/",
    exchange: env.RABBITMQ_EXCHANGE ?? "jokerarena.events",
    cacheTtlSeconds: int(env.LEADERBOARD_CACHE_TTL_SECONDS, 30, "LEADERBOARD_CACHE_TTL_SECONDS"),
    logLevel: env.LOG_LEVEL ?? "info",
  };
}
