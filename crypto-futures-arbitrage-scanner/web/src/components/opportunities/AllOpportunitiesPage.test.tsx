import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { ScannerState } from '../../app/types';
import type { MarketCatalogState } from '../../hooks/useMarketCatalog';
import { AllOpportunitiesPage } from './AllOpportunitiesPage';

const transferRoutes = {
  status: 'ready' as const,
  routes: {
    'COTI:kucoin_spot:binance_spot': {
      asset: 'COTI', source: 'kucoin_spot', destination: 'binance_spot', status: 'blocked' as const, reason: 'withdrawal disabled', checkedAt: 1,
      networks: [], sourceNetworks: [], destinationNetworks: [],
    },
    'BTC:gate_spot:binance_spot': {
      asset: 'BTC', source: 'gate_spot', destination: 'binance_spot', status: 'ready' as const, reason: 'verified common network available', checkedAt: 1,
      networks: [], sourceNetworks: [], destinationNetworks: [],
    },
  },
};

const state: ScannerState = {
  connection: 'live',
  lastUpdatedAt: 20_000,
  prices: {},
  quotes: {
    BTCUSDT: {
      binance_futures: { symbol: 'BTCUSDT', source: 'binance_futures', bestBid: 100, bestAsk: 100.1, timestamp: 20_000 },
      gate_futures: { symbol: 'BTCUSDT', source: 'gate_futures', bestBid: 101, bestAsk: 101.1, timestamp: 20_000 },
      kucoin_futures: { symbol: 'BTCUSDT', source: 'kucoin_futures', bestBid: 100.5, bestAsk: 100.6, timestamp: 20_000 },
    },
    ETHUSDT: {
      binance_futures: { symbol: 'ETHUSDT', source: 'binance_futures', bestBid: 200, bestAsk: 200.1, timestamp: 20_000 },
      kucoin_futures: { symbol: 'ETHUSDT', source: 'kucoin_futures', bestBid: 201, bestAsk: 201.1, timestamp: 20_000 },
    },
  },
  spreads: {},
  history: {},
  alertTriggers: [],
  connections: {},
  feedEvents: [],
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
  it('defaults the Spot desk to verified READY transfer routes only', () => {
    render(<AllOpportunitiesPage now={20_000} state={state} transferRoutes={transferRoutes} />);

    const table = within(screen.getByRole('table'));
    expect(table.getByText('BTC/USDT')).toBeInTheDocument();
    expect(table.queryByText('COTI/USDT')).not.toBeInTheDocument();
    expect(table.queryByText('ETH/USDT')).not.toBeInTheDocument();
    expect(screen.getByText('1 live route')).toBeInTheDocument();
  });

  it('switches between Spot transfers, Futures convergence, and Strategies desks', () => {
    render(<AllOpportunitiesPage now={20_000} state={state} transferRoutes={transferRoutes} />);

    fireEvent.click(screen.getByRole('button', { name: 'Open Futures convergence desk' }));
    expect(screen.getByRole('heading', { name: 'Futures convergence' })).toBeInTheDocument();
    expect(screen.getAllByText('LONG').length).toBeGreaterThan(0);
    expect(screen.getAllByText('SHORT').length).toBeGreaterThan(0);
    expect(screen.getByText('Analyze supported')).toBeInTheDocument();
    expect(screen.getByText('Public scan only')).toBeInTheDocument();
    const futuresRows = within(screen.getByRole('table')).getAllByRole('row').slice(1);
    expect(within(futuresRows[0]).getByText('BTC/USDT')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Open Strategies desk' }));
    expect(screen.getByRole('heading', { name: 'Strategy registry' })).toBeInTheDocument();
    expect(screen.getByText('Trend Pullback')).toBeInTheDocument();
    expect(screen.getByText('Breakout')).toBeInTheDocument();
    expect(screen.getByText('Mean Reversion')).toBeInTheDocument();
    expect(screen.getByText('Adaptive Cross-Venue Convergence')).toBeInTheDocument();
  });

  it('filters routes by market type, search, exchange, and minimum spread', () => {
    render(<AllOpportunitiesPage now={20_000} state={state} transferRoutes={transferRoutes} />);

    fireEvent.change(screen.getByRole('combobox', { name: 'Filter by transfer route' }), { target: { value: 'all' } });
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
    render(<AllOpportunitiesPage now={20_000} state={state} transferRoutes={transferRoutes} />);

    fireEvent.change(screen.getByRole('combobox', { name: 'Filter by transfer route' }), { target: { value: 'all' } });

    fireEvent.click(screen.getByRole('button', { name: 'Sort by pair' }));
    const rows = within(screen.getByRole('table')).getAllByRole('row').slice(1);
    expect(within(rows[0]).getByText('BTC/USDT')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Sort by gross spread' }));
    expect(screen.getByRole('columnheader', { name: /Gross spread/ })).toHaveAttribute('aria-sort', 'descending');
  });

  it('adds and removes discovered markets through the server watchlist', async () => {
    const replace = vi.fn(async () => undefined);
    const marketCatalog: MarketCatalogState = {
      candidates: [
        { symbol: 'BTCUSDT', base: 'BTC', spotSources: ['binance_spot'], futuresSources: ['gate_futures'], sources: ['binance_spot', 'gate_futures'] },
        { symbol: 'COTIUSDT', base: 'COTI', spotSources: ['binance_spot', 'gate_spot'], futuresSources: [], sources: ['binance_spot', 'gate_spot'] },
        { symbol: 'LINKUSDT', base: 'LINK', spotSources: ['binance_spot', 'gate_spot'], futuresSources: ['kucoin_futures'], sources: ['binance_spot', 'gate_spot', 'kucoin_futures'] },
      ],
      sources: [], watchlist: ['BTCUSDT', 'COTIUSDT'], limit: 20, status: 'ready', saving: false, error: null,
      replace, retry: vi.fn(),
    };
    render(<AllOpportunitiesPage marketCatalog={marketCatalog} now={20_000} state={state} />);

    fireEvent.click(screen.getByRole('button', { name: 'Add pair' }));
    expect(screen.getByRole('heading', { name: 'Add a USDT pair' })).toBeInTheDocument();
    fireEvent.change(screen.getByRole('textbox', { name: 'Search market catalog' }), { target: { value: 'LINK' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add LINK/USDT' }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith(['BTCUSDT', 'COTIUSDT', 'LINKUSDT']));

    fireEvent.click(screen.getByRole('button', { name: 'Remove COTI/USDT' }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith(['BTCUSDT']));
  });

  it('filters spot opportunities to READY and CHECK common networks', () => {
    render(<AllOpportunitiesPage
      now={20_000}
      state={state}
      transferRoutes={transferRoutes}
    />);

    fireEvent.change(screen.getByRole('combobox', { name: 'Filter by transfer route' }), { target: { value: 'common' } });
    expect(screen.getByText('BTC/USDT')).toBeInTheDocument();
    expect(screen.queryByText('COTI/USDT')).not.toBeInTheDocument();
    expect(screen.queryByText('ETH/USDT')).not.toBeInTheDocument();
  });

  it('removes disabled sources from routes, counts, and exchange filters', () => {
    const bybitRoute = {
      id: 'wal-bybit', symbol: 'WALUSDT', buySource: 'bybit_spot', sellSource: 'binance_spot',
      buyPrice: 0.02223, sellPrice: 0.0224, profitPct: 0.76, timestamp: 20_000,
    };
    render(<AllOpportunitiesPage
      enabledSources={{ bybit_spot: false, bybit_futures: false }}
      now={20_000}
      state={{ ...state, opportunities: [...state.opportunities, bybitRoute] }}
      transferRoutes={transferRoutes}
    />);

    expect(screen.queryByText('WAL/USDT')).not.toBeInTheDocument();
    expect(screen.getByText('1 live route')).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Bybit Spot' })).not.toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Bybit Futures' })).not.toBeInTheDocument();
  });
});
