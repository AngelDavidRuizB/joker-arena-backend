import Fastify, { type FastifyBaseLogger, type FastifyInstance } from "fastify";
import { MAX_TOP_LIMIT, type LeaderboardService } from "../service";

export const SERVICE_NAME = "leaderboard-stats-ms";
const DEFAULT_TOP_LIMIT = 10;

export type DepStatus = "ok" | "down";

/** Cada función devuelve el estado de una dependencia para /health. */
export interface HealthChecks {
  neo4j: () => Promise<DepStatus>;
  redis: () => Promise<DepStatus>;
  rabbitmq: () => Promise<DepStatus>;
}

export interface AppDeps {
  service: LeaderboardService;
  health: HealthChecks;
  /** Si no se pasa, la app no registra logs (útil en tests). */
  logger?: FastifyBaseLogger;
}

function errorBody(error: string, message: string) {
  return { error, message };
}

/** Un error de conexión a Neo4j se traduce en 503 en vez de 500. */
function isStoreUnavailable(err: unknown): boolean {
  const e = err as { code?: string; name?: string } | null;
  return e?.code === "ServiceUnavailable" || e?.code === "SessionExpired";
}

export function buildApp(deps: AppDeps): FastifyInstance {
  const app: FastifyInstance = deps.logger
    ? Fastify({ loggerInstance: deps.logger })
    : Fastify({ logger: false });

  app.setErrorHandler((err, req, reply) => {
    if (isStoreUnavailable(err)) {
      req.log.warn({ err: err.message }, "Neo4j no disponible");
      return reply.code(503).send(errorBody("store_unavailable", "El almacén del ranking no está disponible. Intente más tarde."));
    }
    if (err.validation) {
      return reply.code(400).send(errorBody("invalid_request", err.message));
    }
    req.log.error({ err }, "error no controlado");
    return reply.code(500).send(errorBody("internal_error", "Error interno del servicio."));
  });

  app.setNotFoundHandler((req, reply) =>
    reply.code(404).send(errorBody("not_found", `Ruta no encontrada: ${req.method} ${req.url}`)),
  );

  // logLevel "warn": el healthcheck de Docker llama cada 10 s y no debe llenar los logs.
  app.get("/health", { logLevel: "warn" }, async () => {
    const [neo4j, redis, rabbitmq] = await Promise.all([
      deps.health.neo4j().catch((): DepStatus => "down"),
      deps.health.redis().catch((): DepStatus => "down"),
      deps.health.rabbitmq().catch((): DepStatus => "down"),
    ]);
    const all = { neo4j, redis, rabbitmq };
    const status = Object.values(all).every((s) => s === "ok") ? "ok" : "degraded";
    return { status, service: SERVICE_NAME, deps: all };
  });

  app.get<{ Querystring: { limit?: string } }>("/leaderboard/top", async (req, reply) => {
    const rawLimit = req.query.limit;
    const limit = rawLimit === undefined ? DEFAULT_TOP_LIMIT : Number(rawLimit);
    if (!Number.isInteger(limit) || limit < 1 || limit > MAX_TOP_LIMIT) {
      return reply
        .code(400)
        .send(errorBody("invalid_limit", `"limit" debe ser un entero entre 1 y ${MAX_TOP_LIMIT}.`));
    }
    const top = await deps.service.getTop(limit);
    return { generated_at: new Date().toISOString(), cached: top.cached, players: top.players };
  });

  app.get<{ Params: { player_id: string } }>("/leaderboard/players/:player_id", async (req, reply) => {
    const stats = await deps.service.getPlayer(req.params.player_id);
    if (!stats) {
      return reply.code(404).send(errorBody("player_not_found", `No hay datos del jugador ${req.params.player_id}.`));
    }
    return stats;
  });

  return app;
}
