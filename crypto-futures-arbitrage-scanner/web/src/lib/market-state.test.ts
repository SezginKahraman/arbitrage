import { describe, expect, it } from 'vitest';

import {
  countFreshSources,
  createInitialScannerState,
  reduceScannerMessage,
  selectBestOpportunity,
} from './market-state';

describe('market state', () => {
  it('stores versioned executable quote updates and charts their midpoint', () => {
    const state = reduceScannerMessage(
      createInitialScannerState(),
      {
        type: 'quote_update',
        version: 1,
        quote: {
          symbol: 'COTIUSDT',
          source: 'binance_spot',
          best_bid: 0.0114,
          best_ask: 0.01142,
          timestamp: 5_000,
        },
      },
      5_000,
    );

    expect(state.quotes.COTIUSDT.binance_spot).toMatchObject({ bestBid: 0.0114, bestAsk: 0.01142 });
    expect(state.prices.COTIUSDT.binance_spot.price).toBeCloseTo(0.01141);
  });

  it('normalizes current price messages for every symbol', () => {
    const state = reduceScannerMessage(
      createInitialScannerState(),
      {
        type: 'prices',
        prices: {
          COTIUSDT: { binance_spot: 0.01140723, gate_futures: 0.01131 },
          BTCUSDT: { binance_spot: 64975.82 },
        },
      },
      1_000,
    );

    expect(state.prices.COTIUSDT.binance_spot).toMatchObject({ price: 0.01140723, updatedAt: 1_000 });
    expect(state.history.COTIUSDT.binance_spot).toEqual([{ time: 0, value: 0.01140723 }]);
    expect(state.prices.BTCUSDT.binance_spot.price).toBe(64975.82);
  });

  it('does not refresh unchanged cached legacy prices', () => {
    const first = reduceScannerMessage(
      createInitialScannerState(),
      { type: 'prices', prices: { COTIUSDT: { binance_spot: 0.01140723 } } },
      1_000,
    );
    const repeated = reduceScannerMessage(
      first,
      { type: 'prices', prices: { COTIUSDT: { binance_spot: 0.01140723 } } },
      20_000,
    );

    expect(repeated.prices.COTIUSDT.binance_spot.updatedAt).toBe(1_000);
    expect(repeated.history.COTIUSDT.binance_spot).toHaveLength(1);
  });

  it('uses source timestamps from versioned price updates', () => {
    const state = reduceScannerMessage(
      createInitialScannerState(),
      {
        type: 'price_update',
        version: 1,
        price: { symbol: 'COTIUSDT', source: 'pyth', price: 0.0114, timestamp: 7_000 },
      },
      9_000,
    );

    expect(state.prices.COTIUSDT.pyth).toEqual({ price: 0.0114, updatedAt: 7_000 });
    expect(state.lastUpdatedAt).toBe(9_000);
  });

  it('retains four hours in five-second chart buckets instead of raw snapshots', () => {
    let state = createInitialScannerState();
    for (let second = 0; second <= 600; second += 1) {
      state = reduceScannerMessage(
        state,
        {
          type: 'price_update',
          version: 1,
          price: { symbol: 'COTIUSDT', source: 'pyth', price: 0.01 + second / 1_000_000, timestamp: second * 1_000 },
        },
        second * 1_000,
      );
    }

    expect(state.history.COTIUSDT.pyth).toHaveLength(121);
  });

  it('does not replace chart history state for ticks inside the same bucket', () => {
    const first = reduceScannerMessage(
      createInitialScannerState(),
      {
        type: 'price_update',
        version: 1,
        price: { symbol: 'COTIUSDT', source: 'pyth', price: 0.0114, timestamp: 7_000 },
      },
      7_000,
    );
    const second = reduceScannerMessage(
      first,
      {
        type: 'price_update',
        version: 1,
        price: { symbol: 'COTIUSDT', source: 'pyth', price: 0.0115, timestamp: 8_000 },
      },
      8_000,
    );

    expect(second.history).toBe(first.history);
    expect(second.prices.COTIUSDT.pyth.price).toBe(0.0115);
  });

  it('ignores malformed messages without replacing valid state', () => {
    const state = createInitialScannerState();

    expect(reduceScannerMessage(state, { type: 'prices', prices: { COTIUSDT: { gate_futures: 'bad' } } })).toBe(
      state,
    );
    expect(reduceScannerMessage(state, 'not an object')).toBe(state);
  });

  it('retains opportunities across symbols and selects the best enabled fresh route', () => {
    const first = reduceScannerMessage(
      createInitialScannerState(),
      {
        type: 'arbitrage',
        opportunity: {
          symbol: 'COTIUSDT',
          buy_source: 'gate_futures',
          sell_source: 'binance_spot',
          buy_price: 0.01131,
          sell_price: 0.01140723,
          profit_pct: 0.86,
          timestamp: 10_000,
        },
      },
      10_000,
    );
    const state = reduceScannerMessage(
      first,
      {
        type: 'arbitrage',
        opportunity: {
          symbol: 'BTCUSDT',
          buy_source: 'binance_spot',
          sell_source: 'bybit_futures',
          buy_price: 64000,
          sell_price: 64100,
          profit_pct: 0.15,
          timestamp: 11_000,
        },
      },
      11_000,
    );

    expect(state.opportunities).toHaveLength(2);
    expect(selectBestOpportunity(state, 'COTIUSDT', { gate_futures: true, binance_spot: true }, 20_000)?.profitPct).toBe(
      0.86,
    );
    expect(selectBestOpportunity(state, 'COTIUSDT', { gate_futures: false, binance_spot: true }, 20_000)).toBeNull();
    expect(selectBestOpportunity(state, 'COTIUSDT', { gate_futures: true, binance_spot: true }, 30_001)).toBeNull();
    expect(selectBestOpportunity(state, 'COTIUSDT', { gate_futures: true, binance_spot: true }, 20_000, 1)).toBeNull();
  });

  it('keeps only the latest update for each executable route', () => {
    const first = reduceScannerMessage(
      createInitialScannerState(),
      {
        type: 'arbitrage',
        opportunity: {
          symbol: 'COTIUSDT', buy_source: 'gate_spot', sell_source: 'binance_spot',
          buy_price: 0.01131, sell_price: 0.01140, profit_pct: 0.79, timestamp: 10_000,
        },
      },
      10_000,
    );
    const latest = reduceScannerMessage(
      first,
      {
        type: 'arbitrage',
        opportunity: {
          symbol: 'COTIUSDT', buy_source: 'gate_spot', sell_source: 'binance_spot',
          buy_price: 0.01132, sell_price: 0.01141, profit_pct: 0.8, timestamp: 20_000,
        },
      },
      20_000,
    );

    expect(latest.opportunities).toHaveLength(1);
    expect(latest.opportunities[0]).toMatchObject({ profitPct: 0.8, timestamp: 20_000 });
  });

  it('atomically replaces the live routes for one symbol from a snapshot', () => {
    const withOtherSymbol = reduceScannerMessage(
      createInitialScannerState(),
      {
        type: 'arbitrage',
        opportunity: {
          symbol: 'BTCUSDT', buy_source: 'gate_spot', sell_source: 'binance_spot',
          buy_price: 64_000, sell_price: 64_100, profit_pct: 0.15, timestamp: 9_000,
        },
      },
      9_000,
    );
    const snapshot = reduceScannerMessage(
      withOtherSymbol,
      {
        type: 'opportunities_snapshot',
        version: 1,
        symbol: 'COTIUSDT',
        opportunities: [
          {
            symbol: 'COTIUSDT', buy_source: 'gate_spot', sell_source: 'binance_spot',
            buy_price: 0.01131, sell_price: 0.0114, profit_pct: 0.79, timestamp: 10_000,
          },
          {
            symbol: 'COTIUSDT', buy_source: 'gate_futures', sell_source: 'binance_futures',
            buy_price: 0.01132, sell_price: 0.01141, profit_pct: 0.8, timestamp: 10_000,
          },
        ],
      },
      10_000,
    );

    expect(snapshot.opportunities.filter((item) => item.symbol === 'COTIUSDT')).toHaveLength(2);
    expect(snapshot.opportunities.some((item) => item.symbol === 'BTCUSDT')).toBe(true);

    const cleared = reduceScannerMessage(
      snapshot,
      { type: 'opportunities_snapshot', version: 1, symbol: 'COTIUSDT', opportunities: [] },
      11_000,
    );
    expect(cleared.opportunities.filter((item) => item.symbol === 'COTIUSDT')).toHaveLength(0);
    expect(cleared.opportunities.some((item) => item.symbol === 'BTCUSDT')).toBe(true);
  });

  it('counts only sources updated in the last fifteen seconds', () => {
    const state = reduceScannerMessage(
      createInitialScannerState(),
      { type: 'prices', prices: { COTIUSDT: { binance_spot: 0.01, gate_futures: 0.011 } } },
      10_000,
    );

    expect(countFreshSources(state, 'COTIUSDT', 24_999)).toBe(2);
    expect(countFreshSources(state, 'COTIUSDT', 24_999, { binance_spot: true, gate_futures: false })).toBe(1);
    expect(countFreshSources(state, 'COTIUSDT', 25_001)).toBe(0);
  });

  it('stores recent versioned alert triggers without duplicating websocket retries', () => {
    const message = {
      type: 'alert_trigger',
      version: 1,
      trigger: {
        id: 7,
        rule_id: 3,
        rule_name: 'COTI gap',
        symbol: 'COTIUSDT',
        buy_source: 'gate_spot',
        sell_source: 'binance_spot',
        buy_price: 0.011,
        sell_price: 0.012,
        gross_spread_pct: 0.82,
        triggered_at_ms: 20_000,
      },
    };
    const first = reduceScannerMessage(createInitialScannerState(), message, 20_000);
    const repeated = reduceScannerMessage(first, message, 21_000);

    expect(repeated.alertTriggers).toHaveLength(1);
    expect(repeated.alertTriggers[0]).toMatchObject({ id: 7, ruleName: 'COTI gap', grossSpreadPct: 0.82 });
  });
});
