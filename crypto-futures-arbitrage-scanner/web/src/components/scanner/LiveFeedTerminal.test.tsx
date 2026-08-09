import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { FeedEvent } from '../../app/types';
import { LiveFeedTerminal } from './LiveFeedTerminal';

const events: FeedEvent[] = [
  {
    id: 'quote-2', kind: 'quote', source: 'gate_spot', symbol: 'COTIUSDT',
    timestamp: 20_000, receivedAt: 20_000, bestBid: 0.011032, bestAsk: 0.01104,
  },
  {
    id: 'connection-1', kind: 'connection', source: 'kucoin_spot', symbols: ['COTIUSDT'],
    timestamp: 19_000, receivedAt: 19_000, connected: true,
  },
];

describe('LiveFeedTerminal', () => {
  it('renders selected-pair feed activity and can be collapsed', () => {
    const onCollapsedChange = vi.fn();
    render(<LiveFeedTerminal collapsed={false} events={events} onCollapsedChange={onCollapsedChange} symbol="COTIUSDT" />);

    expect(screen.getByText('Gate.io Spot')).toBeInTheDocument();
    expect(screen.getByText('KuCoin Spot')).toBeInTheDocument();
    expect(screen.getByText(/bid 0\.01103200/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Collapse live feed terminal' }));
    expect(onCollapsedChange).toHaveBeenCalledWith(true);
  });

  it('clears existing rows while keeping the terminal ready for new activity', () => {
    render(<LiveFeedTerminal collapsed={false} events={events} onCollapsedChange={vi.fn()} symbol="COTIUSDT" />);

    fireEvent.click(screen.getByRole('button', { name: 'Clear live feed terminal' }));

    expect(screen.queryByText('Gate.io Spot')).not.toBeInTheDocument();
    expect(screen.getByText('Waiting for the next feed event…')).toBeInTheDocument();
  });
});
