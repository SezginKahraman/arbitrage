import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useState } from 'react';

const chartMocks = vi.hoisted(() => ({ setData: vi.fn() }));

vi.mock('lightweight-charts', () => ({
  ColorType: { Solid: 'solid' },
  LineSeries: {},
  createChart: () => ({
    addSeries: () => ({ setData: chartMocks.setData }),
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
      binance_futures: { price: 0.01135, updatedAt: 20_000 },
      gate_spot: { price: 0.01132, updatedAt: 20_000 },
      binance_spot: { price: 0.01140723, updatedAt: 20_000 },
    },
  },
  quotes: {},
  spreads: {},
  history: {
    COTIUSDT: {
      gate_spot: [{ time: 20_000, value: 0.01132 }],
      binance_spot: [{ time: 20_000, value: 0.01140723 }],
      gate_futures: [{ time: 20_000, value: 0.01131 }],
      binance_futures: [{ time: 20_000, value: 0.01135 }],
    },
  },
  alertTriggers: [],
  connections: {
    gate_futures: { source: 'gate_futures', connected: true, symbols: ['COTIUSDT'], updatedAt: 20_000 },
    binance_futures: { source: 'binance_futures', connected: true, symbols: ['COTIUSDT'], updatedAt: 20_000 },
    gate_spot: { source: 'gate_spot', connected: true, symbols: ['COTIUSDT'], updatedAt: 20_000 },
    binance_spot: { source: 'binance_spot', connected: true, symbols: ['COTIUSDT'], updatedAt: 20_000 },
  },
  feedEvents: [],
  opportunities: [
    {
      id: 'coti-spot-route',
      symbol: 'COTIUSDT',
      buySource: 'gate_spot',
      sellSource: 'binance_spot',
      buyPrice: 0.01132,
      sellPrice: 0.01140723,
      profitPct: 0.86,
      timestamp: 20_000,
    },
    {
      id: 'coti-futures-route',
      symbol: 'COTIUSDT',
      buySource: 'gate_futures',
      sellSource: 'binance_futures',
      buyPrice: 0.01131,
      sellPrice: 0.01135,
      profitPct: 0.72,
      timestamp: 20_000,
    },
    {
      id: 'coti-cross-route',
      symbol: 'COTIUSDT',
      buySource: 'gate_futures',
      sellSource: 'binance_spot',
      buyPrice: 0.01131,
      sellPrice: 0.01140723,
      profitPct: 0.91,
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

function StatefulDashboard() {
  const [preferences, setPreferences] = useState(DEFAULT_PREFERENCES);
  return (
    <ScannerDashboard
      history={{ items: [], status: 'ready', retry: vi.fn() }}
      now={20_000}
      onPreferencesChange={setPreferences}
      preferences={preferences}
      state={state}
    />
  );
}

describe('ScannerDashboard', () => {
  afterEach(() => {
    vi.useRealTimers();
    chartMocks.setData.mockClear();
  });

  it('renders the best route and honest execution status', () => {
    renderDashboard();

    expect(screen.getByRole('heading', { name: 'COTI/USDT' })).toBeInTheDocument();
    expect(screen.getAllByText('Gate.io Spot').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Binance Spot').length).toBeGreaterThan(0);
    expect(screen.getAllByText('+0.86%').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Transfer route unverified').length).toBeGreaterThan(0);
    expect(screen.getByLabelText('2 of 2 feeds connected')).toBeInTheDocument();
    expect(screen.getByLabelText('2 of 2 books fresh')).toBeInTheDocument();
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

  it('offers spot, futures, and cross-market comparison modes', () => {
    const onPreferencesChange = renderDashboard();

    expect(screen.getByRole('button', { name: 'Compare spot markets' })).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(screen.getByRole('button', { name: 'Compare futures markets' }));

    const update = onPreferencesChange.mock.calls[0][0] as (current: UiPreferences) => UiPreferences;
    expect(update(DEFAULT_PREFERENCES)).toMatchObject({ comparisonMode: 'futures' });
  });

  it('shows only routes that belong to the selected comparison mode', () => {
    renderDashboard({ ...DEFAULT_PREFERENCES, comparisonMode: 'futures' } as UiPreferences);

    const table = within(screen.getByRole('table'));
    expect(table.getByText('Gate.io Futures')).toBeInTheDocument();
    expect(table.getByText('Binance Futures')).toBeInTheDocument();
    expect(table.queryByText('Gate.io Spot')).not.toBeInTheDocument();
    expect(table.queryByText('Binance Spot')).not.toBeInTheDocument();
  });

  it('exposes one market toggle that drives the dashboard source filter', () => {
    const onPreferencesChange = renderDashboard();

    fireEvent.click(screen.getByRole('button', { name: 'Disable Binance Spot' }));

    const update = onPreferencesChange.mock.calls[0][0] as (current: UiPreferences) => UiPreferences;
    expect(update(DEFAULT_PREFERENCES).enabledSources.binance_spot).toBe(false);
  });

  it('applies a market toggle to both the opportunity table and price chart', () => {
    render(<StatefulDashboard />);
    const table = within(screen.getByRole('table'));
    const chart = within(screen.getByRole('heading', { name: 'Price comparison' }).closest('section') as HTMLElement);

    expect(table.getByText('Gate.io Spot')).toBeInTheDocument();
    expect(chart.getByText('GAT-S')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Disable Gate.io Spot' }));

    expect(within(screen.getByRole('table')).queryByText('Gate.io Spot')).not.toBeInTheDocument();
    expect(chart.queryByText('GAT-S')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Enable Gate.io Spot' })).toBeInTheDocument();
  });

  it('keeps chart source filters stable across unrelated dashboard renders', () => {
    const props = {
      history: { items: [], status: 'ready' as const, retry: vi.fn() },
      now: 20_000,
      onPreferencesChange: vi.fn(),
      preferences: DEFAULT_PREFERENCES,
      state,
    };
    const { rerender } = render(<ScannerDashboard {...props} />);
    const initialCalls = chartMocks.setData.mock.calls.length;

    rerender(<ScannerDashboard {...props} />);

    expect(chartMocks.setData).toHaveBeenCalledTimes(initialCalls);
  });

  it('persists collapsible opportunities and split or stacked panel layouts', () => {
    const onPreferencesChange = renderDashboard();

    fireEvent.click(screen.getByRole('button', { name: 'Collapse live opportunities' }));
    fireEvent.click(screen.getByRole('button', { name: 'Stack table and chart' }));

    const collapse = onPreferencesChange.mock.calls[0][0] as (current: UiPreferences) => UiPreferences;
    const layout = onPreferencesChange.mock.calls[1][0] as (current: UiPreferences) => UiPreferences;
    expect(collapse(DEFAULT_PREFERENCES)).toMatchObject({ opportunitiesCollapsed: true });
    expect(layout(DEFAULT_PREFERENCES)).toMatchObject({ dashboardLayout: 'stacked' });
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
      enabledSources: { ...DEFAULT_PREFERENCES.enabledSources, gate_spot: false },
    });

    expect(screen.getByLabelText('1 of 1 feeds connected')).toBeInTheDocument();
    expect(screen.getByLabelText('1 of 1 books fresh')).toBeInTheDocument();
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
    expect(screen.getByLabelText('2 of 2 feeds connected')).toBeInTheDocument();
    expect(screen.getByLabelText('2 of 2 books fresh')).toBeInTheDocument();

    act(() => vi.advanceTimersByTime(16_000));
    expect(screen.getByLabelText('2 of 2 feeds connected')).toBeInTheDocument();
    expect(screen.getByLabelText('0 of 2 books fresh')).toBeInTheDocument();
  });

  it('persists accessible table sort changes', () => {
    const onPreferencesChange = renderDashboard();

    fireEvent.click(screen.getByRole('button', { name: 'Sort by buy source' }));
    const update = onPreferencesChange.mock.calls[0][0] as (current: UiPreferences) => UiPreferences;
    expect(update(DEFAULT_PREFERENCES).sort).toEqual({ field: 'buy_source', direction: 'asc' });
  });

  it('toggles an active table column between descending and ascending', () => {
    render(<StatefulDashboard />);
    const spreadHeader = screen.getByRole('columnheader', { name: 'Gross spread' });
    expect(spreadHeader).toHaveAttribute('aria-sort', 'descending');

    fireEvent.click(screen.getByRole('button', { name: 'Sort by gross spread' }));
    expect(screen.getByRole('columnheader', { name: 'Gross spread' })).toHaveAttribute('aria-sort', 'ascending');
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
          buySource: 'bybit_spot',
          sellSource: 'binance_spot',
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

  it('shows only the latest historical session per route and counts live routes separately', () => {
    renderDashboard(DEFAULT_PREFERENCES, {
      status: 'ready',
      retry: vi.fn(),
      items: [
        {
          id: 'history:older', symbol: 'COTIUSDT', buySource: 'bybit_spot', sellSource: 'binance_spot',
          buyPrice: 0.011, sellPrice: 0.0111, profitPct: 0.7, timestamp: 10_000, historical: true,
        },
        {
          id: 'history:newer', symbol: 'COTIUSDT', buySource: 'bybit_spot', sellSource: 'binance_spot',
          buyPrice: 0.0111, sellPrice: 0.0112, profitPct: 0.8, timestamp: 12_000, historical: true,
        },
      ],
    });

    expect(screen.getAllByText('History')).toHaveLength(1);
    expect(screen.getByText('+0.80%')).toBeInTheDocument();
    expect(screen.queryByText('+0.70%')).not.toBeInTheDocument();
    expect(screen.getByText('1 live')).toBeInTheDocument();
  });
});
