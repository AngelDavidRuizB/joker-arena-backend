import type { ConnectionStatus } from '../lib/types';

const LABEL: Record<ConnectionStatus, { text: string; dot: string }> = {
  idle: { text: 'Sin conexión', dot: 'bg-slate-500' },
  connecting: { text: 'Conectando…', dot: 'bg-neon-amber animate-pulse' },
  open: { text: 'En línea', dot: 'bg-neon-lime shadow-neon-lime' },
  reconnecting: { text: 'Reconectando…', dot: 'bg-neon-amber animate-pulse' },
  closed: { text: 'Desconectado', dot: 'bg-neon-red' },
};

export function ConnectionBadge({ status }: { status: ConnectionStatus }) {
  const { text, dot } = LABEL[status];
  return (
    <div
      role="status"
      aria-live="polite"
      className="flex items-center gap-2 rounded-full border border-white/10 bg-ink-900/70 px-3 py-1.5"
    >
      <span className={`h-2.5 w-2.5 rounded-full ${dot}`} aria-hidden="true" />
      <span className="label-caps !text-slate-300">{text}</span>
    </div>
  );
}
