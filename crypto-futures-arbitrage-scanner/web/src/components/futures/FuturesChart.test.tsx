import { render } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { FuturesAnalysis } from '../../app/types';

const chartMocks = vi.hoisted(() => {
  const candleSetData = vi.fn();
  const volumeSetData = vi.fn();
  const createPriceLine = vi.fn();
  const candleUpdate = vi.fn();
  const volumeUpdate = vi.fn();
  const remove = vi.fn();
  const fitContent = vi.fn();
  const addSeries = vi.fn()
    .mockImplementationOnce(() => ({ setData: candleSetData, update: candleUpdate, createPriceLine }))
    .mockImplementationOnce(() => ({ setData: volumeSetData, update: volumeUpdate }));
  const createChart = vi.fn(() => ({
    addSeries,
    remove,
    priceScale: () => ({ applyOptions: vi.fn() }),
    timeScale: () => ({ fitContent }),
  }));
  return { candleSetData, volumeSetData, candleUpdate, volumeUpdate, createPriceLine, remove, fitContent, addSeries, createChart };
});

vi.mock('lightweight-charts', () => ({
  CandlestickSeries: { kind: 'candles' },
  HistogramSeries: { kind: 'volume' },
  ColorType: { Solid: 'solid' },
  LineStyle: { Dashed: 2 },
  createChart: chartMocks.createChart,
}));

import { FuturesChart, toFuturesChartData } from './FuturesChart';

const analysis = {
  contract: {
    symbol: 'COTI_USDT', status: 'trading', quantoMultiplier: 1, markPrice: 0.0108, indexPrice: 0.01079,
    lastPrice: 0.0108, makerFeeRate: -0.0001, takerFeeRate: 0.00075, orderSizeMin: 1,
    orderSizeMax: 4_500_000, priceIncrement: 0.00001, leverageMin: 1, leverageMax: 25,
    fundingRate: -0.002, fundingInterval: 14_400,
  },
  interval: '15m',
  strategy: 'trend_pullback',
  analyzedAt: 1_700_000_000_000,
  candles: [
    { time: 1_700_000_000, open: 0.0104, high: 0.0107, low: 0.0103, close: 0.0106, volume: 1000 },
    { time: 1_700_000_900, open: 0.0106, high: 0.0109, low: 0.0105, close: 0.0108, volume: 1200 },
  ],
  indicators: {
    ema20: 0.0105, ema50: 0.0102, atr14: 0.0002, rsi14: 58, mean20: 0.0104,
    upperBand: 0.0108, lowerBand: 0.0100, swingHigh: 0.0109, swingLow: 0.0099, marketBias: 'bullish',
  },
  liquidity: { bidWalls: [], askWalls: [] },
  long: {
    direction: 'long', status: 'ready', score: 80, entry: 0.0105, stop: 0.0100, targets: [0.0115],
    riskReward: 2, riskAmount: 10, contracts: 20_000, baseQuantity: 20_000, notional: 210,
    estimatedFees: 0.315, liquidationNote: 'Not estimated.', reasons: ['Trend aligned.'],
  },
  short: {
    direction: 'short', status: 'no_trade', score: 20, entry: 0.0105, stop: 0.0110, targets: [0.0095],
    riskReward: 2, riskAmount: 10, contracts: 20_000, baseQuantity: 20_000, notional: 210,
    estimatedFees: 0.315, liquidationNote: 'Not estimated.', reasons: ['Countertrend.'],
  },
  neutralRoute: null,
} as FuturesAnalysis;

describe('FuturesChart', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    chartMocks.addSeries
      .mockImplementationOnce(() => ({ setData: chartMocks.candleSetData, update: chartMocks.candleUpdate, createPriceLine: chartMocks.createPriceLine }))
      .mockImplementationOnce(() => ({ setData: chartMocks.volumeSetData, update: chartMocks.volumeUpdate }));
  });

  it('maps candles and volume without deriving values from the chart library', () => {
    expect(toFuturesChartData(analysis.candles)).toEqual({
      candles: [
        { time: 1_700_000_000, open: 0.0104, high: 0.0107, low: 0.0103, close: 0.0106 },
        { time: 1_700_000_900, open: 0.0106, high: 0.0109, low: 0.0105, close: 0.0108 },
      ],
      volume: [
        { time: 1_700_000_000, value: 1000, color: 'rgba(39, 229, 140, 0.34)' },
        { time: 1_700_000_900, value: 1200, color: 'rgba(39, 229, 140, 0.34)' },
      ],
    });
  });

  it('draws the selected scenario entry, stop, and target and cleans up', () => {
    const { unmount } = render(<FuturesChart analysis={analysis} direction="long" />);

    expect(chartMocks.candleSetData).toHaveBeenCalledOnce();
    expect(chartMocks.volumeSetData).toHaveBeenCalledOnce();
    expect(chartMocks.createPriceLine).toHaveBeenCalledTimes(3);
    expect(chartMocks.createPriceLine).toHaveBeenCalledWith(expect.objectContaining({ price: 0.0105, title: 'LONG ENTRY' }));
    expect(chartMocks.createPriceLine).toHaveBeenCalledWith(expect.objectContaining({ price: 0.0100, title: 'STOP' }));
    expect(chartMocks.createPriceLine).toHaveBeenCalledWith(expect.objectContaining({ price: 0.0115, title: 'TARGET 1' }));

    unmount();
    expect(chartMocks.remove).toHaveBeenCalledOnce();
  });

  it('uses the light analytical chart palette', () => {
    render(<FuturesChart analysis={analysis} direction="long" />);

    expect(chartMocks.createChart).toHaveBeenCalledWith(expect.anything(), expect.objectContaining({
      layout: expect.objectContaining({ textColor: '#52645d' }),
      grid: {
        vertLines: { color: 'rgba(15, 56, 45, 0.09)' },
        horzLines: { color: 'rgba(15, 56, 45, 0.09)' },
      },
      rightPriceScale: expect.objectContaining({ borderColor: '#d5e0dc' }),
    }));
  });

  it('updates the current candle without recreating the chart', () => {
    const { rerender } = render(<FuturesChart analysis={analysis} direction="long" liveCandle={null} />);
    rerender(<FuturesChart
      analysis={analysis}
      direction="long"
      liveCandle={{ time: 1_700_001_800, open: 0.0108, high: 0.0111, low: 0.0107, close: 0.011, volume: 1_400 }}
    />);

    expect(chartMocks.addSeries).toHaveBeenCalledTimes(2);
    expect(chartMocks.candleUpdate).toHaveBeenCalledWith({
      time: 1_700_001_800, open: 0.0108, high: 0.0111, low: 0.0107, close: 0.011,
    });
    expect(chartMocks.volumeUpdate).toHaveBeenCalledWith({
      time: 1_700_001_800, value: 1_400, color: 'rgba(39, 229, 140, 0.34)',
    });
  });
});
