import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

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

vi.mock('../hooks/useScannerSocket', () => ({
  useScannerSocket: () => ({
    connection: 'live',
    lastUpdatedAt: 20_000,
    prices: {},
    quotes: {},
    spreads: {},
    history: {},
    opportunities: [],
    alertTriggers: [],
  }),
}));

import { App } from './App';

describe('App', () => {
  afterEach(() => window.history.replaceState({}, '', '/'));

  it('renders the scanner identity and live connection region', () => {
    render(<App />);

    expect(screen.getByRole('heading', { name: 'Arbitrage Scanner' })).toBeInTheDocument();
    expect(screen.getByRole('status', { name: 'Live market connection' })).toBeInTheDocument();
  });

  it('navigates to the all-pair opportunities workspace', () => {
    render(<App />);

    fireEvent.click(screen.getByRole('button', { name: 'Open opportunities' }));

    expect(screen.getByRole('heading', { name: 'Opportunities' })).toBeInTheDocument();
    expect(screen.getByText('All live routes across every tracked pair.')).toBeInTheDocument();
  });

  it('navigates to the persisted alerts workspace', () => {
    render(<App />);

    fireEvent.click(screen.getByRole('button', { name: 'Open alerts' }));

    expect(screen.getByRole('heading', { name: 'Alerts' })).toBeInTheDocument();
    expect(screen.getByText('Create actionable spread rules and monitor recent triggers.')).toBeInTheDocument();
  });
});
