/** Utilidades de prueba: llaves RSA propias, JWKS en memoria y firma de tokens RS256 como los de auth. */
import { generateKeyPairSync, sign, type KeyObject } from "node:crypto";
import { createAdminGuard, ISSUER, JwksClient } from "../src/auth";
import { silentLogger } from "../src/logger";

export const KID = "test-kid";
export const authKeys = generateKeyPairSync("rsa", { modulusLength: 2048 });
export const attackerKeys = generateKeyPairSync("rsa", { modulusLength: 2048 });

export const jwksBody = () => ({ keys: [{ ...authKeys.publicKey.export({ format: "jwk" }), kid: KID, use: "sig", alg: "RS256" }] });

/** fetch falso de /.well-known/jwks.json; falla las primeras `failTimes` veces. */
export function fakeJwksFetch(failTimes = 0) {
  let calls = 0;
  const fn = async () => {
    calls++;
    if (calls <= failTimes) throw new Error("ECONNREFUSED");
    return { ok: true, status: 200, json: async () => jwksBody() };
  };
  return Object.assign(fn, { calls: () => calls });
}

export const testJwks = (fetchImpl = fakeJwksFetch(), maxAttempts = 8) =>
  new JwksClient({ logger: silentLogger, fetchImpl, maxAttempts, sleep: async () => undefined });

const b64 = (o: object) => Buffer.from(JSON.stringify(o)).toString("base64url");
const nowS = () => Math.floor(Date.now() / 1000);

export function signToken(
  claims: Record<string, unknown>,
  { key = authKeys.privateKey as KeyObject, kid = KID, alg = "RS256" }: { key?: KeyObject; kid?: string; alg?: string } = {},
): string {
  const head = b64({ alg, typ: "JWT", kid });
  const body = b64(claims);
  const sig = alg === "none" ? "" : sign("sha256", Buffer.from(`${head}.${body}`), key).toString("base64url");
  return `${head}.${body}.${sig}`;
}

/** Claims como los que emite auth-identity-ms para el admin. */
export const adminClaims = (extra: Record<string, unknown> = {}) => ({
  iss: ISSUER, sub: "11111111-1111-1111-1111-111111111111", alias: "admin", role: "admin",
  iat: nowS(), exp: nowS() + 1800, jti: "jti-1", ...extra,
});

export const bearer = (token: string) => ({ authorization: `Bearer ${token}` });
export const adminHeaders = (extra: Record<string, unknown> = {}) => bearer(signToken(adminClaims(extra)));

export const testGuard = (isRevoked?: (jti: string) => Promise<boolean>) => createAdminGuard({ keys: testJwks(), isRevoked });
