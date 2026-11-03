'use client';

import { useState, type FormEvent } from 'react';
import { motion } from 'framer-motion';
import { Bot, Copy, DoorOpen, Swords, Users } from 'lucide-react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { ApiError, createRoom, getRoom, type Difficulty } from '../api/client';
import { NeonButton } from '../components/NeonButton';
import { DEFAULT_TARGET_SCORE, TARGET_OPTIONS } from '../lib/constants';
import { normalizeRoomCode, validateRoomCode } from '../lib/validation';
import { useSession } from '../store/session';


/** F2: creación de salas (con o sin Bot) y unión por código. */
export function Lobby() {
  const router = useRouter();
  const navigate = (to: string) => router.push(to);
  const player = useSession((s) => s.player);
  const [difficulty, setDifficulty] = useState<Difficulty>('normal');

  const [withBot, setWithBot] = useState(true);
  const [target, setTarget] = useState<number>(DEFAULT_TARGET_SCORE);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [createdCode, setCreatedCode] = useState<string | null>(null);

  const [code, setCode] = useState('');
  const [joining, setJoining] = useState(false);
  const [joinError, setJoinError] = useState<string | null>(null);

  const onCreate = async () => {
    setCreating(true);
    setCreateError(null);
    try {
      if (!player) return;
      const room = await createRoom(player.token, withBot, target, difficulty);
      setCreatedCode(room.room_code);
      toast.success(`Sala ${room.room_code} creada`, {
        description: withBot ? `Jugarás contra el Bot (${difficulty}).` : 'Modo solitario.',
      });
      navigate(`/room/${room.room_code}`);
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : 'No se pudo crear la sala.');
    } finally {
      setCreating(false);
    }
  };

  const onJoin = async (e: FormEvent) => {
    e.preventDefault();
    setJoinError(null);
    const normalized = normalizeRoomCode(code);
    const invalid = validateRoomCode(normalized);
    if (invalid) {
      setJoinError(invalid);
      return;
    }
    setJoining(true);
    try {
      if (!player) return;
      const room = await getRoom(player.token, normalized);
      if (room.status === 'closed') {
        setJoinError('Esa partida ya terminó. Crea una sala nueva.');
        return;
      }
      navigate(`/room/${room.room_code}`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) setJoinError(`No existe ninguna sala con el código ${normalized}.`);
      else setJoinError(err instanceof ApiError ? err.message : 'No se pudo consultar la sala.');
    } finally {
      setJoining(false);
    }
  };

  const copyCode = async () => {
    if (!createdCode) return;
    try {
      await navigator.clipboard.writeText(createdCode);
      toast.success('Código copiado');
    } catch {
      toast.error('No se pudo copiar el código');
    }
  };

  return (
    <div className="mx-auto max-w-5xl space-y-8">
      <motion.div initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }}>
        <p className="label-caps">Lobby</p>
        <h1 className="mt-1 font-display text-3xl font-black tracking-wider text-white">
          Listo, <span className="text-neon-cyan text-glow-cyan">{player?.alias ?? '...'}</span>
        </h1>
        <p className="mt-1 text-slate-400">Crea una sala nueva o únete con el código de otra.</p>
      </motion.div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Create room */}
        <motion.section
          initial={{ opacity: 0, x: -20 }}
          animate={{ opacity: 1, x: 0 }}
          className="glass-neon-magenta space-y-6 p-6"
          aria-labelledby="create-title"
        >
          <div className="flex items-center gap-3">
            <Swords className="h-6 w-6 text-neon-magenta" aria-hidden="true" />
            <h2 id="create-title" className="font-display text-lg font-bold tracking-wider">
              Crear sala
            </h2>
          </div>

          <fieldset>
            <legend className="label-caps mb-3">Meta de puntos</legend>
            <div className="grid grid-cols-3 gap-3">
              {TARGET_OPTIONS.map((t) => (
                <label
                  key={t.value}
                  className={`cursor-pointer rounded-xl border p-3 text-center transition has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-neon-cyan ${
                    target === t.value
                      ? 'border-neon-magenta bg-neon-magenta/15 shadow-neon-magenta'
                      : 'border-white/10 bg-ink-950/50 hover:border-white/30'
                  }`}
                >
                  <input
                    type="radio"
                    name="target"
                    value={t.value}
                    checked={target === t.value}
                    onChange={() => setTarget(t.value)}
                    className="sr-only"
                  />
                  <div className="font-display text-xl font-bold">{t.value.toLocaleString('es-CO')}</div>
                  <div className="text-xs text-slate-400">{t.hint}</div>
                </label>
              ))}
            </div>
          </fieldset>

          <div className="flex items-center justify-between gap-4 rounded-xl border border-white/10 bg-ink-950/50 p-4">
            <div className="flex items-center gap-3">
              <Bot className="h-6 w-6 text-neon-cyan" aria-hidden="true" />
              <div>
                <div id="bot-label" className="font-semibold">
                  Jugar contra Bot
                </div>
                <div className="text-sm text-slate-400">Simulador de oponente para practicar.</div>
              </div>
            </div>
            <button
              type="button"
              role="switch"
              aria-checked={withBot}
              aria-labelledby="bot-label"
              onClick={() => setWithBot((v) => !v)}
              className={`relative h-7 w-12 shrink-0 rounded-full border transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neon-cyan ${
                withBot ? 'border-neon-cyan bg-neon-cyan/30' : 'border-white/20 bg-white/5'
              }`}
            >
              <span
                className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all ${
                  withBot ? 'left-[1.5rem] shadow-neon-cyan' : 'left-0.5'
                }`}
              />
            </button>
          </div>

          {withBot && (
            <fieldset>
              <legend className="label-caps mb-3">Dificultad del Bot</legend>
              <div className="grid grid-cols-3 gap-3">
                {(['easy', 'normal', 'hard'] as const).map((d) => (
                  <label
                    key={d}
                    className={`cursor-pointer rounded-xl border p-2 text-center font-display text-xs uppercase tracking-widest transition ${
                      difficulty === d ? 'border-neon-cyan bg-neon-cyan/15 shadow-neon-cyan' : 'border-white/10 bg-ink-950/50 hover:border-white/30'
                    }`}
                  >
                    <input type="radio" name="difficulty" value={d} checked={difficulty === d} onChange={() => setDifficulty(d)} className="sr-only" />
                    {d === 'easy' ? 'Fácil' : d === 'normal' ? 'Normal' : 'Difícil'}
                  </label>
                ))}
              </div>
            </fieldset>
          )}

          {createError && (
            <p role="alert" className="rounded-lg border border-neon-red/40 bg-neon-red/10 px-3 py-2 text-sm text-neon-red">
              {createError}
            </p>
          )}

          <NeonButton block variant="magenta" loading={creating} onClick={onCreate} icon={<Swords className="h-4 w-4" />}>
            {withBot ? 'Crear sala (con Bot)' : 'Crear sala'}
          </NeonButton>

          {createdCode && (
            <div className="flex items-center justify-between rounded-xl border border-neon-lime/40 bg-neon-lime/10 px-4 py-3">
              <span className="font-mono text-lg font-bold tracking-widest text-neon-lime">{createdCode}</span>
              <button
                type="button"
                onClick={copyCode}
                aria-label="Copiar código de sala"
                className="rounded-lg p-2 text-neon-lime hover:bg-neon-lime/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neon-cyan"
              >
                <Copy className="h-5 w-5" />
              </button>
            </div>
          )}
        </motion.section>

        {/* Join room */}
        <motion.section
          initial={{ opacity: 0, x: 20 }}
          animate={{ opacity: 1, x: 0 }}
          className="glass-neon-cyan space-y-6 p-6"
          aria-labelledby="join-title"
        >
          <div className="flex items-center gap-3">
            <Users className="h-6 w-6 text-neon-cyan" aria-hidden="true" />
            <h2 id="join-title" className="font-display text-lg font-bold tracking-wider">
              Unirse a una sala
            </h2>
          </div>
          <p className="text-slate-400">
            Introduce el código que te compartieron. Si la sala es de otro jugador, entrarás como espectador y
            verás sus jugadas en vivo.
          </p>
          <form onSubmit={onJoin} noValidate className="space-y-4">
            <div>
              <label htmlFor="room-code" className="label-caps mb-2 block">
                Código de sala
              </label>
              <input
                id="room-code"
                value={code}
                onChange={(e) => {
                  setCode(e.target.value.toUpperCase());
                  setJoinError(null);
                }}
                placeholder="JKR-A7K2"
                autoComplete="off"
                maxLength={12}
                aria-invalid={joinError ? true : undefined}
                aria-describedby="join-feedback"
                className="field text-center text-lg uppercase tracking-[0.3em]"
              />
              <p id="join-feedback" role={joinError ? 'alert' : undefined} className="mt-2 min-h-[1.25rem] text-sm text-neon-red">
                {joinError}
              </p>
            </div>
            <NeonButton type="submit" block variant="cyan" loading={joining} icon={<DoorOpen className="h-4 w-4" />}>
              Unirme
            </NeonButton>
          </form>
        </motion.section>
      </div>
    </div>
  );
}
