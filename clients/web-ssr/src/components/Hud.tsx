import { motion } from 'framer-motion';
import { Layers, Target, Trophy } from 'lucide-react';
import type { ReactNode } from 'react';
import { DEFAULT_TOTAL_DISCARDS, DEFAULT_TOTAL_HANDS } from '../lib/constants';
import type { RoomState } from '../lib/types';
import { AnimatedNumber } from './AnimatedNumber';

interface PipsProps {
  total: number;
  left: number;
  tone: 'cyan' | 'magenta';
  label: string;
}

function Pips({ total, left, tone, label }: PipsProps) {
  const on = tone === 'cyan' ? 'bg-neon-cyan shadow-neon-cyan' : 'bg-neon-magenta shadow-neon-magenta';
  const aria = `${label}: ${left} de ${total}`;
  if (total > 10) {
    const width = total > 0 ? Math.min(100, Math.max(0, (left / total) * 100)) : 0;
    return (
      <div role="img" aria-label={aria} data-testid="pips-bar" className="h-3 w-full rounded-sm bg-white/10">
        <div
          className={`h-full rounded-sm transition-all duration-300 ${on}`}
          style={{ width: `${width}%` }}
        />
      </div>
    );
  }
  return (
    <div role="img" aria-label={aria} className="flex gap-1.5">
      {Array.from({ length: total }, (_, i) => (
        <span
          key={i}
          data-testid="pip"
          className={`h-3 w-6 rounded-sm transition-all duration-300 ${i < left ? on : 'bg-white/10'}`}
        />
      ))}
    </div>
  );
}

function Stat({ icon, label, children }: { icon: ReactNode; label: string; children: ReactNode }) {
  return (
    <div className="glass flex flex-col gap-2 p-4">
      <div className="flex items-center gap-2 text-slate-400">
        {icon}
        <span className="label-caps">{label}</span>
      </div>
      {children}
    </div>
  );
}

const pos = (v: number | undefined, fb: number) => (typeof v === 'number' && Number.isFinite(v) && v > 0 ? v : fb);

/** Scoreboard: progress towards the target, remaining hands/discards and the deck. */
export function Hud({ room }: { room: RoomState }) {
  const pct = Math.min(100, Math.round((room.current_score / room.target_score) * 100));
  const reached = room.current_score >= room.target_score;
  const handsTotal = pos(room.hands_total, DEFAULT_TOTAL_HANDS);
  const discardsTotal = pos(room.discards_total, DEFAULT_TOTAL_DISCARDS);

  return (
    <section aria-label="Marcador de la partida" className="grid gap-4 md:grid-cols-[2fr_1fr_1fr]">
      <Stat icon={<Trophy className="h-4 w-4" />} label="Puntaje / meta">
        <div className="flex items-baseline gap-2">
          <AnimatedNumber
            value={room.current_score}
            className={`font-mono text-4xl font-black ${reached ? 'text-neon-lime' : 'text-neon-amber'}`}
          />
          <span className="font-display text-xl text-slate-500">/ {room.target_score.toLocaleString('es-CO')}</span>
        </div>
        <div
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={room.target_score}
          aria-valuenow={Math.min(room.current_score, room.target_score)}
          aria-label="Progreso hacia la meta"
          className="h-3 overflow-hidden rounded-full bg-white/10"
        >
          <motion.div
            className="h-full rounded-full bg-gradient-to-r from-neon-cyan via-neon-violet to-neon-magenta"
            initial={false}
            animate={{ width: `${pct}%` }}
            transition={{ type: 'spring', stiffness: 80, damping: 18 }}
          />
        </div>
        <span className="text-sm text-slate-400">{pct}% de la meta</span>
      </Stat>

      <Stat icon={<Target className="h-4 w-4" />} label="Manos">
        <Pips total={handsTotal} left={room.hands_left} tone="cyan" label="Manos restantes" />
        <span className="font-display text-2xl font-bold text-neon-cyan">
          {room.hands_left}
          <span className="text-base text-slate-500"> / {handsTotal}</span>
        </span>
      </Stat>

      <Stat icon={<Layers className="h-4 w-4" />} label="Descartes · Baraja">
        <Pips total={discardsTotal} left={room.discards_left} tone="magenta" label="Descartes restantes" />
        <div className="flex items-baseline justify-between">
          <span className="font-display text-2xl font-bold text-neon-magenta">
            {room.discards_left}
            <span className="text-base text-slate-500"> / {discardsTotal}</span>
          </span>
          <span className="font-mono text-sm text-slate-400">{room.deck_count} en mazo</span>
        </div>
      </Stat>
    </section>
  );
}
