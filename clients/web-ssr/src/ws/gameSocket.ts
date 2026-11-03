import type { ConnectionStatus, ServerMessage } from '../lib/types';

/** Server error codes after which reconnecting is pointless. */
export const FATAL_ERROR_CODES = new Set(['ROOM_NOT_FOUND', 'UNKNOWN_PLAYER', 'ROOM_FULL', 'NOT_JOINED', 'UNAUTHORIZED', 'MATCH_FINISHED']);

export const MAX_RECONNECT_ATTEMPTS = 8;

/** Exponential backoff: 0.5s, 1s, 2s, 4s, 8s, then capped at 10s. */
export function backoffDelay(attempt: number): number {
  return Math.min(10_000, 500 * 2 ** Math.max(0, attempt));
}

/**
 * URL del WebSocket del juego. El navegador se conecta directo al gateway (Next no hace
 * proxy de WebSockets): NEXT_PUBLIC_GATEWAY_WS_URL o, por defecto, el puerto 8080 del mismo
 * host. El JWT viaja en ?token= porque los navegadores no permiten cabeceras en WebSocket.
 */
export function buildWsUrl(
  roomCode: string,
  token: string,
  loc: Pick<Location, 'protocol' | 'hostname'> = window.location,
  base: string | undefined = process.env.NEXT_PUBLIC_GATEWAY_WS_URL,
): string {
  const origin = base && base.length > 0 ? base.replace(/\/$/, '') : `${loc.protocol === 'https:' ? 'wss:' : 'ws:'}//${loc.hostname}:8080`;
  return `${origin}/ws/${encodeURIComponent(roomCode)}?token=${encodeURIComponent(token)}`;
}

export interface GameSocketOptions {
  roomCode: string;
  playerId: string;
  token: string;
  onMessage: (msg: ServerMessage) => void;
  onStatus: (status: ConnectionStatus) => void;
  /** Injectable for tests. */
  createSocket?: (url: string) => WebSocket;
  url?: string;
}

/**
 * Same-origin WebSocket client for a room (connector C2). It sends JOIN_ROOM on every
 * (re)connection and reconnects with exponential backoff unless the server rejected the
 * join with a fatal error or the caller closed it.
 */
export class GameSocket {
  private ws: WebSocket | null = null;
  private attempt = 0;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private intentionalClose = false;
  private fatal = false;
  private readonly opts: GameSocketOptions;

  constructor(opts: GameSocketOptions) {
    this.opts = opts;
  }

  connect(): void {
    this.intentionalClose = false;
    this.fatal = false;
    this.attempt = 0;
    this.open('connecting');
  }

  private open(status: ConnectionStatus): void {
    this.opts.onStatus(status);
    const url = this.opts.url ?? buildWsUrl(this.opts.roomCode, this.opts.token);
    const ws = (this.opts.createSocket ?? ((u: string) => new WebSocket(u)))(url);
    this.ws = ws;

    ws.onopen = () => {
      this.attempt = 0;
      ws.send(
        JSON.stringify({
          type: 'JOIN_ROOM',
          payload: { room_code: this.opts.roomCode, player_id: this.opts.playerId },
        }),
      );
      this.opts.onStatus('open');
    };

    ws.onmessage = (ev: MessageEvent) => {
      let msg: ServerMessage;
      try {
        msg = JSON.parse(String(ev.data)) as ServerMessage;
      } catch {
        return;
      }
      if (msg.type === 'ERROR' && FATAL_ERROR_CODES.has(msg.payload.code)) this.fatal = true;
      this.opts.onMessage(msg);
    };

    ws.onerror = () => {
      // The close event that follows decides what to do.
    };

    ws.onclose = () => {
      if (this.ws !== ws) return; // a newer socket replaced this one
      this.ws = null;
      if (this.intentionalClose || this.fatal) {
        this.opts.onStatus('closed');
        return;
      }
      if (this.attempt >= MAX_RECONNECT_ATTEMPTS) {
        this.opts.onStatus('closed');
        return;
      }
      const delay = backoffDelay(this.attempt++);
      this.opts.onStatus('reconnecting');
      this.timer = setTimeout(() => this.open('reconnecting'), delay);
    };
  }

  /** Sends a client message; returns false when the socket is not open. */
  send(type: 'PLAY_HAND' | 'DISCARD_CARDS', cardIndices: number[]): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return false;
    this.ws.send(JSON.stringify({ type, payload: { card_indices: cardIndices } }));
    return true;
  }

  close(): void {
    this.intentionalClose = true;
    if (this.timer) clearTimeout(this.timer);
    this.timer = null;
    const ws = this.ws;
    this.ws = null;
    if (ws) {
      ws.onclose = null;
      ws.close();
    }
    this.opts.onStatus('closed');
  }
}
