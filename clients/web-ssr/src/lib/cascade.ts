import { handName } from './cards';
import type { HandScored } from './types';

export type CascadeStepKind = 'hand' | 'cards' | 'banner' | 'joker' | 'duo' | 'total';

export interface CascadeStep {
  key: string;
  kind: CascadeStepKind;
  title: string;
  detail: string;
  /** Running totals after applying this step. */
  chips: number;
  mult: number;
  jokerId?: string;
  triggered: boolean;
  /** Only the last step carries the final score. */
  score?: number;
}

/**
 * Turns the server's HAND_SCORED audit into the ordered animation steps of the scoring
 * cascade: hand base -> card chips -> Banner -> Joker Clásico -> The Duo -> Chips x Mult.
 * It only presents numbers computed by the server; the last step always equals `score`.
 */
export function buildCascade(h: HandScored): CascadeStep[] {
  const joker = (id: string) => h.jokers.find((j) => j.id === id);
  const banner = joker('banner');
  const classic = joker('joker');
  const duo = joker('the_duo');
  const scoredCards = h.cards_played.filter((c) => c.scored).length;

  let chips = h.base_chips;
  let mult = h.base_mult;
  const steps: CascadeStep[] = [];

  steps.push({
    key: 'hand',
    kind: 'hand',
    title: handName(h.poker_hand),
    detail: `Base ${h.base_chips} fichas × ${h.base_mult} Mult`,
    chips,
    mult,
    triggered: true,
  });

  chips += h.card_chips;
  steps.push({
    key: 'cards',
    kind: 'cards',
    title: 'Cartas que puntúan',
    detail: `${scoredCards} ${scoredCards === 1 ? 'carta suma' : 'cartas suman'} +${h.card_chips} fichas`,
    chips,
    mult,
    triggered: true,
  });

  const bannerChips = banner?.chips_added ?? 0;
  chips += bannerChips;
  steps.push({
    key: 'banner',
    kind: 'banner',
    title: 'Banner',
    detail: banner?.triggered
      ? `+30 fichas × ${h.discards_left} descartes restantes = +${bannerChips}`
      : 'Sin descartes restantes: +0 fichas',
    chips,
    mult,
    jokerId: 'banner',
    triggered: !!banner?.triggered,
  });

  const classicMult = classic?.mult_added ?? 0;
  mult += classicMult;
  steps.push({
    key: 'joker',
    kind: 'joker',
    title: 'Joker Clásico',
    detail: `+${classicMult} Mult`,
    chips,
    mult,
    jokerId: 'joker',
    triggered: !!classic?.triggered,
  });

  const factor = duo?.triggered ? duo.mult_factor : 1;
  mult = Math.round(mult * factor);
  steps.push({
    key: 'duo',
    kind: 'duo',
    title: 'The Duo',
    detail: duo?.triggered ? `La mano contiene una pareja: Mult × ${factor}` : 'Sin pareja: Mult × 1',
    chips,
    mult,
    jokerId: 'the_duo',
    triggered: !!duo?.triggered,
  });

  steps.push({
    key: 'total',
    kind: 'total',
    title: 'Puntaje de la mano',
    detail: `${chips} × ${mult}`,
    chips,
    mult,
    triggered: true,
    score: h.score,
  });

  return steps;
}

/** The human-readable formula shown in the audit panel. */
export function formulaText(h: HandScored): string {
  const bannerChips = h.jokers.find((j) => j.id === 'banner')?.chips_added ?? 0;
  const classic = h.jokers.find((j) => j.id === 'joker')?.mult_added ?? 0;
  const duo = h.jokers.find((j) => j.id === 'the_duo');
  const factor = duo?.triggered ? duo.mult_factor : 1;
  return `(${h.base_chips} + ${h.card_chips} + ${bannerChips}) × ((${h.base_mult} + ${classic}) × ${factor}) = ${h.final_chips} × ${h.final_mult} = ${h.score}`;
}
