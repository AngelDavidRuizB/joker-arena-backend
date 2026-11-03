/** Contrato del ruleset (contracts/ruleset.md) y su validación. */

export const JOKER_IDS = ["banner", "classic", "duo"] as const;
export type JokerId = (typeof JOKER_IDS)[number];

export interface Ruleset {
  version: number;
  updated_at: string;
  updated_by: string;
  jokers: {
    banner: { enabled: boolean; chips_per_discard: number };
    classic: { enabled: boolean; mult_bonus: number };
    duo: { enabled: boolean; mult_factor: number };
  };
  happy_hour: { enabled: boolean; global_mult: number };
}

/** Lo que el administrador puede cambiar (sin version/updated_*). */
export type RulesetValues = Pick<Ruleset, "jokers" | "happy_hour">;

/** Rangos válidos de cada valor ajustable. */
export const RANGES = {
  "jokers.banner.chips_per_discard": [0, 200],
  "jokers.classic.mult_bonus": [0, 50],
  "jokers.duo.mult_factor": [1, 10],
  "happy_hour.global_mult": [1, 5],
} as const;

export function defaultValues(): RulesetValues {
  return {
    jokers: {
      banner: { enabled: true, chips_per_discard: 30 },
      classic: { enabled: true, mult_bonus: 4 },
      duo: { enabled: true, mult_factor: 2 },
    },
    happy_hour: { enabled: false, global_mult: 1 },
  };
}

export class ValidationError extends Error {
  constructor(public readonly details: string[]) {
    super(details.join("; "));
    this.name = "ValidationError";
  }
}

type Obj = Record<string, unknown>;
const isObj = (v: unknown): v is Obj => typeof v === "object" && v !== null && !Array.isArray(v);

/** Valida un cuerpo completo { jokers, happy_hour } y devuelve los valores normalizados. */
export function validateValues(body: unknown): RulesetValues {
  const errors: string[] = [];
  const root = isObj(body) ? body : {};
  const jokers = isObj(root.jokers) ? root.jokers : {};
  const section = (o: Obj, key: string): Obj => {
    if (!isObj(o[key])) errors.push(`falta "${key}"`);
    return isObj(o[key]) ? (o[key] as Obj) : {};
  };
  if (!isObj(root.jokers)) errors.push('falta "jokers"');
  const banner = section(jokers, "banner");
  const classic = section(jokers, "classic");
  const duo = section(jokers, "duo");
  const hh = section(root, "happy_hour");

  const bool = (o: Obj, k: string, path: string): boolean => {
    if (typeof o[k] !== "boolean") errors.push(`${path}.${k} debe ser booleano`);
    return o[k] === true;
  };
  const int = (o: Obj, k: string, path: keyof typeof RANGES): number => {
    const v = o[k];
    const [lo, hi] = RANGES[path];
    if (typeof v !== "number" || !Number.isInteger(v) || v < lo || v > hi) {
      errors.push(`${path} debe ser un entero entre ${lo} y ${hi}`);
      return lo;
    }
    return v;
  };
  const values: RulesetValues = {
    jokers: {
      banner: { enabled: bool(banner, "enabled", "jokers.banner"), chips_per_discard: int(banner, "chips_per_discard", "jokers.banner.chips_per_discard") },
      classic: { enabled: bool(classic, "enabled", "jokers.classic"), mult_bonus: int(classic, "mult_bonus", "jokers.classic.mult_bonus") },
      duo: { enabled: bool(duo, "enabled", "jokers.duo"), mult_factor: int(duo, "mult_factor", "jokers.duo.mult_factor") },
    },
    happy_hour: { enabled: bool(hh, "enabled", "happy_hour"), global_mult: int(hh, "global_mult", "happy_hour.global_mult") },
  };
  if (errors.length > 0) throw new ValidationError(errors);
  return values;
}
