// All communication goes through the API Gateway — never directly to microservices.
// Change GATEWAY_URL in .env.local for different environments.

export const GATEWAY_URL =
  (import.meta as unknown as { env: Record<string, string> }).env?.VITE_GATEWAY_URL ??
  "http://localhost:8080";

export const WS_METRICS_URL = GATEWAY_URL.replace(/^http/, "ws") + "/admin/cluster/metrics";
export const LOGIN_URL = `${GATEWAY_URL}/api/auth/login`;
export const LIVEOPS_URL = `${GATEWAY_URL}/admin/liveops/ruleset`;
