import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ConnectionStatus, ServerMessage } from '../lib/types';
import { GameSocket, MAX_RECONNECT_ATTEMPTS, backoffDelay, buildWsUrl } from '../ws/gameSocket';

class FakeSocket {
  static instances: FakeSocket[] = [];
  readyState = 0;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  constructor(public url: string) {
    FakeSocket.instances.push(this);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.readyState = 3;
    this.onclose?.();
  }
  // test helpers
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  drop() {
    this.readyState = 3;
    this.onclose?.();
  }
  receive(msg: ServerMessage) {
    this.onmessage?.({ data: JSON.stringify(msg) });
  }
}

function setup() {
  const statuses: ConnectionStatus[] = [];
  const messages: ServerMessage[] = [];
  const socket = new GameSocket({
    roomCode: 'JKR-ABCD',
    playerId: '7',
    token: 't0k',
    url: 'ws://test/ws/JKR-ABCD',
    createSocket: (u) => new FakeSocket(u) as unknown as WebSocket,
    onStatus: (s) => statuses.push(s),
    onMessage: (m) => messages.push(m),
  });
  return { socket, statuses, messages };
}

describe('GameSocket', () => {
  beforeEach(() => {
    FakeSocket.instances = [];
    vi.useFakeTimers();
  });
  afterEach(() => vi.useRealTimers());

  it('builds same-origin ws/wss URLs', () => {
    expect(buildWsUrl('JKR-ABCD', 't0k', { protocol: 'http:', hostname: 'localhost' }, '')).toBe('ws://localhost:8080/ws/JKR-ABCD?token=t0k');
    expect(buildWsUrl('JKR-ABCD', 't0k', { protocol: 'https:', hostname: 'game.example' }, '')).toBe('wss://game.example:8080/ws/JKR-ABCD?token=t0k');
    expect(buildWsUrl('JKR-ABCD', 'a b', { protocol: 'http:', hostname: 'x' }, 'ws://gw:9000/')).toBe('ws://gw:9000/ws/JKR-ABCD?token=a%20b');
  });

  it('computes exponential backoff with a cap', () => {
    expect([0, 1, 2, 3, 4].map(backoffDelay)).toEqual([500, 1000, 2000, 4000, 8000]);
    expect(backoffDelay(5)).toBe(10_000);
    expect(backoffDelay(50)).toBe(10_000);
  });

  it('sends JOIN_ROOM on open and reports statuses', () => {
    const { socket, statuses } = setup();
    socket.connect();
    const ws = FakeSocket.instances[0];
    ws.open();
    expect(statuses).toEqual(['connecting', 'open']);
    expect(JSON.parse(ws.sent[0])).toEqual({
      type: 'JOIN_ROOM',
      payload: { room_code: 'JKR-ABCD', player_id: '7' },
    });
  });

  it('forwards server messages and ignores malformed ones', () => {
    const { socket, messages } = setup();
    socket.connect();
    const ws = FakeSocket.instances[0];
    ws.open();
    ws.onmessage?.({ data: 'not json' });
    ws.receive({ type: 'ERROR', payload: { code: 'INVALID_SELECTION', message: 'x' } });
    expect(messages).toHaveLength(1);
    expect(messages[0].type).toBe('ERROR');
  });

  it('sends play/discard only while open', () => {
    const { socket } = setup();
    socket.connect();
    const ws = FakeSocket.instances[0];
    expect(socket.send('PLAY_HAND', [0, 1])).toBe(false);
    ws.open();
    expect(socket.send('PLAY_HAND', [0, 1])).toBe(true);
    expect(socket.send('DISCARD_CARDS', [2])).toBe(true);
    expect(JSON.parse(ws.sent[1])).toEqual({ type: 'PLAY_HAND', payload: { card_indices: [0, 1] } });
    expect(JSON.parse(ws.sent[2])).toEqual({ type: 'DISCARD_CARDS', payload: { card_indices: [2] } });
  });

  it('reconnects with backoff after an unexpected drop and re-joins', () => {
    const { socket, statuses } = setup();
    socket.connect();
    FakeSocket.instances[0].open();
    FakeSocket.instances[0].drop();
    expect(statuses.at(-1)).toBe('reconnecting');
    expect(FakeSocket.instances).toHaveLength(1);

    vi.advanceTimersByTime(499);
    expect(FakeSocket.instances).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(FakeSocket.instances).toHaveLength(2);

    FakeSocket.instances[1].open();
    expect(statuses.at(-1)).toBe('open');
    expect(JSON.parse(FakeSocket.instances[1].sent[0]).type).toBe('JOIN_ROOM');
  });

  it('gives up after the maximum number of attempts', () => {
    const { socket, statuses } = setup();
    socket.connect();
    for (let i = 0; i <= MAX_RECONNECT_ATTEMPTS; i++) {
      FakeSocket.instances.at(-1)!.drop();
      vi.advanceTimersByTime(10_000);
    }
    expect(statuses.at(-1)).toBe('closed');
  });

  it('does not reconnect after a fatal server error', () => {
    const { socket, statuses } = setup();
    socket.connect();
    const ws = FakeSocket.instances[0];
    ws.open();
    ws.receive({ type: 'ERROR', payload: { code: 'ROOM_NOT_FOUND', message: 'room not found' } });
    ws.drop();
    vi.advanceTimersByTime(60_000);
    expect(FakeSocket.instances).toHaveLength(1);
    expect(statuses.at(-1)).toBe('closed');
  });

  it('does not reconnect after close()', () => {
    const { socket, statuses } = setup();
    socket.connect();
    FakeSocket.instances[0].open();
    socket.close();
    vi.advanceTimersByTime(60_000);
    expect(FakeSocket.instances).toHaveLength(1);
    expect(statuses.at(-1)).toBe('closed');
  });
});
