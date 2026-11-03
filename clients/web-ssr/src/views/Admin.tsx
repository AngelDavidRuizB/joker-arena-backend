'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import { ApiError, request } from '../api/client';
import { NeonButton } from '../components/NeonButton';
import { useSession } from '../store/session';

/* ------------------------------------------------------------------ tipos */

interface Ruleset {
  version: number;
  updated_at: string;
  updated_by: string;
  jokers: {
    banner: { enabled: boolean; chips_per_discard: number };
    classic: { enabled: boolean; mult_bonus: number };
    duo: { enabled: boolean; mult_factor: number };
  };
  happy_hour: { enabled: boolean; global_mult: number };
}

interface ContainerInfo {
  name: string;
  service: string;
  state: string;
  health: string;
  cpu_percent: number;
  mem_bytes: number;
  mem_percent: number;
  light: 'green' | 'yellow' | 'red';
}

interface Snapshot {
  generated_at: string;
  containers: ContainerInfo[];
  rabbitmq: { status: string; messages_ready: number; publish_rate: number; deliver_rate: number; queues: { name: string; messages: number; consumers: number }[] };
  postgres: { status: string; connections: number; max_connections: number; usage_percent: number; latency_ms: number };
  redis: { status: string; connected_clients: number; latency_ms: number };
  sources: Record<string, string>;
  alarms: { source: string; severity: 'warning' | 'critical'; message: string }[];
}

/* ------------------------------------------------------- utilidades puras */

/** Construye el cuerpo del PUT a partir del estado del editor (exportado para pruebas). */
export function toPutBody(expectedVersion: number, r: Pick<Ruleset, 'jokers' | 'happy_hour'>) {
  return { expected_version: expectedVersion, jokers: r.jokers, happy_hour: r.happy_hour };
}

export function wsAdminUrl(token: string, loc: Pick<Location, 'protocol' | 'hostname'> = window.location): string {
  const base = process.env.NEXT_PUBLIC_GATEWAY_WS_URL;
  const origin = base ? base.replace(/\/$/, '') : `${loc.protocol === 'https:' ? 'wss:' : 'ws:'}//${loc.hostname}:8080`;
  return `${origin}/admin/cluster/metrics?token=${encodeURIComponent(token)}`;
}

const LIGHT: Record<ContainerInfo['light'], string> = {
  green: 'border-neon-lime/50 bg-neon-lime/10 text-neon-lime',
  yellow: 'border-neon-amber/60 bg-neon-amber/10 text-neon-amber',
  red: 'border-neon-red/70 bg-neon-red/15 text-neon-red',
};

/* ---------------------------------------------------------- editor reglas */

function Slider(props: { label: string; value: number; min: number; max: number; onChange: (v: number) => void; disabled?: boolean }) {
  return (
    <label className="block">
      <div className="flex justify-between text-sm">
        <span className="text-slate-300">{props.label}</span>
        <span className="font-mono text-neon-cyan">{props.value}</span>
      </div>
      <input
        type="range"
        min={props.min}
        max={props.max}
        value={props.value}
        disabled={props.disabled}
        onChange={(e) => props.onChange(Number(e.target.value))}
        className="mt-1 w-full accent-[#00f0ff] disabled:opacity-40"
      />
    </label>
  );
}

function Toggle(props: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm text-slate-200">
      <input type="checkbox" checked={props.checked} onChange={(e) => props.onChange(e.target.checked)} className="h-4 w-4 accent-[#ff2bd6]" />
      {props.label}
    </label>
  );
}

function RulesEditor({ token }: { token: string }) {
  const [current, setCurrent] = useState<Ruleset | null>(null);
  const [draft, setDraft] = useState<Ruleset | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const r = await request<Ruleset>('/gw-admin/liveops/ruleset', { token });
      setCurrent(r);
      setDraft(structuredClone(r));
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'No se pudo cargar el ruleset.');
    }
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  const save = async () => {
    if (!current || !draft) return;
    setSaving(true);
    setError(null);
    try {
      const saved = await request<Ruleset & { published: boolean }>('/gw-admin/liveops/ruleset', {
        method: 'PUT',
        token,
        body: JSON.stringify(toPutBody(current.version, draft)),
      });
      setCurrent(saved);
      setDraft(structuredClone(saved));
      toast.success(`Ruleset v${saved.version} guardado`, {
        description: saved.published ? 'Publicado: el motor lo aplica desde la siguiente mano.' : 'Guardado, pero el broker no confirmó la publicación.',
      });
    } catch (e) {
      if (e instanceof ApiError && e.code === 'version_conflict') {
        setError('Otro administrador guardó primero. Se recargó la versión vigente.');
        await load();
      } else {
        setError(e instanceof ApiError ? e.message : 'No se pudo guardar.');
      }
    } finally {
      setSaving(false);
    }
  };

  if (!draft || !current) {
    return <p className="text-slate-400">{error ?? 'Cargando ruleset…'}</p>;
  }
  const set = (fn: (r: Ruleset) => void) => {
    const next = structuredClone(draft);
    fn(next);
    setDraft(next);
  };
  const dirty = JSON.stringify(draft) !== JSON.stringify(current);

  return (
    <div className="space-y-5">
      <p className="label-caps !text-[0.6rem]">
        vigente: v{current.version} · {current.updated_by} · {new Date(current.updated_at).toLocaleString('es-CO')}
      </p>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2 rounded-xl border border-white/10 p-3">
          <Toggle label="Banner activo" checked={draft.jokers.banner.enabled} onChange={(v) => set((r) => void (r.jokers.banner.enabled = v))} />
          <Slider label="Chips por descarte" value={draft.jokers.banner.chips_per_discard} min={0} max={200} disabled={!draft.jokers.banner.enabled}
            onChange={(v) => set((r) => void (r.jokers.banner.chips_per_discard = v))} />
        </div>
        <div className="space-y-2 rounded-xl border border-white/10 p-3">
          <Toggle label="Joker Clásico activo" checked={draft.jokers.classic.enabled} onChange={(v) => set((r) => void (r.jokers.classic.enabled = v))} />
          <Slider label="+Mult" value={draft.jokers.classic.mult_bonus} min={0} max={50} disabled={!draft.jokers.classic.enabled}
            onChange={(v) => set((r) => void (r.jokers.classic.mult_bonus = v))} />
        </div>
        <div className="space-y-2 rounded-xl border border-white/10 p-3">
          <Toggle label="The Duo activo" checked={draft.jokers.duo.enabled} onChange={(v) => set((r) => void (r.jokers.duo.enabled = v))} />
          <Slider label="×Mult con par" value={draft.jokers.duo.mult_factor} min={1} max={10} disabled={!draft.jokers.duo.enabled}
            onChange={(v) => set((r) => void (r.jokers.duo.mult_factor = v))} />
        </div>
        <div className="space-y-2 rounded-xl border border-neon-amber/40 p-3">
          <Toggle label="Happy Hour" checked={draft.happy_hour.enabled} onChange={(v) => set((r) => void (r.happy_hour.enabled = v))} />
          <Slider label="×Mult global" value={draft.happy_hour.global_mult} min={1} max={5} disabled={!draft.happy_hour.enabled}
            onChange={(v) => set((r) => void (r.happy_hour.global_mult = v))} />
        </div>
      </div>
      {error && <p role="alert" className="rounded-lg border border-neon-red/40 bg-neon-red/10 px-3 py-2 text-sm text-neon-red">{error}</p>}
      <div className="flex gap-3">
        <NeonButton variant="magenta" loading={saving} disabled={!dirty} onClick={save}>
          Guardar como v{current.version + 1}
        </NeonButton>
        <button type="button" onClick={() => setDraft(structuredClone(current))} disabled={!dirty}
          className="rounded-lg border border-white/15 px-3 text-sm text-slate-300 disabled:opacity-40">
          Descartar cambios
        </button>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ radar */

function Radar({ token }: { token: string }) {
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const [status, setStatus] = useState<'connecting' | 'open' | 'reconnecting'>('connecting');
  const retry = useRef(0);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let stopped = false;
    const open = () => {
      ws = new WebSocket(wsAdminUrl(token));
      ws.onopen = () => {
        retry.current = 0;
        setStatus('open');
      };
      ws.onmessage = (ev) => {
        try {
          setSnap(JSON.parse(String(ev.data)) as Snapshot);
        } catch {
          // mensaje inválido: se ignora
        }
      };
      ws.onclose = () => {
        if (stopped) return;
        setStatus('reconnecting');
        timer = setTimeout(open, Math.min(10_000, 500 * 2 ** retry.current++));
      };
    };
    open();
    return () => {
      stopped = true;
      if (timer) clearTimeout(timer);
      ws?.close();
    };
  }, [token]);

  if (!snap) return <p className="text-slate-400">Conectando al radar… ({status})</p>;
  return (
    <div className="space-y-4">
      <p className="label-caps !text-[0.6rem]">
        {status === 'open' ? 'en vivo' : 'reconectando'} · {new Date(snap.generated_at).toLocaleTimeString('es-CO')}
      </p>
      {snap.alarms.length > 0 && (
        <ul className="space-y-1">
          {snap.alarms.map((a, i) => (
            <li key={i} className={`rounded-lg border px-3 py-1.5 text-sm ${a.severity === 'critical' ? LIGHT.red : LIGHT.yellow}`}>
              {a.severity === 'critical' ? '⛔' : '⚠️'} {a.message}
            </li>
          ))}
        </ul>
      )}
      <ul className="grid gap-2 sm:grid-cols-3 lg:grid-cols-5">
        {snap.containers.map((c) => (
          <li key={c.name} className={`rounded-xl border p-2 ${LIGHT[c.light]}`}>
            <div className="truncate font-mono text-xs" title={c.name}>{c.name.replace('jokerarena-', '')}</div>
            <div className="text-[0.7rem] text-slate-300">
              {c.state === 'running' ? c.health : c.state} · CPU {c.cpu_percent}% · {Math.round(c.mem_bytes / 1048576)} MiB
            </div>
          </li>
        ))}
      </ul>
      <div className="grid gap-3 text-sm sm:grid-cols-3">
        <div className={`rounded-xl border p-3 ${snap.rabbitmq.status === 'ok' ? LIGHT.green : LIGHT.red}`}>
          <b>RabbitMQ</b> · {snap.rabbitmq.publish_rate} msg/s publicados · {snap.rabbitmq.messages_ready} en cola · {snap.rabbitmq.queues.length} colas
        </div>
        <div className={`rounded-xl border p-3 ${snap.postgres.status === 'ok' ? LIGHT.green : LIGHT.red}`}>
          <b>PostgreSQL</b> · {snap.postgres.connections}/{snap.postgres.max_connections} conexiones · {snap.postgres.latency_ms} ms
        </div>
        <div className={`rounded-xl border p-3 ${snap.redis.status === 'ok' ? LIGHT.green : LIGHT.red}`}>
          <b>Redis</b> · {snap.redis.connected_clients} clientes · {snap.redis.latency_ms} ms
        </div>
      </div>
    </div>
  );
}

/* ----------------------------------------------------------------- página */

/** Consola de administración: Editor de balance (LiveOps) y NOC / Radar del clúster. Solo rol admin. */
export function Admin() {
  const player = useSession((s) => s.player);
  if (!player) return <p className="text-slate-400">Inicia sesión como <span className="font-mono">admin</span> para usar la consola.</p>;
  if (player.role !== 'admin') {
    return (
      <p role="alert" className="mx-auto max-w-xl rounded-lg border border-neon-red/40 bg-neon-red/10 px-4 py-3 text-neon-red">
        Esta consola es solo para administradores. Sal y entra con el alias <span className="font-mono">admin</span> y su contraseña.
      </p>
    );
  }
  return (
    <div className="space-y-8">
      <div>
        <p className="label-caps">Administración</p>
        <h1 className="mt-1 font-display text-3xl font-black tracking-wider text-white">Consola de operaciones</h1>
      </div>
      <section className="glass-neon-magenta p-6" aria-labelledby="rules-title">
        <h2 id="rules-title" className="mb-4 font-display text-lg font-bold tracking-wider">Editor de balance (LiveOps)</h2>
        <RulesEditor token={player.token} />
      </section>
      <section className="glass-neon-cyan p-6" aria-labelledby="radar-title">
        <h2 id="radar-title" className="mb-4 font-display text-lg font-bold tracking-wider">NOC · Radar del clúster</h2>
        <Radar token={player.token} />
      </section>
    </div>
  );
}
