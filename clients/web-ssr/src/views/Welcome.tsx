'use client';

import { useState, type FormEvent } from 'react';
import { motion } from 'framer-motion';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { enter, ApiError } from '../api/client';
import { Logo } from '../components/Logo';
import { NeonButton } from '../components/NeonButton';
import { JOKERS } from '../lib/cards';
import { ALIAS_MAX, validateAlias } from '../lib/validation';
import { useSession } from '../store/session';

const JOKER_ACCENT = {
  cyan: 'border-neon-cyan/40 text-neon-cyan',
  magenta: 'border-neon-magenta/40 text-neon-magenta',
  violet: 'border-neon-violet/40 text-neon-violet',
} as const;

/** F1: registro de jugador por alias (identificación liviana, sin contraseña). */
export function Welcome() {
  const router = useRouter();
  const navigate = (to: string) => router.push(to);
  const player = useSession((s) => s.player);
  const setPlayer = useSession((s) => s.setPlayer);
  const clear = useSession((s) => s.clear);

  const [alias, setAlias] = useState('');
  const [password, setPassword] = useState('');
  const isAdmin = alias.trim().toLowerCase() === 'admin';
  const [touched, setTouched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const clientError = validateAlias(alias);
  const shownError = serverError ?? (touched ? clientError : null);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setTouched(true);
    setServerError(null);
    if (clientError) return;
    setLoading(true);
    try {
      const res = await enter(alias.trim(), isAdmin ? password : undefined);
      setPlayer({ id: res.player.player_id, alias: res.player.alias, role: res.player.role, token: res.token });
      toast.success(`¡Bienvenido, ${res.player.alias}!`, { description: 'Sesión iniciada con token JWT.' });
      navigate(res.player.role === 'admin' ? '/admin' : '/lobby');
      router.refresh();
    } catch (err) {
      setServerError(err instanceof ApiError ? err.message : 'No se pudo registrar el alias.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto flex max-w-5xl flex-col items-center gap-10 py-6 sm:py-12">
      <motion.div initial={{ opacity: 0, y: -16 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.6 }}>
        <Logo size="lg" />
      </motion.div>

      <motion.p
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 0.25 }}
        className="max-w-xl text-center text-lg text-slate-300"
      >
        Póker con <span className="text-neon-amber text-glow-amber">Jokers</span> en tiempo real. Combina manos,
        activa multiplicadores y supera la meta antes de quedarte sin jugadas.
      </motion.p>

      <motion.section
        initial={{ opacity: 0, scale: 0.96 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ delay: 0.15, duration: 0.5 }}
        className="glass-neon-cyan w-full max-w-md p-6 sm:p-8"
        aria-labelledby="welcome-title"
      >
        {player ? (
          <div className="space-y-5 text-center">
            <h1 id="welcome-title" className="font-display text-xl font-bold tracking-wider text-white">
              Hola de nuevo
            </h1>
            <p className="text-slate-300">
              Sesión activa como <span className="font-mono text-neon-cyan">{player.alias}</span>
            </p>
            <NeonButton block variant="cyan" onClick={() => navigate('/lobby')}>
              Ir al lobby
            </NeonButton>
            <button
              type="button"
              onClick={clear}
              className="text-sm text-slate-400 underline decoration-dotted underline-offset-4 hover:text-neon-magenta focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neon-cyan"
            >
              Usar otro alias
            </button>
          </div>
        ) : (
          <form onSubmit={onSubmit} noValidate className="space-y-5">
            <div>
              <h1 id="welcome-title" className="font-display text-xl font-bold tracking-wider text-white">
                Entra a la arena
              </h1>
              <p className="mt-1 text-sm text-slate-400">Elige un alias. No necesitas contraseña.</p>
            </div>

            <div>
              <label htmlFor="alias" className="label-caps mb-2 block">
                Alias del jugador
              </label>
              <input
                id="alias"
                name="alias"
                type="text"
                autoComplete="off"
                autoFocus
                maxLength={ALIAS_MAX + 10}
                value={alias}
                placeholder="JokerPro99"
                onChange={(e) => {
                  setAlias(e.target.value);
                  setServerError(null);
                }}
                onBlur={() => setTouched(true)}
                aria-invalid={shownError ? true : undefined}
                aria-describedby="alias-feedback"
                className="field"
              />
              <p
                id="alias-feedback"
                role={shownError ? 'alert' : undefined}
                className={`mt-2 min-h-[1.25rem] text-sm ${shownError ? 'text-neon-red' : 'text-slate-500'}`}
              >
                {shownError ?? `${alias.trim().length}/${ALIAS_MAX} · letras, números, - y _`}
              </p>
            </div>

            {isAdmin && (
              <div>
                <label htmlFor="password" className="label-caps mb-2 block">
                  Contraseña de administración
                </label>
                <input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} className="field" />
              </div>
            )}

            <NeonButton type="submit" block loading={loading} variant="cyan">
              Entrar y jugar
            </NeonButton>
          </form>
        )}
      </motion.section>

      <ul className="grid w-full gap-4 sm:grid-cols-3" aria-label="Jokers del prototipo">
        {JOKERS.map((j, i) => (
          <motion.li
            key={j.id}
            initial={{ opacity: 0, y: 18 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.4 + i * 0.1 }}
            className={`glass flex items-start gap-3 border p-4 ${JOKER_ACCENT[j.color]}`}
          >
            <span className="text-3xl leading-none" aria-hidden="true">
              {j.glyph}
            </span>
            <div>
              <h2 className="font-display text-sm font-bold uppercase tracking-widest">{j.name}</h2>
              <p className="mt-1 text-sm text-slate-300">{j.description}</p>
            </div>
          </motion.li>
        ))}
      </ul>
    </div>
  );
}
