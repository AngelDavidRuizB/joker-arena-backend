import { motion, useReducedMotion } from 'framer-motion';
import { forwardRef } from 'react';
import { cardLabel, isRedSuit, SUIT_SYMBOL } from '../lib/cards';
import type { Card } from '../lib/types';

interface PlayingCardProps {
  card: Card;
  selected: boolean;
  disabled?: boolean;
  /** Position inside the fan, used for rotation and deal delay. */
  index: number;
  total: number;
  onToggle: () => void;
}

/**
 * One interactive neon playing card. Suits: spades/clubs glow cyan, hearts/diamonds magenta.
 * It forwards its ref because AnimatePresence (popLayout) measures exiting children.
 */
export const PlayingCard = forwardRef<HTMLButtonElement, PlayingCardProps>(function PlayingCard(
  { card, selected, disabled = false, index, total, onToggle },
  ref,
) {
  const reduce = useReducedMotion();
  const red = isRedSuit(card.suit);
  const offset = index - (total - 1) / 2;
  const rotate = reduce ? 0 : offset * 3.2;
  const drop = reduce ? 0 : Math.abs(offset) ** 2 * 2.2;
  const symbol = SUIT_SYMBOL[card.suit];
  const tone = red ? 'text-neon-magenta' : 'text-neon-cyan';
  const border = selected
    ? 'border-neon-amber shadow-neon-amber'
    : red
      ? 'border-neon-magenta/50 hover:border-neon-magenta hover:shadow-neon-magenta'
      : 'border-neon-cyan/50 hover:border-neon-cyan hover:shadow-neon-cyan';

  return (
    <motion.button
      ref={ref}
      type="button"
      layout={!reduce}
      aria-pressed={selected}
      aria-label={cardLabel(card)}
      disabled={disabled}
      onClick={onToggle}
      initial={reduce ? false : { opacity: 0, y: -160, rotate: -25, scale: 0.6 }}
      animate={{ opacity: 1, y: selected ? -34 - drop : drop, rotate, scale: selected ? 1.06 : 1 }}
      exit={reduce ? undefined : { opacity: 0, y: -90, scale: 0.8, transition: { duration: 0.25 } }}
      transition={{
        type: 'spring',
        stiffness: 280,
        damping: 22,
        delay: reduce ? 0 : index * 0.05,
      }}
      whileHover={disabled || reduce ? undefined : { y: (selected ? -42 : -14) + drop, scale: 1.05 }}
      whileTap={disabled ? undefined : { scale: 0.97 }}
      className={`group relative mx-[-0.6cqw] aspect-[5/7] w-[clamp(3.6rem,11.5cqw,8rem)] shrink-0 select-none overflow-hidden rounded-xl border-2 bg-gradient-to-b from-ink-700 to-ink-900 p-[0.5em] text-left text-[clamp(0.5rem,1.45cqw,1rem)] outline-none transition-colors focus-visible:ring-4 focus-visible:ring-neon-cyan/60 disabled:cursor-not-allowed ${border}`}
    >
      {/* Holographic sheen that sweeps while selected */}
      <span
        aria-hidden="true"
        className={`holo pointer-events-none absolute inset-0 transition-opacity ${selected ? 'opacity-100 animate-shine' : 'opacity-0 group-hover:opacity-60'}`}
      />
      <span aria-hidden="true" className="relative flex h-full flex-col justify-between">
        <span className={`flex flex-col items-start font-display font-black leading-none ${tone}`}>
          <span className="text-[2.6em]">{card.rank}</span>
          <span className="mt-[0.15em] text-[2.1em]">{symbol}</span>
        </span>
        <span className={`absolute inset-0 flex items-center justify-center text-[5.6em] opacity-90 ${tone}`}>
          {symbol}
        </span>
        <span className="flex items-end justify-between">
          <span className="rounded-md bg-black/50 px-[0.4em] py-[0.15em] font-mono text-[1em] font-bold text-neon-amber">
            +{card.chips}
          </span>
          <span className={`rotate-180 font-display text-[2.6em] font-black leading-none ${tone}`}>
            {card.rank}
          </span>
        </span>
      </span>
    </motion.button>
  );
});
