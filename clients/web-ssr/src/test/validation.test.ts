import { describe, expect, it } from 'vitest';
import { normalizeRoomCode, validateAlias, validateRoomCode } from '../lib/validation';

describe('validateAlias', () => {
  it('accepts valid aliases', () => {
    for (const a of ['JokerPro99', 'ab', 'a_b-c', 'x'.repeat(30), '  padded  ']) {
      expect(validateAlias(a), a).toBeNull();
    }
  });

  it('rejects empty, short, long and invalid characters', () => {
    expect(validateAlias('')).toMatch(/alias/i);
    expect(validateAlias('   ')).toMatch(/alias/i);
    expect(validateAlias('a')).toMatch(/al menos 2/);
    expect(validateAlias('x'.repeat(31))).toMatch(/máximo 30/);
    for (const a of ['bad alias', '<script>', 'ñandú', 'a@b', 'x;drop']) {
      expect(validateAlias(a), a).not.toBeNull();
    }
  });

  it('reserves the bot alias', () => {
    expect(validateAlias('Bot-JokerBot')).toMatch(/reservado/);
    expect(validateAlias('bot-jokerbot')).toMatch(/reservado/);
  });
});

describe('room codes', () => {
  it('normalizes to upper case without spaces', () => {
    expect(normalizeRoomCode('  jkr-a7k2 ')).toBe('JKR-A7K2');
  });

  it('accepts the JKR-XXXX format and the legacy JKR-402', () => {
    expect(validateRoomCode('jkr-a7k2')).toBeNull();
    expect(validateRoomCode('JKR-402')).toBeNull();
  });

  it('rejects empty and malformed codes', () => {
    expect(validateRoomCode('')).not.toBeNull();
    for (const c of ['x', 'JK', '-ABC', 'JKR_A7K2', 'JKR A7K2', 'JKR-ABCDEFGHIJKL']) {
      expect(validateRoomCode(c), c).not.toBeNull();
    }
  });
});
