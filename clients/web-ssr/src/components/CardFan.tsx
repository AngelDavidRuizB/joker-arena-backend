import { AnimatePresence } from 'framer-motion';
import type { Card } from '../lib/types';
import { PlayingCard } from './PlayingCard';

interface CardFanProps {
  cards: Card[];
  selected: number[];
  disabled: boolean;
  onToggle: (index: number) => void;
  /**
   * Opponent (bot) view: cards are shown but never interactive. The selected indices keep the
   * raised/amber 'selected' visual so the player sees which cards the bot chose.
   */
  readOnly?: boolean;
}

/** The player's hand laid out as a fan; each card is keyed by identity so refills animate only new cards. */
export function CardFan({ cards, selected, disabled, onToggle, readOnly = false }: CardFanProps) {
  const noop = () => {};
  return (
    <div
      role="group"
      aria-label={readOnly ? 'Mano del bot' : 'Tu mano de cartas'}
      className="flex min-h-[13rem] flex-wrap items-end justify-center gap-y-3 px-2 pb-4 pt-12 [container-type:inline-size] sm:min-h-[16rem]"
    >
      <AnimatePresence mode="popLayout" initial>
        {cards.map((card, i) => (
          <PlayingCard
            key={`${card.rank}-${card.suit}`}
            card={card}
            index={i}
            total={cards.length}
            selected={selected.includes(i)}
            disabled={readOnly || disabled}
            onToggle={readOnly ? noop : () => onToggle(i)}
          />
        ))}
      </AnimatePresence>
    </div>
  );
}
