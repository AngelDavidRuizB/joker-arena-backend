import Fastify, { type FastifyBaseLogger, type FastifyInstance } from "fastify";
import { JOKER_IDS, type JokerId, ValidationError } from "./ruleset";
import { LiveopsService, VersionConflictError } from "./service";

export type DepStatus = "ok" | "down";
export interface HealthChecks {
  postgres: () => Promise<DepStatus>;
  redis: () => Promise<DepStatus>;
  rabbitmq: () => Promise<DepStatus>;
}

const err = (error: string, message: string, extra: Record<string, unknown> = {}) => ({ error, message, ...extra });

/** Quien hace el cambio: alias del JWT que el gateway validó (o "admin" si no viene). */
function actor(authorization: string | undefined): string {
  try {
    const payload = (authorization ?? "").replace(/^Bearer\s+/i, "").split(".")[1];
    if (!payload) return "admin";
    const claims = JSON.parse(Buffer.from(payload, "base64url").toString("utf8")) as { alias?: string };
    return claims.alias ?? "admin";
  } catch {
    return "admin";
  }
}

export function buildApp(service: LiveopsService, health: HealthChecks, logger?: FastifyBaseLogger): FastifyInstance {
  const app: FastifyInstance = logger ? Fastify({ loggerInstance: logger }) : Fastify({ logger: false });

  app.setErrorHandler((e, req, reply) => {
    if (e instanceof ValidationError) return reply.code(400).send(err("invalid_ruleset", e.message, { details: e.details }));
    if (e instanceof VersionConflictError) {
      return reply.code(409).send(err("version_conflict", `Otro administrador guardó primero. ${e.message}: recarga y vuelve a intentar.`, { current_version: e.current }));
    }
    if ((e as { statusCode?: number }).statusCode === 400) return reply.code(400).send(err("invalid_request", e.message));
    req.log.error({ err: e }, "error");
    return reply.code(503).send(err("store_unavailable", "El almacén de reglas no está disponible."));
  });
  app.setNotFoundHandler((req, reply) => reply.code(404).send(err("not_found", `Ruta no encontrada: ${req.method} ${req.url}`)));

  app.get("/health", { logLevel: "warn" }, async () => {
    const [postgres, redis, rabbitmq] = await Promise.all([health.postgres(), health.redis(), health.rabbitmq()]);
    const deps = { postgres, redis, rabbitmq };
    return { status: Object.values(deps).every((s) => s === "ok") ? "ok" : "degraded", service: "liveops-rules-ms", deps };
  });

  app.get("/liveops/ruleset", async () => service.current());

  app.put<{ Body: Record<string, unknown> }>("/liveops/ruleset", async (req) => {
    const body = req.body ?? {};
    const res = await service.save(body.expected_version, body, actor(req.headers.authorization));
    return { ...res.ruleset, published: res.published };
  });

  app.post<{ Params: { id: string } }>("/liveops/jokers/:id/toggle", async (req, reply) => {
    if (!JOKER_IDS.includes(req.params.id as JokerId)) {
      return reply.code(404).send(err("joker_not_found", `Joker desconocido. Use: ${JOKER_IDS.join(", ")}.`));
    }
    const res = await service.toggleJoker(req.params.id as JokerId, actor(req.headers.authorization));
    return { ...res.ruleset, published: res.published };
  });

  app.post("/liveops/happy-hour", async (req) => {
    const res = await service.setHappyHour(req.body, actor(req.headers.authorization));
    return { ...res.ruleset, published: res.published };
  });

  app.get<{ Querystring: { limit?: string } }>("/liveops/ruleset/history", async (req) => {
    const limit = Math.min(Math.max(Number(req.query.limit ?? 20) || 20, 1), 100);
    return { versions: await service.history(limit) };
  });

  return app;
}
