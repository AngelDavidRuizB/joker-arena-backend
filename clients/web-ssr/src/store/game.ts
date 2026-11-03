import { create } from 'zustand';
import { BOT_PLAYER_ID } from '../lib/constants';
import type { ConnectionStatus, HandScored, MatchFinished, RoomState, ServerMessage } from '../lib/types';

export const MAX_SELECTION = 5;

/** How long the bot hand is revealed before its chosen cards light up. */
export const REVEAL_MS = 700;
/** How long the highlighted selection is held before the play/discard resolves. */
export const SELECT_HOLD_MS = 900;

/** The bot's announced move, driving the on-felt animation during its turn. */
export interface BotTurn {
  action: 'PLAY' | 'DISCARD';
  indices: number[];
  handNumber: number;
}

export type BotPhase = 'idle' | 'reveal' | 'select' | 'resolve';

export interface GameState {
  connection: ConnectionStatus;
  room: RoomState | null;
  /** Indices (into room.cards_in_hand) of the cards the user picked. */
  selected: number[];
  /** The hand being animated right now; server follow-ups are queued until it ends. */
  cascade: HandScored | null;
  history: HandScored[];
  finished: MatchFinished | null;
  /** True while an action was sent and the server has not answered yet. */
  busy: boolean;
  /** Last server/transport error shown to the user. */
  error: { code: string; message: string } | null;
  queued: ServerMessage[];
  /** The bot's current move while its turn is visible on the felt; null when idle. */
  botTurn: BotTurn | null;
  /** Progression of the bot turn animation: reveal hand -> highlight -> resolve. */
  botPhase: BotPhase;

  reset: () => void;
  setConnection: (status: ConnectionStatus) => void;
  applyMessage: (msg: ServerMessage) => void;
  toggleCard: (index: number) => void;
  clearSelection: () => void;
  setBusy: (busy: boolean) => void;
  endCascade: () => void;
  drainQueue: () => void;
  advanceBotPhase: () => void;
  dismissError: () => void;
}

const initial = {
  connection: 'idle' as ConnectionStatus,
  room: null,
  selected: [],
  cascade: null,
  history: [],
  finished: null,
  busy: false,
  error: null,
  queued: [],
  botTurn: null,
  botPhase: 'idle' as BotPhase,
};

export const useGame = create<GameState>((set, get) => ({
  ...initial,

  reset: () => set({ ...initial }),

  setConnection: (connection) => set({ connection }),

  applyMessage: (msg) => {
    const state = get();
    // While a scoring cascade is animating, hold back every follow-up (state, finish and
    // the bot's own HAND_SCORED) so the player sees the active cascade to the end before the
    // old cards change or the next cascade plays. endCascade drains the queue one by one.
    if (
      state.cascade &&
      (msg.type === 'ROOM_STATE_UPDATE' ||
        msg.type === 'MATCH_FINISHED' ||
        msg.type === 'HAND_SCORED' ||
        msg.type === 'BOT_ACTION')
    ) {
      set({ queued: [...state.queued, msg] });
      return;
    }
    switch (msg.type) {
      case 'ROOM_STATE_UPDATE':
        set({ room: msg.payload, selected: [], busy: false, error: null });
        break;
      case 'HAND_SCORED':
        set({
          cascade: msg.payload,
          history: [...state.history, msg.payload],
          selected: [],
        });
        break;
      case 'MATCH_FINISHED':
        set({ finished: msg.payload, busy: false });
        break;
      case 'BOT_ACTION': {
        // Open the bot's visible turn: show its hand, then highlight the chosen cards.
        // Guard against matches without a bot or a missing bot hand (safe no-op).
        const { room } = state;
        if (!room?.with_bot || !room.bot_hand || room.bot_hand.length === 0) break;
        set({
          botTurn: {
            action: msg.payload.action,
            indices: msg.payload.card_indices,
            handNumber: msg.payload.hand_number,
          },
          botPhase: 'reveal',
        });
        break;
      }
      case 'ERROR':
        set({ error: msg.payload, busy: false });
        break;
    }
  },

  toggleCard: (index) => {
    const { selected, room, busy, cascade, finished, botTurn } = get();
    if (busy || cascade || finished || botTurn || !room || room.status === 'FINISHED') return;
    if (index < 0 || index >= room.cards_in_hand.length) return;
    if (selected.includes(index)) {
      set({ selected: selected.filter((i) => i !== index) });
    } else if (selected.length < MAX_SELECTION) {
      set({ selected: [...selected, index] });
    }
  },

  clearSelection: () => set({ selected: [] }),

  setBusy: (busy) => set({ busy }),

  endCascade: () => {
    // The active cascade finished playing. When it belonged to the bot, its visible turn is over.
    const { cascade } = get();
    const wasBot = cascade?.player_id === BOT_PLAYER_ID;
    set({ cascade: null, ...(wasBot ? { botTurn: null, botPhase: 'idle' as BotPhase } : {}) });
    get().drainQueue();
  },

  drainQueue: () => {
    // Drain the held-back messages one at a time. A queued bot HAND_SCORED re-enters applyMessage
    // and sets the next cascade, which must re-queue every message still pending behind it — so we
    // take the head and keep the tail in the store before replaying. Stop as soon as a cascade opens
    // or a *new* visible bot turn opens: the view animates it and calls back here (endCascade /
    // drainQueue) to continue. A bot turn already open before draining (the PLAY 'resolve' hand-off)
    // does not stop the drain — the bot HAND_SCORED must become the active cascade under that turn.
    // Called from endCascade and whenever the view resolves a bot phase that leaves a queued message
    // with no active cascade to carry it (bot PLAY 'resolve', closed DISCARD turn).
    const openTurnBefore = get().botTurn;
    const { queued } = get();
    for (let i = 0; i < queued.length; i++) {
      set({ queued: queued.slice(i + 1) });
      get().applyMessage(queued[i]);
      if (get().cascade || get().botTurn !== openTurnBefore) break;
    }
  },

  advanceBotPhase: () => {
    const { botPhase, botTurn } = get();
    if (!botTurn) return;
    if (botPhase === 'reveal') {
      // The hand has been shown; now light up the chosen cards.
      set({ botPhase: 'select' });
      return;
    }
    if (botPhase === 'select') {
      if (botTurn.action === 'PLAY') {
        // Hand off to the queued bot HAND_SCORED cascade; botTurn clears on its endCascade.
        set({ botPhase: 'resolve' });
      } else {
        // DISCARD: let the next bot ROOM_STATE_UPDATE (new bot_hand) apply, then close the turn.
        set({ botTurn: null, botPhase: 'idle' });
      }
    }
  },

  dismissError: () => set({ error: null }),
}));

/** True when the signed-in player owns the match (only the host can play). */
export const isHost = (room: RoomState | null, playerId: string | undefined): boolean =>
  !!room && !!playerId && room.host_id === playerId;
