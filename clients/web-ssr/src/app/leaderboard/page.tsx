import Link from 'next/link';
import { gatewayGet } from '../../lib/server/gateway';
import type { RankedPlayer } from '../../lib/types';

export const dynamic = 'force-dynamic';

/** SSR: ranking global (Next → gateway → leaderboard-stats-ms → Neo4j/Redis). Página pública. */
export default async function LeaderboardPage() {
  const res = await gatewayGet<{ players: RankedPlayer[]; cached: boolean; generated_at: string }>('/api/leaderboard/top?limit=20');
  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <div>
        <p className="label-caps">Ranking global</p>
        <h1 className="mt-1 font-display text-3xl font-black tracking-wider text-white">Salón de la fama</h1>
        <p className="mt-1 text-slate-400">Victorias y mejores puntajes de todas las partidas terminadas.</p>
      </div>
      {!res.ok ? (
        <p role="alert" className="rounded-lg border border-neon-red/40 bg-neon-red/10 px-3 py-2 text-neon-red">{res.message}</p>
      ) : res.data.players.length === 0 ? (
        <p className="glass p-4 text-slate-400">Todavía no hay partidas terminadas.</p>
      ) : (
        <div className="glass-neon-violet overflow-x-auto p-2">
          <table className="w-full text-left">
            <thead className="label-caps">
              <tr>
                <th className="p-3">#</th>
                <th className="p-3">Jugador</th>
                <th className="p-3 text-right">Victorias</th>
                <th className="p-3 text-right">Partidas</th>
                <th className="p-3 text-right">Mejor puntaje</th>
              </tr>
            </thead>
            <tbody>
              {res.data.players.map((p) => (
                <tr key={p.player_id} className="border-t border-white/10">
                  <td className="p-3 font-display font-bold text-neon-amber">{p.rank}</td>
                  <td className="p-3 font-mono text-slate-100">{p.alias ?? p.player_id.slice(0, 8)}</td>
                  <td className="p-3 text-right font-mono text-neon-lime">{p.wins}</td>
                  <td className="p-3 text-right font-mono">{p.matches}</td>
                  <td className="p-3 text-right font-mono text-neon-cyan">{p.best_score.toLocaleString('es-CO')}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="label-caps p-3 !text-[0.6rem]">
            {res.data.cached ? 'servido desde la caché de Redis' : 'calculado en Neo4j'} · renderizado en servidor
          </p>
        </div>
      )}
      <Link href="/lobby" className="text-sm text-neon-cyan underline underline-offset-4">Volver al lobby</Link>
    </div>
  );
}
