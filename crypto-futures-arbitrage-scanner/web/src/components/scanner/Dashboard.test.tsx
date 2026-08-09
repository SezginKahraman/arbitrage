import { act, fireEvent, render, screen, within } from '@testing-library/react';
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

import type { ScannerState, UiPreferences } from '../../app/types';
import { DEFAULT_PREFERENCES } from '../../lib/preferences';
import { ScannerDashboard } from './ScannerDashboard';

const state: ScannerState = {
  connection: 'live',
  lastUpdatedAt: 20_000,
  prices: {
    COTIUSDT: {
      gate_futures: { price: 0.01131, updatedAt: 20_000 },
      binance_spot: { price: 0.01140723, updatedAt: 20_000 },
    },
  },
  quotes: {},
  spreads: {},
  history: {},
  opportunities: [
    {
      id: 'coti-route',
      symbol: 'COTIUSDT',
      buySource: 'gate_futures',
      sellSource: 'binance_spot',
      buyPrice: 0.01131,
      sellPrice: 0.01140723,
      profitPct: 0.86,
      timestamp: 20_000,
    },
  ],
};

function renderDashboard(
  preferences: UiPreferences = DEFAULT_PREFERENCES,
  history: Parameters<typeof ScannerDashboard>[0]['history'] = { items: [], status: 'ready', retry: vi.fn() },
) {
  const onPreferencesChange = vi.fn();
  render(
    <ScannerDashboard
      history={history}
      now={20_000}
      onPreferencesChange={onPreferencesChange}
      preferences={preferences}
      state={state}
    />,
  );
  return onPreferencesChange;
}

describe('ScannerDashboard', () => {
  afterEach(() => vi.useRealTimers());

  it('renders the best route and honest execution status', () => {
    renderDashboard();

    expect(screen.getByRole('heading', { name: 'COTI/USDT' })).toBeInTheDocument();
    expect(screen.getAllByText('Gate.io Futures').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Binance Spot').length).toBeGreaterThan(0);
    expect(screen.getAllByText('+0.86%').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Transfer route unverified').length).toBeGreaterThan(0);
    expect(screen.getByText('2 / 2')).toBeInTheDocument();
    expect(screen.getAllByText(/0\.01140723/).length).toBeGreaterThan(0);
  });

  it('filters opportunities by the persisted minimum spread', () => {
    renderDashboard({ ...DEFAULT_PREFERENCES, minSpread: 1 });

    expect(screen.getByText('No opportunities match the current filters.')).toBeInTheDocument();
    expect(screen.getByText('Waiting for a qualifying route')).toBeInTheDocument();
    expect(screen.queryByText('+0.86%')).not.toBeInTheDocument();
  });

  it('opens settings and emits a source preference update', () => {
    const onPreferencesChange = renderDashboard();

    fireEvent.click(screen.getAllByRole('button', { name: 'Open settings' })[0]);
    fireEvent.click(screen.getByRole('checkbox', { name: 'Binance Spot' }));

    expect(onPreferencesChange).toHaveBeenCalledOnce();
  });

  it('filters the opportunity table to the selected symbol', () => {
    renderDashboard(DEFAULT_PREFERENCES, {
      status: 'ready',
      retry: vi.fn(),
      items: [
        {
          id: 'history:btc',
          symbol: 'BTCUSDT',
          buySource: 'bybit_futures',
          sellSource: 'binance_futures',
          buyPrice: 60_000,
          sellPrice: 61_000,
          profitPct: 1.6,
          timestamp: 10_000,
          historical: true,
        },
      ],
    });

    expect(within(screen.getByRole('table')).queryByText('BTC/USDT')).not.toBeInTheDocument();
  });

  it('uses the same enabled-source filter for online and total counts', () => {
    renderDashboard({
      ...DEFAULT_PREFERENCES,
      enabledSources: { ...DEFAULT_PREFERENCES.enabledSources, gate_futures: false },
    });

    expect(screen.getByText('1 / 1')).toBeInTheDocument();
  });

  it('marks silent sources stale without requiring another socket frame', () => {
    vi.useFakeTimers();
    vi.setSystemTime(20_000);
    render(
      <ScannerDashboard
        history={{ items: [], status: 'ready', retry: vi.fn() }}
        onPreferencesChange={vi.fn()}
        preferences={DEFAULT_PREFERENCES}
        state={state}
      />,
    );
    expect(screen.getByText('2 / 2')).toBeInTheDocument();

    act(() => vi.advanceTimersByTime(16_000));
    expect(screen.getByText('0 / 2')).toBeInTheDocument();
  });

  it('persists accessible table sort changes', () => {
    const onPreferencesChange = renderDashboard();

    fireEvent.click(screen.getByRole('button', { name: 'Sort by buy source' }));
    const update = onPreferencesChange.mock.calls[0][0] as (current: UiPreferences) => UiPreferences;
    expect(update(DEFAULT_PREFERENCES).sort).toEqual({ field: 'buy_source', direction: 'asc' });
  });

  it('exposes settings as a keyboard modal and closes it with Escape', () => {
    renderDashboard();
    const opener = screen.getAllByRole('button', { name: 'Open settings' })[0];
    opener.focus();
    fireEvent.click(opener);

    expect(screen.getByRole('dialog', { name: 'Scanner settings' })).toBeInTheDocument();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('dialog', { name: 'Scanner settings' })).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it('keeps live opportunities visible while history is degraded', () => {
    const retry = vi.fn();
    renderDashboard(DEFAULT_PREFERENCES, { items: [], status: 'degraded', retry });

    expect(screen.getByText('Opportunity history is unavailable. Live scanning is unaffected.')).toBeInTheDocument();
    expect(screen.getAllByText('COTI/USDT').length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: 'Retry opportunity history' }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it('labels persisted opportunity sessions as history', () => {
    renderDashboard(DEFAULT_PREFERENCES, {
      status: 'ready',
      retry: vi.fn(),
      items: [
        {
          id: 'history:7',
          symbol: 'COTIUSDT',
          buySource: 'bybit_futures',
          sellSource: 'binance_futures',
          buyPrice: 0.011,
          sellPrice: 0.0111,
          profitPct: 0.72,
          peakProfitPct: 0.91,
          timestamp: 10_000,
          historical: true,
        },
      ],
    });

    expect(screen.getByText('History')).toBeInTheDocument();
    expect(screen.getByText('Peak 0.91%')).toBeInTheDocument();
  });
});
