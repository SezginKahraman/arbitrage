import { describe, expect, it } from 'vitest';

import type { ScannerState } from '../app/types';
import {
  buildAdaptiveConvergenceSignals,
  buildFuturesConvergenceCandidates,
  FUTURES_SCAN_SOURCES,
} from './opportunity-desks';

const quoteState = (quotes: ScannerState['quotes']): ScannerState => ({
  connection: 'live',
  lastUpdatedAt: 100_000,
  prices: {},
  quotes,
  spreads: {},
  history: {},
  alertTriggers: [],
  connections: {},
  feedEvents: [],
  opportunities: [],
});

describe('buildFuturesConvergenceCandidates', () => {
  it('ranks executable ask-to-bid convergence after four screening fees', () => {
    const state = quoteState({
      BTCUSDT: {
        binance_futures: { symbol: 'BTCUSDT', source: 'binance_futures', bestBid: 99.9, bestAsk: 100, timestamp: 99_900 },
        gate_futures: { symbol: 'BTCUSDT', source: 'gate_futures', bestBid: 101, bestAsk: 101.2, timestamp: 99_800 },
        kucoin_futures: { symbol: 'BTCUSDT', source: 'kucoin_futures', bestBid: 100.5, bestAsk: 100.7, timestamp: 99_700 },
      },
      ETHUSDT: {
        binance_futures: { symbol: 'ETHUSDT', source: 'binance_futures', bestBid: 200, bestAsk: 200.1, timestamp: 99_900 },
        gate_futures: { symbol: 'ETHUSDT', source: 'gate_futures', bestBid: 201.1, bestAsk: 201.2, timestamp: 99_800 },
      },
    });

    const candidates = buildFuturesConvergenceCandidates(state.quotes, {}, 100_000);

    expect(candidates.map((item) => item.symbol)).toEqual(['BTCUSDT', 'ETHUSDT']);
    expect(candidates[0]).toMatchObject({
      longSource: 'binance_futures',
      shortSource: 'gate_futures',
      longEntry: 100,
      shortEntry: 101,
      estimatedRoundTripFeePct: 0.25,
      support: 'analyze',
    });
    expect(candidates[0].grossSpreadPct).toBeCloseTo(1, 8);
    expect(candidates[0].estimatedNetSpreadPct).toBeCloseTo(0.75, 8);
    expect(candidates[1].estimatedNetSpreadPct).toBeCloseTo(0.24975, 5);
  });

  it('ignores stale, disabled, unsupported, and crossed quote inputs', () => {
    const state = quoteState({
      SOLUSDT: {
        binance_futures: { symbol: 'SOLUSDT', source: 'binance_futures', bestBid: 149.9, bestAsk: 150, timestamp: 100_000 },
        gate_futures: { symbol: 'SOLUSDT', source: 'gate_futures', bestBid: 151, bestAsk: 151.2, timestamp: 80_000 },
        kucoin_futures: { symbol: 'SOLUSDT', source: 'kucoin_futures', bestBid: 150.8, bestAsk: 150.9, timestamp: 99_900 },
        bybit_futures: { symbol: 'SOLUSDT', source: 'bybit_futures', bestBid: 190, bestAsk: 191, timestamp: 100_000 },
      },
      XRPUSDT: {
        binance_futures: { symbol: 'XRPUSDT', source: 'binance_futures', bestBid: 1.2, bestAsk: 0, timestamp: 100_000 },
        gate_futures: { symbol: 'XRPUSDT', source: 'gate_futures', bestBid: 1.21, bestAsk: 1.22, timestamp: 100_000 },
      },
    });

    const candidates = buildFuturesConvergenceCandidates(
      state.quotes,
      { kucoin_futures: false },
      100_000,
    );

    expect(candidates).toEqual([]);
    expect(FUTURES_SCAN_SOURCES).toEqual(['binance_futures', 'gate_futures', 'kucoin_futures']);
  });

  it('marks KuCoin routes as public scan only and Binance-Gate routes as analyzable', () => {
    const state = quoteState({
      COTIUSDT: {
        binance_futures: { symbol: 'COTIUSDT', source: 'binance_futures', bestBid: 0.0109, bestAsk: 0.011, timestamp: 100_000 },
        kucoin_futures: { symbol: 'COTIUSDT', source: 'kucoin_futures', bestBid: 0.0113, bestAsk: 0.0114, timestamp: 100_000 },
      },
    });

    expect(buildFuturesConvergenceCandidates(state.quotes, {}, 100_000)[0].support).toBe('scan_only');
  });
});

describe('buildAdaptiveConvergenceSignals', () => {
  it('distinguishes an abnormal executable spread from the route historical median', () => {
    const times = Array.from({ length: 720 }, (_, index) => index * 5_000);
    const candidate = {
      id: 'HUSDT:gate_futures:binance_futures',
      symbol: 'HUSDT',
      longSource: 'gate_futures' as const,
      shortSource: 'binance_futures' as const,
      longEntry: 100,
      shortEntry: 100.8,
      grossSpreadPct: 0.8,
      estimatedRoundTripFeePct: 0.25,
      estimatedNetSpreadPct: 0.55,
      timestamp: 360_000,
      support: 'analyze' as const,
    };
    const history = {
      HUSDT: {
        gate_futures: times.map((time) => ({ time, value: 100 })),
        binance_futures: times.map((time, index) => ({ time, value: index % 2 ? 100.08 : 100.12 })),
      },
    };

    const signal = buildAdaptiveConvergenceSignals([candidate], history)[0];

    expect(signal.sampleCount).toBe(720);
    expect(signal.baselineMedianPct).toBeCloseTo(0.1, 3);
    expect(signal.deviationFromNormalPct).toBeCloseTo(0.7, 3);
    expect(signal.zScore).toBeGreaterThan(2);
    expect(signal.status).toBe('qualified');
  });

  it('refuses to qualify a route without enough paired history', () => {
    const candidate = {
      id: 'COTIUSDT:gate_futures:kucoin_futures',
      symbol: 'COTIUSDT',
      longSource: 'gate_futures' as const,
      shortSource: 'kucoin_futures' as const,
      longEntry: 1,
      shortEntry: 1.01,
      grossSpreadPct: 1,
      estimatedRoundTripFeePct: 0.27,
      estimatedNetSpreadPct: 0.73,
      timestamp: 50_000,
      support: 'scan_only' as const,
    };
    const times = Array.from({ length: 719 }, (_, index) => index * 5_000);
    const history = {
      COTIUSDT: {
        gate_futures: times.map((time) => ({ time, value: 1 })),
        kucoin_futures: times.map((time) => ({ time, value: 1.001 })),
      },
    };

    expect(buildAdaptiveConvergenceSignals([candidate], history)[0].status).toBe('insufficient_history');
  });
});
