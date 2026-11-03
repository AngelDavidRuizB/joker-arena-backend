import { Bot, Crown, Eye } from 'lucide-react';
import type { RoomState } from '../lib/types';

/** Roster of the room: the owner, the optional bot opponent and any spectators. */
export function PlayerPanel({ room, meId }: { room: RoomState; meId: string }) {
  return (
    <section aria-labelledby="players-title" className="glass p-4">
      <h2 id="players-title" className="label-caps mb-3">
        Jugadores ({room.players.length})
      </h2>
      <ul className="space-y-2">
        {room.players.map((p) => {
          const host = p.id === room.host_id;
          const me = p.id === meId;
          return (
            <li
              key={p.id}
              className={`flex items-center gap-3 rounded-xl border px-3 py-2 ${
                me ? 'border-neon-cyan/50 bg-neon-cyan/10' : 'border-white/10 bg-ink-950/50'
              }`}
            >
              {p.is_bot ? (
                <Bot className="h-5 w-5 text-neon-magenta" aria-hidden="true" />
              ) : host ? (
                <Crown className="h-5 w-5 text-neon-amber" aria-hidden="true" />
              ) : (
                <Eye className="h-5 w-5 text-slate-400" aria-hidden="true" />
              )}
              <div className="min-w-0 flex-1">
                <div className="truncate font-semibold">
                  {p.alias}
                  {me && <span className="ml-2 text-xs text-neon-cyan">(tú)</span>}
                </div>
                <div className="text-xs text-slate-400">
                  {p.is_bot ? 'Oponente simulado' : host ? 'Jugador de la partida' : 'Espectador'}
                </div>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
