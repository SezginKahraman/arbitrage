import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

vi.mock('lightweight-charts', () => ({
  ColorType: { Solid: 'solid' },
  LineSeries: {},
  createChart: () => ({
    addSeries: () => ({ setData: () => undefined }),
    remove: () => undefined,
    timeScale: () => ({ fitContent: () => undefined }),
  }),
}));

vi.mock('../hooks/useOpportunityHistory', () => ({
  useOpportunityHistory: () => ({ items: [], status: 'ready', retry: vi.fn() }),
}));

import { App } from './App';

describe('App', () => {
  it('renders the scanner identity and live connection region', () => {
    render(<App />);

    expect(screen.getByRole('heading', { name: 'Arbitrage Scanner' })).toBeInTheDocument();
    expect(screen.getByRole('status', { name: 'Live market connection' })).toBeInTheDocument();
  });
});
