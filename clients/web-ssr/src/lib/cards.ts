import type { Card, Suit } from './types';

export const SUIT_SYMBOL: Record<Suit, string> = {
  SPADES: '♠',
  HEARTS: '♥',
  CLUBS: '♣',
  DIAMONDS: '♦',
};

export const SUIT_NAME: Record<Suit, string> = {
  SPADES: 'picas',
  HEARTS: 'corazones',
  CLUBS: 'tréboles',
  DIAMONDS: 'diamantes',
};

const RANK_NAME: Record<string, string> = {
  A: 'As',
  K: 'Rey',
  Q: 'Reina',
  J: 'Jota',
};

export const isRedSuit = (suit: Suit): boolean => suit === 'HEARTS' || suit === 'DIAMONDS';

export function cardLabel(card: Pick<Card, 'rank' | 'suit' | 'chips'>): string {
  const rank = RANK_NAME[card.rank] ?? card.rank;
  return `${rank} de ${SUIT_NAME[card.suit]}, ${card.chips} fichas`;
}

/** Spanish names of the nine poker hands (server sends the enum in English caps). */
export const HAND_NAME: Record<string, string> = {
  HIGH_CARD: 'Carta Alta',
  PAIR: 'Pareja',
  TWO_PAIR: 'Doble Pareja',
  THREE_OF_A_KIND: 'Trío',
  STRAIGHT: 'Escalera',
  FLUSH: 'Color',
  FULL_HOUSE: 'Full House',
  FOUR_OF_A_KIND: 'Póker',
  STRAIGHT_FLUSH: 'Escalera de Color',
};

export const handName = (hand: string): string => HAND_NAME[hand] ?? hand;

export interface JokerMeta {
  id: string;
  name: string;
  description: string;
  kind: string;
  color: 'cyan' | 'magenta' | 'violet';
  glyph: string;
}

/** Static presentation data for the three canonical Jokers. */
export const JOKERS: JokerMeta[] = [
  {
    id: 'banner',
    name: 'Banner',
    description: '+30 fichas por cada descarte restante',
    kind: 'Aditivo de fichas',
    color: 'cyan',
    glyph: '⚑',
  },
  {
    id: 'joker',
    name: 'Joker Clásico',
    description: '+4 Mult en toda mano jugada',
    kind: 'Aditivo de Mult',
    color: 'magenta',
    glyph: '☺',
  },
  {
    id: 'the_duo',
    name: 'The Duo',
    description: '×2 Mult si la mano contiene una pareja',
    kind: 'Multiplicador ×',
    color: 'violet',
    glyph: '✦',
  },
];
