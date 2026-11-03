import { Activity, Bot, Crown } from 'lucide-react';
import { SUIT_SYMBOL, handName, isRedSuit } from '../lib/cards';
import { formulaText } from '../lib/cascade';
import { BOT_PLAYER_ID } from '../lib/constants';
import type { HandScored } from '../lib/types';

/** Live audit console: the exact algebraic breakdown of every hand played in the match. */
export function AuditPanel({ history }: { history: HandScored[] }) {
  const items = [...history].reverse();
  return (
    <section aria-labelledby="audit-title" className="glass-neon-cyan flex min-h-0 flex-col p-4">
      <div className="mb-3 flex items-center gap-2 border-b border-white/10 pb-3">
        <Activity className="h-4 w-4 text-neon-lime" aria-hidden="true" />
        <h2 id="audit-title" className="font-display text-xs font-bold uppercase tracking-[0.2em] text-slate-200">
          Auditoría en vivo
        </h2>
        <span className="ml-auto font-mono text-xs text-slate-500">{history.length} manos</span>
      </div>

      {items.length === 0 ? (
        <p className="py-6 text-center text-slate-500">
          Juega una mano para ver aquí el desglose exacto de fichas, Jokers y multiplicadores.
        </p>
      ) : (
        <ol className="max-h-[28rem] space-y-3 overflow-y-auto pr-1">
          {items.map((h) => {
            const isBot = h.player_id === BOT_PLAYER_ID;
            return (
            <li key={`${h.player_id}-${h.hand_number}-${h.timestamp}`} className="rounded-xl border border-white/10 bg-ink-950/60 p-3">
              <div className="flex items-center justify-between">
                <span className="font-display text-xs font-bold uppercase tracking-widest text-neon-cyan">
                  #{h.hand_number} · {handName(h.poker_hand)}
                </span>
                <span className="font-display text-lg font-black text-neon-amber">+{h.score.toLocaleString('es-CO')}</span>
              </div>
              <div
                className={`mt-1 inline-flex items-center gap-1 font-display text-[0.65rem] font-bold uppercase tracking-widest ${
                  isBot ? 'text-neon-magenta' : 'text-neon-amber'
                }`}
              >
                {isBot ? <Bot className="h-3 w-3" aria-hidden="true" /> : <Crown className="h-3 w-3" aria-hidden="true" />}
                {isBot ? 'Bot' : 'Anfitrión'}
              </div>
              <div className="mt-2 flex flex-wrap gap-1" aria-hidden="true">
                {h.cards_played.map((c, i) => (
                  <span
                    key={i}
                    className={`rounded border px-1.5 py-0.5 font-mono text-xs ${
                      c.scored ? 'border-neon-amber/60 text-white' : 'border-white/10 text-slate-500'
                    } ${isRedSuit(c.suit) ? 'text-neon-magenta' : ''}`}
                  >
                    {c.rank}
                    {SUIT_SYMBOL[c.suit]}
                  </span>
                ))}
              </div>
              <ul className="mt-2 space-y-0.5 text-sm text-slate-300">
                <li>
                  Base: {h.base_chips} fichas × {h.base_mult} Mult
                </li>
                <li>Cartas: +{h.card_chips} fichas</li>
                {h.jokers.map((j) => (
                  <li key={j.id} className={j.triggered ? 'text-slate-200' : 'text-slate-600'}>
                    {j.name}:{' '}
                    {j.chips_added > 0 && `+${j.chips_added} fichas`}
                    {j.mult_added > 0 && `+${j.mult_added} Mult`}
                    {j.mult_factor > 1 && `Mult ×${j.mult_factor}`}
                    {!j.triggered && 'sin efecto'}
                  </li>
                ))}
              </ul>
              <p className="mt-2 break-words rounded-md bg-black/40 px-2 py-1.5 font-mono text-[0.7rem] leading-relaxed text-neon-lime">
                {formulaText(h)}
              </p>
              <p className="mt-1 text-right text-xs text-slate-500">Acumulado: {h.accumulated_score.toLocaleString('es-CO')}</p>
            </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}
