import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { FuturesAnalysis } from '../../app/types';
import type { useFuturesWorkspace } from '../../hooks/useFuturesWorkspace';

vi.mock('lightweight-charts', () => ({
  CandlestickSeries: {}, HistogramSeries: {}, ColorType: { Solid: 'solid' }, LineStyle: { Dashed: 2 },
  createChart: () => ({
    addSeries: (_type: unknown) => ({ setData: () => undefined, update: () => undefined, createPriceLine: () => undefined }),
    remove: () => undefined,
    priceScale: () => ({ applyOptions: () => undefined }),
    timeScale: () => ({ fitContent: () => undefined }),
  }),
}));

import { FuturesPageView } from './FuturesPage';

const contract = {
  symbol: 'COTI_USDT', status: 'trading', quantoMultiplier: 1, markPrice: 0.0106, indexPrice: 0.01059,
  lastPrice: 0.0106, makerFeeRate: -0.0001, takerFeeRate: 0.00075, orderSizeMin: 1,
  orderSizeMax: 4_500_000, priceIncrement: 0.00001, leverageMin: 1, leverageMax: 25,
  fundingRate: -0.002, fundingInterval: 14_400,
};

const baseScenario = {
  direction: 'long' as const, status: 'ready' as const, score: 80, entry: 0.0105, stop: 0.0100,
  targets: [0.0115], riskReward: 2, riskAmount: 10, contracts: 20_000, baseQuantity: 20_000,
  notional: 210, estimatedFees: 0.315, liquidationNote: 'Liquidation is intentionally not estimated.',
  reasons: ['EMA20 is above EMA50.', 'Mark price holds above the pullback mean.'],
};

const analysis: FuturesAnalysis = {
  contract, interval: '15m', strategy: 'trend_pullback', analyzedAt: 1_700_000_000_000,
  candles: [{ time: 1_700_000_000, open: 0.0104, high: 0.0107, low: 0.0103, close: 0.0106, volume: 1000 }],
  indicators: {
    ema20: 0.0105, ema50: 0.0102, atr14: 0.0002, rsi14: 58, mean20: 0.0104,
    upperBand: 0.0108, lowerBand: 0.0100, swingHigh: 0.0109, swingLow: 0.0099, marketBias: 'bullish',
  },
  liquidity: {
    bidWalls: [{ price: 0.0102, size: 100_000, notional: 1020 }],
    askWalls: [{ price: 0.0115, size: 120_000, notional: 1380 }],
  },
  long: baseScenario,
  short: { ...baseScenario, direction: 'short', status: 'no_trade', score: 20, entry: 0.0105, stop: 0.011, targets: [0.0095] },
  neutralRoute: {
    status: 'executable', reasonCode: '', reasons: ['Executable after four taker fees.'], symbol: 'COTIUSDT',
    longVenue: 'gate_futures', shortVenue: 'binance_futures', longEntry: 0.0105, shortEntry: 0.0107,
    baseQuantity: 10_000, longContracts: 10_000, shortContracts: 10_000,
    longNotional: 105, shortNotional: 107, grossSpreadPct: 1.9, openFees: 0.13,
    estimatedCloseFees: 0.13, netConvergenceSpreadPct: 1.65, nextFundingCarryPct: 0.01,
    indexDivergencePct: 0.1,
  },
};

type Workspace = ReturnType<typeof useFuturesWorkspace>;

function workspace(overrides: Partial<Workspace> = {}): Workspace {
  return {
    contracts: [contract], contractsStatus: 'ready', analysis: null, analysisStatus: 'idle',
    liveSnapshot: null, liveStatus: 'idle', executionStatus: 'idle', executionResult: null, error: null,
    analyze: vi.fn(async () => undefined), executeRoute: vi.fn(async () => undefined),
    retryContracts: vi.fn(async () => undefined),
    ...overrides,
  };
}

describe('FuturesPageView', () => {
  it('prefills the contract selected from a convergence opportunity link', () => {
    window.history.replaceState({}, '', '/futures?contract=H_USDT');
    const hContract = { ...contract, symbol: 'H_USDT' };

    render(<FuturesPageView workspace={workspace({ contracts: [contract, hContract] })} />);

    expect(screen.getByRole('combobox', { name: 'Gate contract' })).toHaveValue('H_USDT');
    window.history.replaceState({}, '', '/futures');
  });

  it('requires an explicit analysis action with the selected strategy and risk inputs', () => {
    const current = workspace();
    render(<FuturesPageView workspace={current} />);

    expect(screen.getByRole('heading', { name: 'Futures decision lab' })).toBeInTheDocument();
    expect(screen.getByText('Manual live execution')).toBeInTheDocument();
    expect(screen.getByText('Choose a contract and run an analysis.')).toBeInTheDocument();

    fireEvent.change(screen.getByRole('combobox', { name: 'Strategy' }), { target: { value: 'breakout' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Account balance' }), { target: { value: '2500' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Risk per setup' }), { target: { value: '0.75' } });
    fireEvent.click(screen.getByRole('button', { name: 'Analyze market' }));

    expect(current.analyze).toHaveBeenCalledWith({
      contract: 'COTI_USDT', interval: '15m', strategy: 'breakout', accountBalance: 2500, riskPercent: 0.75,
      tradeNotional: 100, minNetSpreadPct: 0.1,
    });
  });

  it('renders mirrored plans, the neutral route, and executes both legs only on click', () => {
    const current = workspace({ analysis, analysisStatus: 'ready' });
    render(<FuturesPageView workspace={current} />);

    expect(screen.getByText('Bullish regime')).toBeInTheDocument();
    const longPlan = screen.getByRole('region', { name: 'Long scenario' });
    const shortPlan = screen.getByRole('region', { name: 'Short scenario' });
    expect(within(longPlan).getByText('2.00R')).toBeInTheDocument();
    expect(within(longPlan).getByText('Ready')).toBeInTheDocument();
    expect(within(shortPlan).getByText('No trade')).toBeInTheDocument();
    expect(screen.getByText('Bid liquidity')).toBeInTheDocument();
    expect(screen.getByText('Ask liquidity')).toBeInTheDocument();

    expect(screen.getByRole('region', { name: 'Binance Gate neutral route' })).toHaveTextContent('Gate Futures');
    fireEvent.click(screen.getByRole('button', { name: 'Execute both futures legs' }));
    expect(current.executeRoute).toHaveBeenCalledOnce();
    expect(screen.queryByText('Paper positions')).not.toBeInTheDocument();
  });
});
