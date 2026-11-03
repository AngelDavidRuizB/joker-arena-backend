'use client';

import { AnimatePresence, motion, useReducedMotion } from 'framer-motion';
import { ArrowLeft, Bot, Copy, Play, RotateCcw } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { toast } from 'sonner';
import { AuditPanel } from '../components/AuditPanel';
import { BotScoreboard } from '../components/BotScoreboard';
import { CardFan } from '../components/CardFan';
import { ConnectionBadge } from '../components/ConnectionBadge';
import { Hud } from '../components/Hud';
import { JokerBar } from '../components/JokerBar';
import { NeonButton } from '../components/NeonButton';
import { PlayerPanel } from '../components/PlayerPanel';
import { ResultOverlay } from '../components/ResultOverlay';
import { ScoreCascade } from '../components/ScoreCascade';
import type { CascadeStep } from '../lib/cascade';
import { MAX_SELECTION, REVEAL_MS, SELECT_HOLD_MS, isHost, useGame } from '../store/game';
import { useSession } from '../store/session';
import { normalizeRoomCode, validateRoomCode } from '../lib/validation';
import { FATAL_ERROR_CODES, GameSocket } from '../ws/gameSocket';

const ACTION_TIMEOUT_MS = 8000;

/** F3–F5: the interactive table. All game rules run on the server; this view only renders state. */
export function Table() {
  const { code: rawCode } = useParams<{ code: string }>();
  const code = normalizeRoomCode(rawCode ?? '');
  const router = useRouter();
  const navigate = (to: string) => router.push(to);
  const player = useSession((s) => s.player)!;

  const connection = useGame((s) => s.connection);
  const room = useGame((s) => s.room);
  const selected = useGame((s) => s.selected);
  const cascade = useGame((s) => s.cascade);
  const history = useGame((s) => s.history);
  const finished = useGame((s) => s.finished);
  const busy = useGame((s) => s.busy);
  const error = useGame((s) => s.error);
  const botTurn = useGame((s) => s.botTurn);
  const botPhase = useGame((s) => s.botPhase);
  const queued = useGame((s) => s.queued);
  const toggleCard = useGame((s) => s.toggleCard);
  const clearSelection = useGame((s) => s.clearSelection);
  const endCascade = useGame((s) => s.endCascade);

  const reduce = useReducedMotion();
  const socketRef = useRef<GameSocket | null>(null);
  const [activeJoker, setActiveJoker] = useState<string | null>(null);
  const [showResult, setShowResult] = useState(true);
  const [fatal, setFatal] = useState<string | null>(null);
  const codeError = validateRoomCode(code);

  // Open the room socket for this code/player; closing it on unmount frees the server client.
  useEffect(() => {
    if (codeError) return;
    const game = useGame.getState();
    game.reset();
    setFatal(null);
    setShowResult(true);
    const sock = new GameSocket({
      roomCode: code,
      playerId: player.id,
      token: player.token,
      onMessage: (msg) => {
        useGame.getState().applyMessage(msg);
        if (msg.type === 'ERROR' && FATAL_ERROR_CODES.has(msg.payload.code)) setFatal(msg.payload.message);
      },
      onStatus: (s) => useGame.getState().setConnection(s),
    });
    socketRef.current = sock;
    sock.connect();
    return () => {
      sock.close();
      socketRef.current = null;
      useGame.getState().reset();
    };
  }, [code, player.id, player.token, codeError]);

  // Surface non-fatal server errors as toasts.
  useEffect(() => {
    if (error && !FATAL_ERROR_CODES.has(error.code)) {
      toast.error(error.message);
      useGame.getState().dismissError();
    }
  }, [error]);

  // Safety net: never leave the controls locked if the server does not answer.
  useEffect(() => {
    if (!busy) return;
    const t = setTimeout(() => {
      useGame.getState().setBusy(false);
      toast.error('El servidor tardó en responder. Inténtalo de nuevo.');
    }, ACTION_TIMEOUT_MS);
    return () => clearTimeout(t);
  }, [busy]);

  // Drive the bot turn animation: reveal the hand, then highlight the chosen cards, then resolve
  // (PLAY hands off to the queued bot cascade; DISCARD closes the turn so the new hand can arrive).
  useEffect(() => {
    if (!botTurn || botPhase === 'resolve' || botPhase === 'idle') return;
    const delay = reduce ? 0 : botPhase === 'reveal' ? REVEAL_MS : SELECT_HOLD_MS;
    const t = setTimeout(() => useGame.getState().advanceBotPhase(), delay);
    return () => clearTimeout(t);
  }, [botTurn, botPhase, reduce]);

  // Drain the held-back queue when no cascade is mounted to carry it and the bot turn is not
  // mid-animation. Covers the two seams a ScoreCascade can't reach on its own: a bot PLAY that
  // reached 'resolve' (the queued bot HAND_SCORED becomes the active cascade) and a closed bot
  // DISCARD turn (the refilled bot_hand STATE applies). onCascadeDone handles the live cascade.
  useEffect(() => {
    if (cascade || botPhase === 'reveal' || botPhase === 'select') return;
    if (queued.length === 0) return;
    useGame.getState().drainQueue();
  }, [cascade, botPhase, queued]);

  const host = isHost(room, player.id);
  const live = connection === 'open';
  const matchOver = !!finished || room?.status === 'FINISHED';
  const botPlaying = !!botTurn;
  const canAct = host && live && !!room && !matchOver && !busy && !cascade && !botPlaying && selected.length >= 1;

  const send = useCallback(
    (type: 'PLAY_HAND' | 'DISCARD_CARDS') => {
      const { selected: picked, setBusy } = useGame.getState();
      if (picked.length < 1 || picked.length > MAX_SELECTION) return;
      if (socketRef.current?.send(type, picked)) setBusy(true);
      else toast.error('Sin conexión con la sala. Reintentando…');
    },
    [],
  );

  // Keyboard shortcuts: 1-8 toggle cards, P plays, D discards, Esc clears the selection.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.ctrlKey || e.metaKey || e.altKey) return;
      const t = e.target as HTMLElement | null;
      if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')) return;
      if (/^[1-8]$/.test(e.key)) toggleCard(Number(e.key) - 1);
      else if (e.key === 'Escape') clearSelection();
      else if (e.key.toLowerCase() === 'p' && canAct) send('PLAY_HAND');
      else if (e.key.toLowerCase() === 'd' && canAct && (room?.discards_left ?? 0) > 0) send('DISCARD_CARDS');
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [canAct, room?.discards_left, send, toggleCard, clearSelection]);

  const onStep = useCallback((step: CascadeStep) => setActiveJoker(step.jokerId ?? null), []);
  const onCascadeDone = useCallback(() => {
    setActiveJoker(null);
    endCascade();
  }, [endCascade]);

  const copyCode = async () => {
    try {
      await navigator.clipboard.writeText(code);
      toast.success('Código copiado');
    } catch {
      toast.error('No se pudo copiar el código');
    }
  };

  if (codeError || fatal) {
    return (
      <div className="mx-auto max-w-md">
        <div className="glass-neon-magenta space-y-5 p-8 text-center" role="alert">
          <h1 className="font-display text-2xl font-black tracking-wider text-neon-red">No se puede entrar</h1>
          <p className="text-slate-300">{codeError ?? fatal}</p>
          <NeonButton block variant="cyan" onClick={() => navigate('/lobby')} icon={<ArrowLeft className="h-4 w-4" />}>
            Volver al lobby
          </NeonButton>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Top bar */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => navigate('/lobby')}
            aria-label="Volver al lobby"
            className="rounded-lg border border-white/15 p-2 text-slate-300 hover:text-neon-cyan focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neon-cyan"
          >
            <ArrowLeft className="h-5 w-5" />
          </button>
          <div>
            <p className="label-caps">Sala</p>
            <div className="flex items-center gap-2">
              <span className="font-mono text-2xl font-bold tracking-[0.25em] text-neon-cyan text-glow-cyan">{code}</span>
              <button
                type="button"
                onClick={copyCode}
                aria-label="Copiar código de sala"
                className="rounded-lg p-1.5 text-slate-400 hover:text-neon-cyan focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neon-cyan"
              >
                <Copy className="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
        <ConnectionBadge status={connection} />
      </div>

      {!room ? (
        <div className="glass-neon-cyan flex min-h-[24rem] flex-col items-center justify-center gap-4 p-10" role="status">
          <span className="h-10 w-10 animate-spin rounded-full border-4 border-neon-cyan border-t-transparent" aria-hidden="true" />
          <p className="font-display text-sm uppercase tracking-[0.25em] text-slate-300">
            {connection === 'closed' ? 'No se pudo conectar con la sala' : 'Conectando y repartiendo cartas…'}
          </p>
          {connection === 'closed' && (
            <NeonButton variant="cyan" onClick={() => window.location.reload()}>
              Reintentar
            </NeonButton>
          )}
        </div>
      ) : (
        <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
          <div className="min-w-0 space-y-6">
            <Hud room={room} />
            <JokerBar owned={room.active_jokers} activeId={activeJoker} />

            {!host && (
              <p role="status" className="rounded-xl border border-neon-violet/40 bg-neon-violet/10 px-4 py-3 text-neon-violet">
                Modo espectador: ves la partida en vivo, pero solo el dueño de la sala puede jugar.
              </p>
            )}

            {/* Felt */}
            <section
              aria-label="Tapete de juego"
              className="relative overflow-hidden rounded-[2rem] border border-neon-cyan/30 bg-[radial-gradient(ellipse_at_center,rgba(139,92,246,0.22),rgba(10,7,32,0.92)_70%)] p-4 shadow-neon-cyan sm:p-6"
            >
              <div aria-hidden="true" className="pointer-events-none absolute inset-0 opacity-[0.07] [background-image:repeating-linear-gradient(45deg,#fff_0,#fff_1px,transparent_1px,transparent_22px)]" />

              <div className="relative flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="relative h-14 w-10" aria-hidden="true">
                    {[0, 1, 2].map((i) => (
                      <span
                        key={i}
                        className="absolute h-14 w-10 rounded-md border border-neon-violet/60 bg-gradient-to-br from-ink-700 to-ink-900"
                        style={{ left: i * 3, top: -i * 2 }}
                      />
                    ))}
                  </div>
                  <span className="font-mono text-sm text-slate-400">{room.deck_count} cartas en el mazo</span>
                </div>
                {botPlaying ? (
                  <span
                    className="flex items-center gap-1.5 rounded-full border border-neon-magenta/50 bg-neon-magenta/10 px-3 py-1 font-display text-sm font-bold uppercase tracking-widest text-neon-magenta shadow-neon-magenta"
                    aria-live="polite"
                  >
                    <Bot className="h-4 w-4" aria-hidden="true" />
                    Turno del Bot
                  </span>
                ) : (
                  <span className="rounded-full border border-neon-amber/40 bg-neon-amber/10 px-3 py-1 font-mono text-sm text-neon-amber" aria-live="polite">
                    Seleccionadas {selected.length}/{MAX_SELECTION}
                  </span>
                )}
              </div>

              {botPlaying ? (
                <CardFan
                  cards={room.bot_hand ?? []}
                  selected={botTurn.indices}
                  disabled
                  onToggle={() => {}}
                  readOnly
                />
              ) : (
                <CardFan
                  cards={room.cards_in_hand}
                  selected={selected}
                  disabled={!host || !live || matchOver || busy || !!cascade}
                  onToggle={toggleCard}
                />
              )}

              {botPlaying ? (
                <div className="relative flex flex-wrap items-center justify-center gap-3 border-t border-white/10 pt-5" aria-live="polite">
                  <span className="flex items-center gap-2 font-display text-sm uppercase tracking-widest text-neon-magenta">
                    <Bot className="h-4 w-4" aria-hidden="true" />
                    {botTurn.action === 'DISCARD'
                      ? `El bot descarta ${botTurn.indices.length}`
                      : 'El bot juega su mano'}
                  </span>
                </div>
              ) : (
                <>
                  <div className="relative flex flex-wrap items-center justify-center gap-3 border-t border-white/10 pt-5">
                    <NeonButton
                      variant="amber"
                      disabled={!canAct}
                      loading={busy}
                      onClick={() => send('PLAY_HAND')}
                      icon={<Play className="h-4 w-4 fill-current" />}
                    >
                      Jugar mano ({selected.length}/5)
                    </NeonButton>
                    <NeonButton
                      variant="magenta"
                      disabled={!canAct || room.discards_left <= 0}
                      onClick={() => send('DISCARD_CARDS')}
                      icon={<RotateCcw className="h-4 w-4" />}
                    >
                      Descartar ({room.discards_left})
                    </NeonButton>
                    <NeonButton variant="ghost" disabled={selected.length === 0 || busy} onClick={clearSelection}>
                      Limpiar
                    </NeonButton>
                  </div>
                  <p className="relative mt-3 text-center text-xs text-slate-500">
                    Atajos: <kbd className="font-mono">1–8</kbd> selecciona · <kbd className="font-mono">P</kbd> juega ·{' '}
                    <kbd className="font-mono">D</kbd> descarta · <kbd className="font-mono">Esc</kbd> limpia
                  </p>
                </>
              )}

              <AnimatePresence>
                {cascade && (
                  <ScoreCascade
                    key={`${cascade.player_id}-${cascade.hand_number}`}
                    hand={cascade}
                    authorLabel={room.players.find((p) => p.id === room.host_id)?.alias ?? 'Anfitrión'}
                    onStep={onStep}
                    onDone={onCascadeDone}
                  />
                )}
              </AnimatePresence>
            </section>
          </div>

          <aside className="space-y-6">
            <PlayerPanel room={room} meId={player.id} />
            <BotScoreboard room={room} />
            <AuditPanel history={history} />
          </aside>
        </div>
      )}

      <AnimatePresence>
        {finished && !cascade && showResult && (
          <ResultOverlay result={finished} onLobby={() => navigate('/lobby')} onReview={() => setShowResult(false)} />
        )}
        {finished && room?.match_id && (
          <div className="fixed bottom-6 right-6 z-50">
            <Link
              href={`/matches/${room.match_id}`}
              className="rounded-xl border border-neon-violet/60 bg-ink-900/90 px-4 py-2 font-display text-xs uppercase tracking-widest text-neon-violet shadow-neon-violet hover:bg-neon-violet/20"
            >
              Ver auditoría de la partida
            </Link>
          </div>
        )}
      </AnimatePresence>

      {finished && !showResult && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="text-center">
          <NeonButton variant="cyan" onClick={() => navigate('/lobby')}>
            Nueva partida
          </NeonButton>
        </motion.div>
      )}
    </div>
  );
}
