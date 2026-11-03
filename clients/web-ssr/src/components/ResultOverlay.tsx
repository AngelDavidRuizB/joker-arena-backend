import confetti from 'canvas-confetti';
import { motion, useReducedMotion } from 'framer-motion';
import { useEffect, useRef } from 'react';
import type { MatchFinished } from '../lib/types';
import { NeonButton } from './NeonButton';

interface ResultOverlayProps {
  result: MatchFinished;
  onLobby: () => void;
  onReview: () => void;
}

/** End-of-match dialog: victory (confetti) or defeat. */
export function ResultOverlay({ result, onLobby, onReview }: ResultOverlayProps) {
  const reduce = useReducedMotion();
  const win = result.result === 'WIN';
  const primary = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    primary.current?.focus();
    if (!win || reduce) return;
    const colors = ['#00f0ff', '#ff2bd6', '#8b5cf6', '#39ff88', '#ffb800'];
    const end = Date.now() + 1800;
    let raf = 0;
    const frame = () => {
      confetti({ particleCount: 5, angle: 60, spread: 60, origin: { x: 0, y: 0.8 }, colors });
      confetti({ particleCount: 5, angle: 120, spread: 60, origin: { x: 1, y: 0.8 }, colors });
      if (Date.now() < end) raf = requestAnimationFrame(frame);
    };
    frame();
    return () => cancelAnimationFrame(raf);
  }, [win, reduce]);

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink-950/85 p-4 backdrop-blur-md"
    >
      <motion.div
        role="dialog"
        aria-modal="true"
        aria-labelledby="result-title"
        initial={reduce ? false : { scale: 0.7, y: 40 }}
        animate={{ scale: 1, y: 0 }}
        transition={{ type: 'spring', stiffness: 200, damping: 16 }}
        className={`${win ? 'glass-neon-cyan' : 'glass-neon-magenta'} w-full max-w-md space-y-5 p-8 text-center`}
      >
        <h2
          id="result-title"
          className={`font-display text-4xl font-black tracking-widest ${
            win ? 'text-neon-lime text-glow-lime' : 'text-neon-red'
          }`}
        >
          {win ? '¡VICTORIA!' : 'DERROTA'}
        </h2>
        <p className="text-lg text-slate-300">
          {win
            ? 'Alcanzaste la meta de puntos. ¡Los Jokers están contigo!'
            : 'Te quedaste sin manos antes de llegar a la meta. ¡La revancha te espera!'}
        </p>

        <dl className="grid grid-cols-3 gap-3 text-center">
          <div className="rounded-xl border border-white/10 bg-ink-950/60 p-3">
            <dt className="label-caps">Puntaje</dt>
            <dd className="font-display text-2xl font-black text-neon-amber">{result.final_score.toLocaleString('es-CO')}</dd>
          </div>
          <div className="rounded-xl border border-white/10 bg-ink-950/60 p-3">
            <dt className="label-caps">Meta</dt>
            <dd className="font-display text-2xl font-black text-white">{result.target_score.toLocaleString('es-CO')}</dd>
          </div>
          <div className="rounded-xl border border-white/10 bg-ink-950/60 p-3">
            <dt className="label-caps">Manos</dt>
            <dd className="font-display text-2xl font-black text-neon-cyan">{result.hands_played}</dd>
          </div>
        </dl>

        <div className="flex flex-col gap-3 sm:flex-row">
          <NeonButton ref={primary} block variant="cyan" onClick={onLobby}>
            Nueva partida
          </NeonButton>
          <NeonButton block variant="ghost" onClick={onReview}>
            Revisar mesa
          </NeonButton>
        </div>
      </motion.div>
    </motion.div>
  );
}
