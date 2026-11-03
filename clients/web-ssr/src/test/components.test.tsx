import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it } from 'vitest';
import { CardFan } from '../components/CardFan';
import { AuditPanel } from '../components/AuditPanel';
import { BotScoreboard } from '../components/BotScoreboard';
import { Hud } from '../components/Hud';
import { JokerBar } from '../components/JokerBar';
import { ScoreCascade } from '../components/ScoreCascade';
import { useGame } from '../store/game';
import type { Card } from '../lib/types';
import { hand, room } from './fixtures';

const HAND: Card[] = [
  { rank: 'A', suit: 'SPADES', chips: 11 },
  { rank: 'K', suit: 'HEARTS', chips: 10 },
  { rank: '10', suit: 'CLUBS', chips: 10 },
  { rank: '7', suit: 'DIAMONDS', chips: 7 },
  { rank: '5', suit: 'SPADES', chips: 5 },
  { rank: '4', suit: 'HEARTS', chips: 4 },
  { rank: '3', suit: 'CLUBS', chips: 3 },
  { rank: '2', suit: 'DIAMONDS', chips: 2 },
];

function Harness({ disabled = false }: { disabled?: boolean }) {
  const selected = useGame((s) => s.selected);
  const toggle = useGame((s) => s.toggleCard);
  return <CardFan cards={HAND} selected={selected} disabled={disabled} onToggle={toggle} />;
}

describe('card hand (F3: selección interactiva)', () => {
  beforeEach(() => {
    useGame.getState().reset();
    useGame.getState().applyMessage({ type: 'ROOM_STATE_UPDATE', payload: room({ cards_in_hand: HAND }) });
  });

  it('renders the eight cards with accessible names', () => {
    render(<Harness />);
    expect(screen.getAllByRole('button')).toHaveLength(8);
    expect(screen.getByRole('button', { name: 'As de picas, 11 fichas' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Rey de corazones, 10 fichas' })).toBeInTheDocument();
  });

  it('marks cards as selected with aria-pressed and deselects on a second click', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const ace = screen.getByRole('button', { name: /^As de picas/ });
    expect(ace).toHaveAttribute('aria-pressed', 'false');
    await user.click(ace);
    expect(ace).toHaveAttribute('aria-pressed', 'true');
    await user.click(ace);
    expect(ace).toHaveAttribute('aria-pressed', 'false');
  });

  it('allows at most five selected cards', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    for (const b of screen.getAllByRole('button')) await user.click(b);
    expect(screen.getAllByRole('button', { pressed: true })).toHaveLength(5);
  });

  it('supports keyboard selection (Space/Enter on a focused card)', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.tab();
    await user.keyboard(' ');
    expect(screen.getAllByRole('button', { pressed: true })).toHaveLength(1);
    await user.tab();
    await user.keyboard('{Enter}');
    expect(screen.getAllByRole('button', { pressed: true })).toHaveLength(2);
  });

  it('does not select when disabled (spectator or waiting for the server)', async () => {
    const user = userEvent.setup();
    render(<Harness disabled />);
    await user.click(screen.getByRole('button', { name: /^As de picas/ }));
    expect(screen.queryAllByRole('button', { pressed: true })).toHaveLength(0);
  });
});

describe('read-only CardFan (bot turn)', () => {
  it('renders the bot hand with the chosen indices highlighted and non-interactive', async () => {
    const user = userEvent.setup();
    render(<CardFan cards={HAND} selected={[0, 1]} disabled onToggle={() => {}} readOnly />);

    expect(screen.getByRole('group', { name: 'Mano del bot' })).toBeInTheDocument();
    const ace = screen.getByRole('button', { name: /^As de picas/ });
    const king = screen.getByRole('button', { name: /^Rey de corazones/ });
    // The two chosen cards show the selected visual via aria-pressed.
    expect(ace).toHaveAttribute('aria-pressed', 'true');
    expect(king).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getAllByRole('button', { pressed: true })).toHaveLength(2);

    // Clicking a card does nothing: the hand is read-only.
    await user.click(ace);
    expect(ace).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getAllByRole('button', { pressed: true })).toHaveLength(2);
  });
});

describe('HUD', () => {
  it('shows score progress, hands and discards', () => {
    render(<Hud room={room({ current_score: 11000, target_score: 22000, hands_left: 19, discards_left: 2, deck_count: 39 })} />);
    const bar = screen.getByRole('progressbar', { name: /progreso hacia la meta/i });
    expect(bar).toHaveAttribute('aria-valuenow', '11000');
    expect(bar).toHaveAttribute('aria-valuemax', '22000');
    expect(screen.getByText('50% de la meta')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Manos restantes: 19 de 20' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Descartes restantes: 2 de 3' })).toBeInTheDocument();
    expect(screen.getByText('39 en mazo')).toBeInTheDocument();
    // 20 hands render as a progress bar, only the 3 discards use pips.
    expect(screen.getAllByTestId('pip')).toHaveLength(3);
    expect(screen.getByTestId('pips-bar')).toBeInTheDocument();
  });

  it('caps the progress at 100% when the target is exceeded', () => {
    render(<Hud room={room({ current_score: 25000, target_score: 22000 })} />);
    expect(screen.getByText('100% de la meta')).toBeInTheDocument();
  });

  it.each([undefined, 0, -3])('falls back to 20 hands when hands_total is %s', (total) => {
    render(<Hud room={room({ hands_left: 20, hands_total: total })} />);
    expect(screen.getByRole('img', { name: 'Manos restantes: 20 de 20' })).toBeInTheDocument();
    expect(screen.getByTestId('pips-bar').innerHTML).not.toContain('NaN');
  });

  it('keeps pips for small totals (older server)', () => {
    render(<Hud room={room({ hands_left: 4, hands_total: 4, discards_total: 3 })} />);
    expect(screen.getAllByTestId('pip')).toHaveLength(7);
    expect(screen.queryByTestId('pips-bar')).toBeNull();
  });
});

describe('Jokers and audit', () => {
  it('lists the three jokers in evaluation order', () => {
    render(<JokerBar owned={['banner', 'joker', 'the_duo']} />);
    const names = screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent);
    expect(names).toEqual(['Banner', 'Joker Clásico', 'The Duo']);
  });

  it('prints the exact formula for the specification example (1248)', () => {
    render(<AuditPanel history={[hand()]} />);
    expect(screen.getByText('(40 + 38 + 0) × ((4 + 4) × 2) = 78 × 16 = 1248')).toBeInTheDocument();
    expect(screen.getByText('#1 · Full House')).toBeInTheDocument();
  });

  it('shows an empty state before any hand is played', () => {
    render(<AuditPanel history={[]} />);
    expect(screen.getByText(/juega una mano/i)).toBeInTheDocument();
  });

  it('labels each audit entry as Anfitrión or Bot from the player_id', () => {
    render(
      <AuditPanel
        history={[
          hand({ player_id: '1', hand_number: 1, timestamp: '2026-10-12T19:40:00Z' }),
          hand({ player_id: 'bot', hand_number: 1, timestamp: '2026-10-12T19:40:01Z' }),
        ]}
      />,
    );
    expect(screen.getByText('Anfitrión')).toBeInTheDocument();
    expect(screen.getByText('Bot')).toBeInTheDocument();
  });
});

describe('Score cascade author badge', () => {
  it('shows the host label for a host hand', () => {
    render(<ScoreCascade hand={hand({ player_id: '1' })} authorLabel="Host" onDone={() => {}} />);
    expect(screen.getByTestId('cascade-author')).toHaveTextContent('Host');
  });

  it('shows the Bot label for a player_id="bot" hand', () => {
    render(<ScoreCascade hand={hand({ player_id: 'bot' })} authorLabel="Host" onDone={() => {}} />);
    expect(screen.getByTestId('cascade-author')).toHaveTextContent('Bot');
  });
});

describe('Bot scoreboard', () => {
  it('shows the bot score, hands, discards and progress when with_bot', () => {
    render(<BotScoreboard room={room({ with_bot: true, bot_score: 11000, bot_hands_left: 18, bot_discards_left: 2, target_score: 22000 })} />);
    const bar = screen.getByRole('progressbar', { name: /progreso del bot/i });
    expect(bar).toHaveAttribute('aria-valuenow', '11000');
    expect(bar).toHaveAttribute('aria-valuemax', '22000');
    expect(screen.getByText('50% de la meta')).toBeInTheDocument();
    expect(screen.getByText('Marcador del bot')).toBeInTheDocument();
  });

  it('renders nothing when the room has no bot', () => {
    const { container } = render(<BotScoreboard room={room({ with_bot: false })} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('tolerates missing bot_* fields with a zero fallback', () => {
    const r = room({ with_bot: true, target_score: 22000 });
    delete r.bot_score;
    delete r.bot_hands_left;
    delete r.bot_discards_left;
    render(<BotScoreboard room={r} />);
    expect(screen.getByText('0% de la meta')).toBeInTheDocument();
    expect(screen.getByRole('progressbar', { name: /progreso del bot/i })).toHaveAttribute('aria-valuenow', '0');
  });
});
