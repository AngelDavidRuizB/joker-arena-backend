import { AnimatePresence, motion, useReducedMotion } from 'framer-motion';
import { Bot, Crown } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { buildCascade, type CascadeStep } from '../lib/cascade';
import { SUIT_SYMBOL, isRedSuit } from '../lib/cards';
import { BOT_PLAYER_ID } from '../lib/constants';
import type { HandScored } from '../lib/types';
import { AnimatedNumber } from './AnimatedNumber';

interface ScoreCascadeProps {
  hand: HandScored;
  /** Label of the hand's author (the host alias); the bot is always labelled 'Bot'. */
  authorLabel?: string;
  /** Called with each step so the parent can highlight the matching Joker. */
  onStep?: (step: CascadeStep) => void;
  onDone: () => void;
}

const STEP_MS = 800;
const STEP_MS_REDUCED = 250;
const HOLD_MS = 1500;

/** Overlay that reveals the server's scoring maths step by step: Chips x Mult. */
export function ScoreCascade({ hand, authorLabel, onStep, onDone }: ScoreCascadeProps) {
  const reduce = useReducedMotion();
  const steps = useMemo(() => buildCascade(hand), [hand]);
  const [index, setIndex] = useState(0);
  const step = steps[index];
  const last = index === steps.length - 1;
  const isBot = hand.player_id === BOT_PLAYER_ID;
  const author = isBot ? 'Bot' : authorLabel ?? 'Anfitrión';

  useEffect(() => {
    onStep?.(step);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [index]);

  useEffect(() => {
    const delay = last ? HOLD_MS : reduce ? STEP_MS_REDUCED : STEP_MS;
    const t = setTimeout(() => (last ? onDone() : setIndex((i) => i + 1)), delay);
    return () => clearTimeout(t);
  }, [index, last, reduce, onDone]);

  return (
    <motion.div
      role="dialog"
      aria-modal="false"
      aria-label={`Puntuación de la mano ${hand.hand_number} de ${author}`}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="absolute inset-0 z-30 flex flex-col items-center justify-center gap-5 rounded-[2rem] bg-ink-950/85 p-4 backdrop-blur-md"
    >
      <div
        data-testid="cascade-author"
        className={`absolute left-4 top-4 flex items-center gap-1.5 rounded-lg border px-3 py-1 font-display text-[0.65rem] font-bold uppercase tracking-widest ${
          isBot ? 'border-neon-magenta/60 bg-neon-magenta/10 text-neon-magenta' : 'border-neon-amber/60 bg-neon-amber/10 text-neon-amber'
        }`}
      >
        {isBot ? <Bot className="h-3.5 w-3.5" aria-hidden="true" /> : <Crown className="h-3.5 w-3.5" aria-hidden="true" />}
        {author}
      </div>

      <button
        type="button"
        onClick={onDone}
        className="absolute right-4 top-4 rounded-lg border border-white/15 px-3 py-1 font-display text-[0.65rem] uppercase tracking-widest text-slate-300 hover:text-neon-cyan focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neon-cyan"
      >
        Saltar
      </button>

      <div className="flex flex-wrap justify-center gap-2" aria-hidden="true">
        {hand.cards_played.map((c, i) => (
          <motion.div
            key={`${c.rank}${c.suit}${i}`}
            initial={reduce ? false : { y: 30, opacity: 0 }}
            animate={{ y: 0, opacity: c.scored ? 1 : 0.35, scale: c.scored && index >= 1 ? 1.08 : 1 }}
            transition={{ delay: reduce ? 0 : i * 0.06 }}
            className={`flex h-20 w-14 flex-col items-center justify-center rounded-lg border-2 bg-ink-800 font-display font-black ${
              c.scored ? 'border-neon-amber shadow-neon-amber' : 'border-white/15'
            } ${isRedSuit(c.suit) ? 'text-neon-magenta' : 'text-neon-cyan'}`}
          >
            <span className="text-lg leading-none">{c.rank}</span>
            <span className="text-xl leading-none">{SUIT_SYMBOL[c.suit]}</span>
            <span className="mt-1 font-mono text-[0.6rem] text-neon-amber">{c.scored ? `+${c.chips}` : '—'}</span>
          </motion.div>
        ))}
      </div>

      <AnimatePresence mode="wait">
        <motion.div
          key={step.key}
          initial={reduce ? false : { opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0 }}
          className="text-center"
        >
          <h3
            className={`font-display text-2xl font-black uppercase tracking-widest ${
              step.kind === 'total' ? 'text-neon-lime text-glow-lime' : 'text-white'
            }`}
          >
            {step.title}
          </h3>
          <p className="mt-1 min-h-[1.5rem] text-slate-300">{step.detail}</p>
        </motion.div>
      </AnimatePresence>

      <div className="flex items-center gap-4 sm:gap-6" aria-hidden="true">
        <div className="min-w-[7rem] rounded-xl border border-neon-cyan/60 bg-neon-cyan/10 px-4 py-3 text-center shadow-neon-cyan sm:min-w-[9rem]">
          <div className="label-caps !text-neon-cyan">Fichas</div>
          <AnimatedNumber value={step.chips} className="font-display text-4xl font-black text-neon-cyan sm:text-5xl" />
        </div>
        <span className="font-display text-3xl text-slate-400">×</span>
        <div className="min-w-[7rem] rounded-xl border border-neon-magenta/60 bg-neon-magenta/10 px-4 py-3 text-center shadow-neon-magenta sm:min-w-[9rem]">
          <div className="label-caps !text-neon-magenta">Mult</div>
          <AnimatedNumber value={step.mult} className="font-display text-4xl font-black text-neon-magenta sm:text-5xl" />
        </div>
      </div>

      <div className="h-16" aria-hidden={!last}>
        {last && (
          <motion.div
            initial={reduce ? false : { scale: 0.4, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ type: 'spring', stiffness: 220, damping: 14 }}
            className="text-center"
          >
            <span className="font-display text-sm uppercase tracking-widest text-slate-400">= </span>
            <AnimatedNumber value={hand.score} duration={0.7} className="font-display text-5xl font-black text-neon-amber text-glow-amber" />
            <span className="ml-2 font-display text-sm uppercase tracking-widest text-slate-400">puntos</span>
          </motion.div>
        )}
      </div>

      <p className="sr-only" aria-live="polite">
        {last ? `${step.title}: ${hand.final_chips} fichas por ${hand.final_mult} de multiplicador, ${hand.score} puntos.` : step.detail}
      </p>

      <div className="flex gap-1.5" aria-hidden="true">
        {steps.map((s, i) => (
          <span key={s.key} className={`h-1.5 w-8 rounded-full transition ${i <= index ? 'bg-neon-cyan' : 'bg-white/15'}`} />
        ))}
      </div>
    </motion.div>
  );
}
