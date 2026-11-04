// Client-side validation mirroring the Game Engine rules (the server remains authoritative).

const ALIAS_RE = /^[a-zA-Z0-9_-]+$/;
const ROOM_CODE_RE = /^[A-Z0-9][A-Z0-9-]{2,11}$/;

export const ALIAS_MIN = 2;
export const ALIAS_MAX = 30;

/** Returns an error message in Spanish, or null when the alias is valid. */
export function validateAlias(raw: string): string | null {
  const alias = raw.trim();
  if (alias.length === 0) return 'Escribe un alias para continuar.';
  if (alias.length < ALIAS_MIN) return `El alias necesita al menos ${ALIAS_MIN} caracteres.`;
  if (alias.length > ALIAS_MAX) return `El alias admite máximo ${ALIAS_MAX} caracteres.`;
  if (!ALIAS_RE.test(alias)) return 'Solo letras, números, guion y guion bajo (sin espacios).';
  if (alias.toLowerCase() === 'bot-jokerbot') return 'Ese alias está reservado.';
  return null;
}

export function normalizeRoomCode(raw: string): string {
  return raw.trim().toUpperCase();
}

/** Returns an error message in Spanish, or null when the room code looks valid. */
export function validateRoomCode(raw: string): string | null {
  const code = normalizeRoomCode(raw);
  if (code.length === 0) return 'Escribe el código de la sala (ej. JKR-A7K2).';
  if (!ROOM_CODE_RE.test(code)) return 'Código inválido. Formato esperado: JKR-XXXX.';
  return null;
}
