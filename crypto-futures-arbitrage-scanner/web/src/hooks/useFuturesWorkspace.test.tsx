import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { FuturesAnalyzeInput } from '../app/types';
import { useFuturesWorkspace } from './useFuturesWorkspace';

const contractWire = {
  symbol: 'COTI_USDT', status: 'trading', quanto_multiplier: 1, mark_price: 0.0106,
  index_price: 0.01059, last_price: 0.0106, maker_fee_rate: -0.0001, taker_fee_rate: 0.00075,
  order_size_min: 1, order_size_max: 4_500_000, price_increment: 0.00001,
  leverage_min: 1, leverage_max: 25, funding_rate: -0.002, funding_interval: 14_400,
};

const scenarioWire = {
  direction: 'long', status: 'ready', score: 80, entry: 0.0105, stop: 0.0100,
  targets: [0.0115], risk_reward: 2, risk_amount: 10, contracts: 20_000,
  base_quantity: 20_000, notional: 210, estimated_fees: 0.315,
  liquidation_note: 'Not estimated.', reasons: ['EMA20 is above EMA50.'],
};

const analysisWire = {
  contract: contractWire, interval: '15m', strategy: 'trend_pullback', analyzed_at: 1_700_000_000_000,
  candles: [{ time: 1_700_000_000, open: 0.0104, high: 0.0107, low: 0.0103, close: 0.0106, volume: 1000 }],
  indicators: {
    ema20: 0.0105, ema50: 0.0102, atr14: 0.0002, rsi14: 58, mean20: 0.0104,
    upper_band: 0.0108, lower_band: 0.0100, swing_high: 0.0109, swing_low: 0.0099, market_bias: 'bullish',
  },
  liquidity: {
    bid_walls: [{ price: 0.0102, size: 100_000, notional: 1020 }],
    ask_walls: [{ price: 0.0115, size: 120_000, notional: 1380 }],
  },
  long: scenarioWire,
  short: { ...scenarioWire, direction: 'short', status: 'no_trade', entry: 0.0105, stop: 0.011, targets: [0.0095] },
  neutral_route: {
    status: 'executable', reason_code: '', reasons: ['Executable after four taker fees.'], symbol: 'COTIUSDT',
    long_venue: 'gate_futures', short_venue: 'binance_futures', long_entry: 0.0105, short_entry: 0.0107,
    base_quantity: 10_000, long_contracts: 10_000, short_contracts: 10_000,
    long_notional: 105, short_notional: 107, gross_spread_pct: 1.9, open_fees: 0.13,
    estimated_close_fees: 0.13, net_convergence_spread_pct: 1.65,
    next_funding_carry_pct: 0.01, index_divergence_pct: 0.1,
  },
};

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(new Response(JSON.stringify(body), {
    status, headers: { 'Content-Type': 'application/json' },
  }));
}

const input: FuturesAnalyzeInput = {
  contract: 'COTI_USDT', interval: '15m', strategy: 'trend_pullback', accountBalance: 1000, riskPercent: 1,
  tradeNotional: 100, minNetSpreadPct: 0.1,
};

describe('useFuturesWorkspace', () => {
  it('loads contracts and performs analysis only after an explicit action', async () => {
    const fetcher = vi.fn((url: string | URL | Request, init?: RequestInit) => {
      const path = String(url);
      if (path === '/api/futures/contracts') return jsonResponse({ items: [contractWire] });
      if (path.startsWith('/api/futures/live?')) return jsonResponse({
        captured_at: 1_700_000_000_100,
        candle: analysisWire.candles[0], gate: {}, binance: {}, route: { status: 'watch', reasons: [] },
      });
      if (path === '/api/futures/paper-positions') return jsonResponse({ items: [] });
      if (path === '/api/futures/analyze' && init?.method === 'POST') return jsonResponse(analysisWire);
      throw new Error(`Unexpected request ${path}`);
    }) as typeof fetch;

    const { result } = renderHook(() => useFuturesWorkspace(fetcher));
    await waitFor(() => expect(result.current.contractsStatus).toBe('ready'));
    expect(result.current.contracts[0].symbol).toBe('COTI_USDT');
    expect(result.current.analysis).toBeNull();
    expect(fetcher).not.toHaveBeenCalledWith('/api/futures/analyze', expect.anything());

    await act(async () => result.current.analyze(input));

    expect(result.current.analysisStatus).toBe('ready');
    expect(result.current.analysis?.long.riskReward).toBe(2);
    expect(fetcher).toHaveBeenCalledWith('/api/futures/analyze', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({
        contract: 'COTI_USDT', interval: '15m', strategy: 'trend_pullback', account_balance: 1000, risk_percent: 1,
        trade_notional: 100, min_net_spread_pct: 0.1,
      }),
    }));
  });

  it('refreshes the selected market candle after analysis', async () => {
    let liveCalls = 0;
    const fetcher = vi.fn((url: string | URL | Request) => {
      const path = String(url);
      if (path === '/api/futures/contracts') return jsonResponse({ items: [contractWire] });
      if (path === '/api/futures/paper-positions') return jsonResponse({ items: [] });
      if (path === '/api/futures/analyze') return jsonResponse(analysisWire);
      if (path.startsWith('/api/futures/live?')) {
        liveCalls += 1;
        return jsonResponse({
          captured_at: 1_700_000_000_000 + liveCalls,
          candle: { ...analysisWire.candles[0], time: 1_700_000_900, close: 0.0106 + liveCalls * 0.0001 },
          gate: {}, binance: {}, route: { status: 'watch', reasons: [] },
        });
      }
      throw new Error(`Unexpected request ${path}`);
    }) as typeof fetch;

    const { result } = renderHook(() => useFuturesWorkspace(fetcher, 20));
    await waitFor(() => expect(result.current.contractsStatus).toBe('ready'));
    await act(async () => result.current.analyze(input));

    await waitFor(() => expect(result.current.liveSnapshot?.candle.close).toBeGreaterThan(0.0107));
    expect(liveCalls).toBeGreaterThanOrEqual(2);
  });

  it('sends both live futures legs only after explicit execution', async () => {
    const fetcher = vi.fn((url: string | URL | Request, init?: RequestInit) => {
      const path = String(url);
      if (path === '/api/futures/contracts') return jsonResponse({ items: [contractWire] });
      if (path === '/api/futures/analyze') return jsonResponse(analysisWire);
      if (path.startsWith('/api/futures/live?')) return jsonResponse({
        captured_at: 1_700_000_000_100, candle: analysisWire.candles[0], gate: {}, binance: {},
        route: analysisWire.neutral_route,
      });
      if (path === '/api/futures/execute' && init?.method === 'POST') return jsonResponse({
        status: 'opened', route: analysisWire.neutral_route,
        fills: [
          { venue: 'gate_futures', order_id: 'gate-1', filled_contracts: 10_000, average_price: 0.0105 },
          { venue: 'binance_futures', order_id: 'binance-1', filled_contracts: 10_000, average_price: 0.0107 },
        ],
      });
      throw new Error(`Unexpected request ${path}`);
    }) as typeof fetch;

    const { result } = renderHook(() => useFuturesWorkspace(fetcher));
    await waitFor(() => expect(result.current.contractsStatus).toBe('ready'));
    await act(async () => result.current.analyze(input));
    await waitFor(() => expect(result.current.liveSnapshot?.route.status).toBe('executable'));
    expect(fetcher).not.toHaveBeenCalledWith('/api/futures/execute', expect.anything());

    await act(async () => result.current.executeRoute());

    expect(result.current.executionResult?.status).toBe('opened');
    expect(fetcher).toHaveBeenCalledWith('/api/futures/execute', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({
        contract: 'COTI_USDT', trade_notional: 100, min_net_spread_pct: 0.1,
        expected_long_venue: 'gate_futures', expected_short_venue: 'binance_futures', confirm_live: true,
      }),
    }));
  });

  it('keeps the sanitized exposure result visible when compensation fails', async () => {
    const exposedResult = {
      status: 'exposed', route: analysisWire.neutral_route,
      fills: [{ venue: 'gate_futures', order_id: 'gate-9', filled_contracts: 10_000, average_price: 0.0105 }],
      compensations: [],
    };
    const fetcher = vi.fn((url: string | URL | Request) => {
      const path = String(url);
      if (path === '/api/futures/contracts') return jsonResponse({ items: [contractWire] });
      if (path === '/api/futures/analyze') return jsonResponse(analysisWire);
      if (path.startsWith('/api/futures/live?')) return jsonResponse({
        captured_at: 1_700_000_000_100, candle: analysisWire.candles[0], gate: {}, binance: {}, route: analysisWire.neutral_route,
      });
      if (path === '/api/futures/execute') return jsonResponse({
        error: 'A futures leg remains exposed; inspect both exchange positions immediately', result: exposedResult,
      }, 500);
      throw new Error(`Unexpected request ${path}`);
    }) as typeof fetch;

    const { result } = renderHook(() => useFuturesWorkspace(fetcher));
    await waitFor(() => expect(result.current.contractsStatus).toBe('ready'));
    await act(async () => result.current.analyze(input));
    await waitFor(() => expect(result.current.liveSnapshot?.route.status).toBe('executable'));
    await act(async () => result.current.executeRoute());

    expect(result.current.executionStatus).toBe('error');
    expect(result.current.executionResult?.status).toBe('exposed');
    expect(result.current.executionResult?.fills[0].orderId).toBe('gate-9');
    expect(result.current.error).toContain('remains exposed');
  });

  it('shows the sanitized server error and leaves previous analysis untouched', async () => {
    let fail = false;
    const fetcher = vi.fn((url: string | URL | Request) => {
      const path = String(url);
      if (path === '/api/futures/contracts') return jsonResponse({ items: [contractWire] });
      if (path === '/api/futures/paper-positions') return jsonResponse({ items: [] });
      if (path === '/api/futures/analyze' && fail) return jsonResponse({ error: 'Gate futures market data unavailable' }, 502);
      if (path === '/api/futures/analyze') return jsonResponse(analysisWire);
      throw new Error(`Unexpected request ${path}`);
    }) as typeof fetch;

    const { result } = renderHook(() => useFuturesWorkspace(fetcher));
    await waitFor(() => expect(result.current.contractsStatus).toBe('ready'));
    await act(async () => result.current.analyze(input));
    fail = true;
    await act(async () => result.current.analyze({ ...input, interval: '1h' }));

    expect(result.current.analysis?.interval).toBe('15m');
    expect(result.current.analysisStatus).toBe('error');
    expect(result.current.error).toBe('Gate futures market data unavailable');
  });
});
