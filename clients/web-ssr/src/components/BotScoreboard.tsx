import { motion } from 'framer-motion';
import { Bot } from 'lucide-react';
import { DEFAULT_TOTAL_DISCARDS, DEFAULT_TOTAL_HANDS } from '../lib/constants';
import type { RoomState } from '../lib/types';
import { AnimatedNumber } from './AnimatedNumber';

const n = (v: number | undefined, fb = 0) => (typeof v === 'number' && Number.isFinite(v) ? v : fb);

/**
 * Marcador del oponente simulado. Se muestra SOLO en partidas con bot (room.with_bot) y
 * tolera que falten los campos bot_* del ROOM_STATE_UPDATE (respaldo 0), reutilizando el
 * estilo neón del HUD (barra de progreso hacia la meta).
 */
export function BotScoreboard({ room }: { room: RoomState }) {
  if (!room.with_bot) return null;

  const score = n(room.bot_score);
  const handsLeft = n(room.bot_hands_left);
  const discardsLeft = n(room.bot_discards_left);
  const target = room.target_score;
  const pct = target > 0 ? Math.min(100, Math.round((score / target) * 100)) : 0;
  const reached = target > 0 && score >= target;

  return (
    <section aria-labelledby="bot-score-title" className="glass-neon-magenta p-4">
      <div className="mb-3 flex items-center gap-2 border-b border-white/10 pb-3">
        <Bot className="h-4 w-4 text-neon-magenta" aria-hidden="true" />
        <h2 id="bot-score-title" className="font-display text-xs font-bold uppercase tracking-[0.2em] text-slate-200">
          Marcador del bot
        </h2>
      </div>

      <div className="flex items-baseline gap-2">
        <AnimatedNumber
          value={score}
          className={`font-mono text-3xl font-black ${reached ? 'text-neon-lime' : 'text-neon-magenta'}`}
        />
        <span className="font-display text-base text-slate-500">/ {target.toLocaleString('es-CO')}</span>
      </div>

      <div
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={target}
        aria-valuenow={Math.min(score, target)}
        aria-label="Progreso del bot hacia la meta"
        className="mt-2 h-3 overflow-hidden rounded-full bg-white/10"
      >
        <motion.div
          className="h-full rounded-full bg-gradient-to-r from-neon-magenta via-neon-violet to-neon-cyan"
          initial={false}
          animate={{ width: `${pct}%` }}
          transition={{ type: 'spring', stiffness: 80, damping: 18 }}
        />
      </div>
      <span className="mt-1 block text-sm text-slate-400">{pct}% de la meta</span>

      <dl className="mt-3 grid grid-cols-2 gap-3">
        <div>
          <dt className="label-caps">Manos</dt>
          <dd className="font-display text-xl font-bold text-neon-cyan">
            {handsLeft}
            <span className="text-sm text-slate-500"> / {DEFAULT_TOTAL_HANDS}</span>
          </dd>
        </div>
        <div>
          <dt className="label-caps">Descartes</dt>
          <dd className="font-display text-xl font-bold text-neon-magenta">
            {discardsLeft}
            <span className="text-sm text-slate-500"> / {DEFAULT_TOTAL_DISCARDS}</span>
          </dd>
        </div>
      </dl>
    </section>
  );
}
