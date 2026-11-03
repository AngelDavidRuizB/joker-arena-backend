import { describe, expect, it } from 'vitest';
import { buildCascade, formulaText } from '../lib/cascade';
import { hand } from './fixtures';

describe('scoring cascade', () => {
  it('reproduces the specification example: Full House with 0 discards = 78 x 16 = 1248', () => {
    const steps = buildCascade(hand());
    expect(steps.map((s) => s.kind)).toEqual(['hand', 'cards', 'banner', 'joker', 'duo', 'total']);

    expect(steps[0]).toMatchObject({ title: 'Full House', chips: 40, mult: 4 });
    expect(steps[1]).toMatchObject({ chips: 78, mult: 4 }); // 40 + 38
    expect(steps[2]).toMatchObject({ chips: 78, mult: 4, jokerId: 'banner', triggered: false });
    expect(steps[3]).toMatchObject({ chips: 78, mult: 8, jokerId: 'joker', triggered: true });
    expect(steps[4]).toMatchObject({ chips: 78, mult: 16, jokerId: 'the_duo', triggered: true });
    expect(steps[5]).toMatchObject({ chips: 78, mult: 16, score: 1248 });
  });

  it('applies the Banner with remaining discards', () => {
    const h = hand({
      discards_left: 2,
      final_chips: 138,
      score: 2208,
      jokers: [
        { id: 'banner', name: 'Banner', effect: '', triggered: true, chips_added: 60, mult_added: 0, mult_factor: 1 },
        ...hand().jokers.slice(1),
      ],
    });
    const steps = buildCascade(h);
    expect(steps[2]).toMatchObject({ chips: 138, triggered: true });
    expect(steps[2].detail).toContain('2 descartes');
    expect(steps.at(-1)).toMatchObject({ chips: 138, mult: 16, score: 2208 });
  });

  it('leaves the multiplier untouched when The Duo does not trigger', () => {
    const h = hand({
      poker_hand: 'STRAIGHT',
      base_mult: 4,
      final_mult: 8,
      final_chips: 100,
      score: 800,
      jokers: [
        hand().jokers[0],
        hand().jokers[1],
        { id: 'the_duo', name: 'The Duo', effect: '', triggered: false, chips_added: 0, mult_added: 0, mult_factor: 1 },
      ],
    });
    const steps = buildCascade(h);
    expect(steps[4]).toMatchObject({ mult: 8, triggered: false });
    expect(steps[4].detail).toMatch(/× 1/);
  });

  it('always ends with the server-reported final chips and mult', () => {
    const h = hand();
    const last = buildCascade(h).at(-1)!;
    expect(last.chips).toBe(h.final_chips);
    expect(last.mult).toBe(h.final_mult);
    expect(last.chips * last.mult).toBe(h.score);
  });

  it('renders the audit formula', () => {
    expect(formulaText(hand())).toBe('(40 + 38 + 0) × ((4 + 4) × 2) = 78 × 16 = 1248');
  });
});
