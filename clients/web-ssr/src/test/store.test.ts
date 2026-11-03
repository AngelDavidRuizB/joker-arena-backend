import { beforeEach, describe, expect, it } from 'vitest';
import { MAX_SELECTION, isHost, useGame } from '../store/game';
import type { Card, HandScored, MatchFinished, RoomState } from '../lib/types';
import { botAction, room as makeRoom, hand as makeHand, BOT_HAND } from './fixtures';

const cards = (n: number): Card[] =>
  Array.from({ length: n }, (_, i) => ({ rank: String((i % 9) + 2), suit: 'SPADES', chips: (i % 9) + 2 }));

describe('game store', () => {
  beforeEach(() => useGame.getState().reset());

  it('applies ROOM_STATE_UPDATE and clears the selection', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8) }) });
    s.toggleCard(0);
    s.toggleCard(3);
    expect(useGame.getState().selected).toEqual([0, 3]);
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), hands_left: 19 }) });
    expect(useGame.getState().selected).toEqual([]);
    expect(useGame.getState().room?.hands_left).toBe(19);
  });

  it('toggles cards and enforces the maximum of five', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8) }) });
    for (let i = 0; i < 8; i++) s.toggleCard(i);
    expect(useGame.getState().selected).toHaveLength(MAX_SELECTION);
    expect(useGame.getState().selected).toEqual([0, 1, 2, 3, 4]);
    s.toggleCard(2); // deselect
    expect(useGame.getState().selected).toEqual([0, 1, 3, 4]);
    s.toggleCard(7); // room again
    expect(useGame.getState().selected).toEqual([0, 1, 3, 4, 7]);
  });

  it('ignores out-of-range indices and toggles while busy or without a room', () => {
    const s = useGame.getState();
    s.toggleCard(0); // no room yet
    expect(useGame.getState().selected).toEqual([]);
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8) }) });
    s.toggleCard(-1);
    s.toggleCard(8);
    expect(useGame.getState().selected).toEqual([]);
    s.setBusy(true);
    s.toggleCard(1);
    expect(useGame.getState().selected).toEqual([]);
  });

  it('does not allow selecting after the match finished', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), status: 'FINISHED' }) });
    s.toggleCard(0);
    expect(useGame.getState().selected).toEqual([]);
  });

  it('queues state and result messages while the scoring cascade is playing', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), hands_left: 20 }) });
    s.setBusy(true);
    const scored: HandScored = makeHand();
    const finished: MatchFinished = {
      room_code: 'JKR-TEST', match_id: '1', winner_id: '1', result: 'WIN', final_score: 1248,
      target_score: 22000, target_reached: true, reason: 'TARGET_SCORE_REACHED', hands_played: 1,
    };

    s.applyMessage({ type: 'HAND_SCORED', payload: scored });
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), hands_left: 19, current_score: 1248 }) });
    s.applyMessage({ type: 'MATCH_FINISHED', payload: finished });

    let st = useGame.getState();
    expect(st.cascade).toEqual(scored);
    expect(st.history).toHaveLength(1);
    expect(st.room?.hands_left).toBe(20); // old state still shown during the animation
    expect(st.finished).toBeNull();
    expect(st.queued).toHaveLength(2);

    s.endCascade();
    st = useGame.getState();
    expect(st.cascade).toBeNull();
    expect(st.queued).toHaveLength(0);
    expect(st.room?.hands_left).toBe(19);
    expect(st.room?.current_score).toBe(1248);
    expect(st.finished?.result).toBe('WIN');
    expect(st.busy).toBe(false);
  });

  it('queues the bot HAND_SCORED during the host cascade and replays both in order', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), hands_left: 20 }) });

    const hostHand: HandScored = makeHand({ player_id: '1', hand_number: 1 });
    const botHand: HandScored = makeHand({ player_id: 'bot', hand_number: 1, accumulated_score: 900, score: 900 });

    // Per-round order with a bot: HAND_SCORED(host), STATE(host), HAND_SCORED(bot), STATE(bot).
    s.applyMessage({ type: 'HAND_SCORED', payload: hostHand });
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), hands_left: 19, current_score: 1248 }) });
    s.applyMessage({ type: 'HAND_SCORED', payload: botHand });
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), hands_left: 19, current_score: 1248, bot_score: 900 }) });

    let st = useGame.getState();
    // The host cascade is active; the bot HAND_SCORED and both STATEs are held back.
    expect(st.cascade).toEqual(hostHand);
    expect(st.history).toHaveLength(1);
    expect(st.room?.hands_left).toBe(20);
    expect(st.room?.bot_score).toBeUndefined();
    expect(st.queued).toHaveLength(3);

    // First endCascade drains the host STATE, then re-enters the bot HAND_SCORED as the next cascade.
    s.endCascade();
    st = useGame.getState();
    expect(st.cascade).toEqual(botHand);
    expect(st.history).toHaveLength(2);
    expect(st.history.map((h) => h.player_id)).toEqual(['1', 'bot']);
    expect(st.room?.hands_left).toBe(19); // host STATE applied before the bot cascade
    expect(st.queued).toHaveLength(1); // the bot STATE is still pending behind the bot cascade

    // Second endCascade drains the final bot STATE.
    s.endCascade();
    st = useGame.getState();
    expect(st.cascade).toBeNull();
    expect(st.queued).toHaveLength(0);
    expect(st.room?.bot_score).toBe(900);
    expect(st.room?.current_score).toBe(1248);
  });

  it('queues a BOT_ACTION(PLAY) during the host cascade, then drives botTurn and the bot cascade in order', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND }) });

    const hostHand: HandScored = makeHand({ player_id: '1', hand_number: 1 });
    const botHand: HandScored = makeHand({ player_id: 'bot', hand_number: 1, accumulated_score: 900, score: 900 });

    // Per-round order: HAND_SCORED(host), STATE(host), BOT_ACTION, HAND_SCORED(bot), STATE(bot).
    s.applyMessage({ type: 'HAND_SCORED', payload: hostHand });
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND, hands_left: 19 }) });
    s.applyMessage({ type: 'BOT_ACTION', payload: botAction({ action: 'PLAY', card_indices: [0, 1] }) });
    s.applyMessage({ type: 'HAND_SCORED', payload: botHand });
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND, hands_left: 19, bot_score: 900 }) });

    let st = useGame.getState();
    // The host cascade is active; the BOT_ACTION and everything behind it are held back.
    expect(st.cascade).toEqual(hostHand);
    expect(st.botTurn).toBeNull();
    expect(st.queued).toHaveLength(4);

    // Drain the host cascade: host STATE applies, then BOT_ACTION opens the visible bot turn.
    s.endCascade();
    st = useGame.getState();
    expect(st.cascade).toBeNull();
    expect(st.botTurn).toEqual({ action: 'PLAY', indices: [0, 1], handNumber: 1 });
    expect(st.botPhase).toBe('reveal');
    expect(st.room?.hands_left).toBe(19);
    // The bot HAND_SCORED and STATE are still queued behind the BOT_ACTION.
    expect(st.queued).toHaveLength(2);

    // The view advances reveal -> select, then hands off to the queued bot cascade.
    s.advanceBotPhase();
    expect(useGame.getState().botPhase).toBe('select');
    s.advanceBotPhase();
    expect(useGame.getState().botPhase).toBe('resolve');
    expect(useGame.getState().botTurn).not.toBeNull();

    // Now the queued bot HAND_SCORED must become the active cascade. endCascade drains it.
    s.endCascade();
    st = useGame.getState();
    expect(st.cascade).toEqual(botHand);
    expect(st.history.map((h) => h.player_id)).toEqual(['1', 'bot']);
    expect(st.botTurn).not.toBeNull(); // still the bot's turn until the bot cascade ends

    // The bot cascade ends: botTurn clears and the final bot STATE applies.
    s.endCascade();
    st = useGame.getState();
    expect(st.cascade).toBeNull();
    expect(st.botTurn).toBeNull();
    expect(st.botPhase).toBe('idle');
    expect(st.room?.bot_score).toBe(900);
    expect(st.queued).toHaveLength(0);
  });

  it('drives a bot PLAY end-to-end through the view path (phase advances + queue drain, no manual endCascade past resolve)', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND }) });

    const hostHand: HandScored = makeHand({ player_id: '1', hand_number: 1 });
    const botHand: HandScored = makeHand({ player_id: 'bot', hand_number: 1, accumulated_score: 900, score: 900 });

    s.applyMessage({ type: 'HAND_SCORED', payload: hostHand });
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND, hands_left: 19 }) });
    s.applyMessage({ type: 'BOT_ACTION', payload: botAction({ action: 'PLAY', card_indices: [0, 1] }) });
    s.applyMessage({ type: 'HAND_SCORED', payload: botHand });
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND, hands_left: 19, bot_score: 900 }) });

    // The host cascade finishes (view: onCascadeDone -> endCascade): host STATE applies, BOT_ACTION opens the turn.
    s.endCascade();
    let st = useGame.getState();
    expect(st.cascade).toBeNull();
    expect(st.botTurn).toEqual({ action: 'PLAY', indices: [0, 1], handNumber: 1 });
    expect(st.botPhase).toBe('reveal');
    expect(st.queued).toHaveLength(2); // bot HAND_SCORED + bot STATE held behind the BOT_ACTION

    // The view's phase timer advances reveal -> select -> resolve (no cascade mounted during these).
    s.advanceBotPhase();
    expect(useGame.getState().botPhase).toBe('select');
    s.advanceBotPhase();
    expect(useGame.getState().botPhase).toBe('resolve');

    // The view's drain effect fires on 'resolve' (cascade===null, queue non-empty): drainQueue, NOT endCascade.
    s.drainQueue();
    st = useGame.getState();
    expect(st.cascade).toEqual(botHand); // the queued bot HAND_SCORED is now the active cascade
    expect(st.history.map((h) => h.player_id)).toEqual(['1', 'bot']);
    expect(st.botTurn).not.toBeNull(); // still the bot's turn while its cascade plays
    expect(st.queued).toHaveLength(1); // the final bot STATE is still pending

    // The bot cascade ends (view: onCascadeDone -> endCascade): botTurn clears, the final STATE applies.
    s.endCascade();
    st = useGame.getState();
    expect(st.cascade).toBeNull();
    expect(st.botTurn).toBeNull();
    expect(st.botPhase).toBe('idle');
    expect(st.room?.bot_score).toBe(900);
    expect(st.queued).toHaveLength(0);
  });

  it('drives a bot DISCARD end-to-end through the view path: the queued refilled STATE drains after the turn closes', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND }) });

    const hostHand: HandScored = makeHand({ player_id: '1', hand_number: 1 });
    const refilled: Card[] = [...BOT_HAND];
    refilled[2] = { rank: '7', suit: 'SPADES', chips: 7 };

    // Per-round order with a discard: HAND_SCORED(host), STATE(host), BOT_ACTION(DISCARD), STATE(bot refill).
    s.applyMessage({ type: 'HAND_SCORED', payload: hostHand });
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND, hands_left: 19 }) });
    s.applyMessage({ type: 'BOT_ACTION', payload: botAction({ action: 'DISCARD', card_indices: [2, 3] }) });
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: refilled, bot_discards_left: 2 }) });

    // Host cascade finishes: host STATE applies, BOT_ACTION opens the discard turn, the refill STATE stays queued.
    s.endCascade();
    let st = useGame.getState();
    expect(st.botTurn).toEqual({ action: 'DISCARD', indices: [2, 3], handNumber: 1 });
    expect(st.botPhase).toBe('reveal');
    expect(st.room?.bot_hand?.[2].rank).toBe('K'); // still the old hand during the turn
    expect(st.queued).toHaveLength(1); // the refilled STATE is held behind the BOT_ACTION

    // reveal -> select -> close (advanceBotPhase on DISCARD clears the turn to idle).
    s.advanceBotPhase();
    expect(useGame.getState().botPhase).toBe('select');
    s.advanceBotPhase();
    st = useGame.getState();
    expect(st.botTurn).toBeNull();
    expect(st.botPhase).toBe('idle');

    // The view's drain effect fires now that no cascade/turn carries the queue: the refilled bot hand shows.
    s.drainQueue();
    st = useGame.getState();
    expect(st.room?.bot_hand?.[2].rank).toBe('7');
    expect(st.room?.bot_discards_left).toBe(2);
    expect(st.queued).toHaveLength(0);
  });

  it('sets botTurn for a BOT_ACTION(DISCARD), updates the bot hand from the next STATE and clears the turn', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND }) });

    // No active cascade here, so BOT_ACTION applies immediately.
    s.applyMessage({ type: 'BOT_ACTION', payload: botAction({ action: 'DISCARD', card_indices: [2, 3] }) });
    let st = useGame.getState();
    expect(st.botTurn).toEqual({ action: 'DISCARD', indices: [2, 3], handNumber: 1 });
    expect(st.botPhase).toBe('reveal');

    // reveal -> select highlights the discarded cards.
    s.advanceBotPhase();
    expect(useGame.getState().botPhase).toBe('select');

    // After the select hold, DISCARD closes the turn so the refilled bot hand can show.
    s.advanceBotPhase();
    st = useGame.getState();
    expect(st.botTurn).toBeNull();
    expect(st.botPhase).toBe('idle');

    const refilled: Card[] = [...BOT_HAND];
    refilled[2] = { rank: '7', suit: 'SPADES', chips: 7 };
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: refilled, bot_discards_left: 2 }) });
    st = useGame.getState();
    expect(st.room?.bot_hand?.[2].rank).toBe('7');
    expect(st.room?.bot_discards_left).toBe(2);
  });

  it('ignores a BOT_ACTION when with_bot is false or bot_hand is missing', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), with_bot: false, bot_hand: undefined }) });
    s.applyMessage({ type: 'BOT_ACTION', payload: botAction() });
    expect(useGame.getState().botTurn).toBeNull();
    expect(useGame.getState().botPhase).toBe('idle');

    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), with_bot: true, bot_hand: [] }) });
    s.applyMessage({ type: 'BOT_ACTION', payload: botAction() });
    expect(useGame.getState().botTurn).toBeNull();
  });

  it('locks host card selection while the bot turn is active', () => {
    const s = useGame.getState();
    s.applyMessage({ type: 'ROOM_STATE_UPDATE', payload: makeRoom({ cards_in_hand: cards(8), bot_hand: BOT_HAND }) });
    s.applyMessage({ type: 'BOT_ACTION', payload: botAction({ action: 'DISCARD', card_indices: [0] }) });
    s.toggleCard(0);
    expect(useGame.getState().selected).toEqual([]);
  });

  it('stores server errors and releases the busy flag', () => {
    const s = useGame.getState();
    s.setBusy(true);
    s.applyMessage({ type: 'ERROR', payload: { code: 'INVALID_SELECTION', message: 'card index 99 is out of range' } });
    const st = useGame.getState();
    expect(st.error?.code).toBe('INVALID_SELECTION');
    expect(st.busy).toBe(false);
    s.dismissError();
    expect(useGame.getState().error).toBeNull();
  });

  it('identifies the host of the room', () => {
    const r: RoomState = makeRoom({ host_id: '7' });
    expect(isHost(r, '7')).toBe(true);
    expect(isHost(r, '8')).toBe(false);
    expect(isHost(null, '7')).toBe(false);
    expect(isHost(r, undefined)).toBe(false);
  });
});
