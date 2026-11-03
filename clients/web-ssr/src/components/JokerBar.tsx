import { motion } from 'framer-motion';
import { JOKERS } from '../lib/cards';
import type { JokerMeta } from '../lib/cards';

const TONE: Record<JokerMeta['color'], { border: string; text: string; glow: string }> = {
  cyan: { border: 'border-neon-cyan/50', text: 'text-neon-cyan', glow: 'shadow-neon-cyan' },
  magenta: { border: 'border-neon-magenta/50', text: 'text-neon-magenta', glow: 'shadow-neon-magenta' },
  violet: { border: 'border-neon-violet/60', text: 'text-neon-violet', glow: 'shadow-neon-violet' },
};

interface JokerBarProps {
  /** Joker currently being applied by the scoring cascade, if any. */
  activeId?: string | null;
  /** Ids of the jokers the match is using (server value). */
  owned: string[];
}

/** The three canonical Jokers, evaluated strictly in this order. The active one pulses. */
export function JokerBar({ activeId, owned }: JokerBarProps) {
  return (
    <section aria-label="Jokers activos" className="grid gap-3 sm:grid-cols-3">
      {JOKERS.filter((j) => owned.includes(j.id)).map((j, i) => {
        const tone = TONE[j.color];
        const active = activeId === j.id;
        return (
          <motion.div
            key={j.id}
            animate={active ? { scale: 1.06, y: -4 } : { scale: 1, y: 0 }}
            transition={{ type: 'spring', stiffness: 300, damping: 18 }}
            className={`glass relative flex items-center gap-3 overflow-hidden border p-3 ${tone.border} ${
              active ? tone.glow : ''
            }`}
          >
            <span className="absolute right-2 top-1 font-mono text-[0.6rem] text-slate-500" aria-hidden="true">
              #{i + 1}
            </span>
            <span
              className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-lg border bg-ink-950/70 text-2xl ${tone.border} ${tone.text} ${
                active ? 'animate-pulse' : ''
              }`}
              aria-hidden="true"
            >
              {j.glyph}
            </span>
            <div className="min-w-0">
              <h3 className={`font-display text-xs font-bold uppercase tracking-widest ${tone.text}`}>{j.name}</h3>
              <p className="text-sm leading-tight text-slate-300">{j.description}</p>
            </div>
          </motion.div>
        );
      })}
    </section>
  );
}
