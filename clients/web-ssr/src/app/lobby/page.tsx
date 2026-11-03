import Link from 'next/link';
import { redirect } from 'next/navigation';
import { gatewayGet, hasSession } from '../../lib/server/gateway';
import type { LobbyRoom } from '../../lib/types';
import { Lobby } from '../../views/Lobby';

export const dynamic = 'force-dynamic';

const MODE = { bot: 'vs Bot', pvp: 'Solitario' } as const;
const STATUS = { open: 'Abierta', in_progress: 'En juego', closed: 'Cerrada' } as const;

/**
 * SSR: la lista de salas se consulta en el servidor (Next → gateway → rooms-lobby-ms) y
 * llega al navegador ya renderizada en el HTML. El formulario de crear/unirse es interactivo.
 */
export default async function LobbyPage() {
  if (!hasSession()) redirect('/');
  const [open, playing] = await Promise.all([
    gatewayGet<{ rooms: LobbyRoom[] }>('/api/rooms?status=open&limit=12', true),
    gatewayGet<{ rooms: LobbyRoom[] }>('/api/rooms?status=in_progress&limit=12', true),
  ]);
  const rooms = [...(open.ok ? open.data.rooms : []), ...(playing.ok ? playing.data.rooms : [])];

  return (
    <div className="space-y-10">
      <Lobby />
      <section className="mx-auto max-w-5xl" aria-labelledby="rooms-title">
        <div className="mb-3 flex items-end justify-between">
          <h2 id="rooms-title" className="font-display text-lg font-bold tracking-wider text-white">
            Salas activas
          </h2>
          <span className="label-caps !text-[0.6rem]">renderizado en servidor · {new Date().toLocaleTimeString('es-CO')}</span>
        </div>
        {!open.ok ? (
          <p role="alert" className="rounded-lg border border-neon-red/40 bg-neon-red/10 px-3 py-2 text-sm text-neon-red">
            {open.message}
          </p>
        ) : rooms.length === 0 ? (
          <p className="glass p-4 text-slate-400">No hay salas activas. ¡Crea la primera!</p>
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {rooms.map((r) => (
              <li key={r.room_code} className="glass flex items-center justify-between p-4">
                <div>
                  <div className="font-mono text-lg font-bold tracking-widest text-neon-cyan">{r.room_code}</div>
                  <div className="text-sm text-slate-400">
                    {MODE[r.mode]} · {r.difficulty} · meta {r.target_score.toLocaleString('es-CO')}
                  </div>
                  <div className="text-xs text-slate-500">
                    {STATUS[r.status]} · anfitrión {r.players[0]?.alias ?? '—'}
                  </div>
                </div>
                <Link
                  href={`/room/${r.room_code}`}
                  className="rounded-lg border border-neon-cyan/50 px-3 py-1.5 font-display text-[0.65rem] uppercase tracking-widest text-neon-cyan hover:bg-neon-cyan/15"
                >
                  {r.status === 'open' ? 'Entrar' : 'Ver'}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
