import Link from 'next/link';
import { gatewayGet } from '../../../lib/server/gateway';

export const dynamic = 'force-dynamic';

interface Joker { id: string; name: string; triggered: boolean; chips_added: number; mult_added: number; mult_factor: number }
interface HandRow {
  player_id: string; is_bot: boolean; hand_number: number; hand_type: string; chips: number; mult: number;
  score: number; accumulated_score: number; ruleset_version: number; jokers: Joker[];
}
interface MatchDoc {
  match: { room_code: string; status: string; reason?: string; target_score?: number;
           results?: { player_id: string; alias?: string; score: number; won: boolean; is_bot: boolean }[] } | null;
  hands: HandRow[];
}

/** SSR: auditoría mano a mano de una partida (Next → gateway → telemetry-ms → MongoDB). */
export default async function MatchPage({ params }: { params: { id: string } }) {
  const res = await gatewayGet<MatchDoc>(`/api/telemetry/matches/${encodeURIComponent(params.id)}`, true);
  if (!res.ok) {
    return (
      <div className="mx-auto max-w-3xl space-y-4">
        <h1 className="font-display text-2xl font-bold text-white">Auditoría de partida</h1>
        <p role="alert" className="rounded-lg border border-neon-red/40 bg-neon-red/10 px-3 py-2 text-neon-red">{res.message}</p>
        <Link href="/lobby" className="text-sm text-neon-cyan underline underline-offset-4">Volver al lobby</Link>
      </div>
    );
  }
  const { match, hands } = res.data;
  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <div>
        <p className="label-caps">Auditoría · telemetría</p>
        <h1 className="mt-1 font-display text-3xl font-black tracking-wider text-white">
          Partida <span className="text-neon-cyan">{match?.room_code ?? params.id.slice(0, 8)}</span>
        </h1>
        {match?.results && (
          <p className="mt-2 text-slate-300">
            {match.results.map((r) => `${r.is_bot ? 'Bot' : r.alias ?? 'Jugador'}: ${r.score.toLocaleString('es-CO')}${r.won ? ' 🏆' : ''}`).join(' · ')}
            {match.reason ? ` · ${match.reason}` : ''}
          </p>
        )}
      </div>
      <div className="glass overflow-x-auto p-2">
        <table className="w-full text-left text-sm">
          <thead className="label-caps">
            <tr>
              <th className="p-2">Mano</th><th className="p-2">Quién</th><th className="p-2">Jugada</th>
              <th className="p-2 text-right">Chips × Mult</th><th className="p-2 text-right">Puntos</th>
              <th className="p-2 text-right">Acumulado</th><th className="p-2">Jokers activados</th><th className="p-2 text-right">Ruleset</th>
            </tr>
          </thead>
          <tbody>
            {hands.map((h) => (
              <tr key={`${h.player_id}-${h.hand_number}`} className="border-t border-white/10">
                <td className="p-2 font-mono">{h.hand_number}</td>
                <td className={`p-2 ${h.is_bot ? 'text-neon-magenta' : 'text-neon-cyan'}`}>{h.is_bot ? 'Bot' : 'Anfitrión'}</td>
                <td className="p-2 font-mono">{h.hand_type}</td>
                <td className="p-2 text-right font-mono">{h.chips} × {h.mult}</td>
                <td className="p-2 text-right font-mono text-neon-lime">{h.score.toLocaleString('es-CO')}</td>
                <td className="p-2 text-right font-mono">{h.accumulated_score.toLocaleString('es-CO')}</td>
                <td className="p-2 text-xs text-slate-300">
                  {h.jokers.filter((j) => j.triggered).map((j) => j.name).join(', ') || '—'}
                </td>
                <td className="p-2 text-right font-mono text-neon-amber">v{h.ruleset_version}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Link href="/lobby" className="text-sm text-neon-cyan underline underline-offset-4">Volver al lobby</Link>
    </div>
  );
}
