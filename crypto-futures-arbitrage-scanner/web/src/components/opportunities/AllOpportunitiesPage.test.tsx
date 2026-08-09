import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import type { ScannerState } from '../../app/types';
import { AllOpportunitiesPage } from './AllOpportunitiesPage';

const state: ScannerState = {
  connection: 'live',
  lastUpdatedAt: 20_000,
  prices: {},
  quotes: {},
  spreads: {},
  history: {},
  alertTriggers: [],
  opportunities: [
    {
      id: 'coti-spot', symbol: 'COTIUSDT', buySource: 'kucoin_spot', sellSource: 'binance_spot',
      buyPrice: 0.0112, sellPrice: 0.0127, profitPct: 13.39, timestamp: 20_000,
    },
    {
      id: 'btc-spot', symbol: 'BTCUSDT', buySource: 'gate_spot', sellSource: 'binance_spot',
      buyPrice: 64_000, sellPrice: 64_320, profitPct: 0.5, timestamp: 19_000,
    },
    {
      id: 'eth-futures', symbol: 'ETHUSDT', buySource: 'binance_futures', sellSource: 'kucoin_futures',
      buyPrice: 3_400, sellPrice: 3_410.2, profitPct: 0.3, timestamp: 18_000,
    },
    {
      id: 'sol-mixed', symbol: 'SOLUSDT', buySource: 'gate_spot', sellSource: 'kucoin_futures',
      buyPrice: 160, sellPrice: 160.32, profitPct: 0.2, timestamp: 17_000,
    },
  ],
};

describe('AllOpportunitiesPage', () => {
  it('lists live routes across every pair instead of one selected pair', () => {
    render(<AllOpportunitiesPage now={20_000} state={state} />);

    const table = within(screen.getByRole('table'));
    expect(table.getByText('COTI/USDT')).toBeInTheDocument();
    expect(table.getByText('BTC/USDT')).toBeInTheDocument();
    expect(table.getByText('ETH/USDT')).toBeInTheDocument();
    expect(table.getByText('SOL/USDT')).toBeInTheDocument();
    expect(screen.getByText('4 live routes')).toBeInTheDocument();
  });

  it('filters routes by market type, search, exchange, and minimum spread', () => {
    render(<AllOpportunitiesPage now={20_000} state={state} />);

    fireEvent.click(screen.getByRole('button', { name: 'Show spot to spot routes' }));
    expect(screen.getByText('COTI/USDT')).toBeInTheDocument();
    expect(screen.getByText('BTC/USDT')).toBeInTheDocument();
    expect(screen.queryByText('ETH/USDT')).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole('searchbox', { name: 'Search opportunities' }), { target: { value: 'gate' } });
    expect(screen.queryByText('COTI/USDT')).not.toBeInTheDocument();
    expect(screen.getByText('BTC/USDT')).toBeInTheDocument();

    fireEvent.change(screen.getByRole('combobox', { name: 'Filter by exchange' }), { target: { value: 'binance_spot' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Minimum gross spread' }), { target: { value: '0.6' } });
    expect(screen.getByText('No live routes match these filters.')).toBeInTheDocument();
  });

  it('sorts the all-pair table by pair and spread', () => {
    render(<AllOpportunitiesPage now={20_000} state={state} />);

    fireEvent.click(screen.getByRole('button', { name: 'Sort by pair' }));
    const rows = within(screen.getByRole('table')).getAllByRole('row').slice(1);
    expect(within(rows[0]).getByText('BTC/USDT')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Sort by gross spread' }));
    expect(screen.getByRole('columnheader', { name: /Gross spread/ })).toHaveAttribute('aria-sort', 'descending');
  });
});
