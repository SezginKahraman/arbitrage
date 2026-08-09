import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const chartMocks = vi.hoisted(() => {
  const setData = vi.fn();
  const addSeries = vi.fn(() => ({ setData }));
  const remove = vi.fn();
  const fitContent = vi.fn();
  return {
    addSeries,
    fitContent,
    remove,
    setData,
    createChart: vi.fn(() => ({
      addSeries,
      remove,
      timeScale: () => ({ fitContent }),
    })),
  };
});

vi.mock('lightweight-charts', () => ({
  ColorType: { Solid: 'solid' },
  LineSeries: {},
  createChart: chartMocks.createChart,
}));

import { PriceComparisonChart, priceFormatForValue, toChartData } from './PriceComparisonChart';

describe('PriceComparisonChart', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('deduplicates sub-second points and filters the selected range', () => {
    expect(
      toChartData(
        [
          { time: 0, value: 1 },
          { time: 1_000_100, value: 0.0114 },
          { time: 1_000_800, value: 0.0115 },
          { time: 1_001_000, value: 0.0116 },
        ],
        '15m',
      ),
    ).toEqual([
      { time: 1000, value: 0.0115 },
      { time: 1001, value: 0.0116 },
    ]);
  });

  it('uses eight-decimal chart precision for COTI prices', () => {
    expect(priceFormatForValue(0.01140723)).toEqual({ type: 'price', precision: 8, minMove: 0.00000001 });
  });

  it('creates enabled source series, changes range, and removes the chart on unmount', () => {
    const onRangeChange = vi.fn();
    const { rerender, unmount } = render(
      <PriceComparisonChart
        enabledSources={{ binance_spot: true }}
        history={{ binance_spot: [{ time: 1_000_000, value: 0.01140723 }] }}
        onRangeChange={onRangeChange}
        range="15m"
      />,
    );

    expect(chartMocks.addSeries).toHaveBeenCalledOnce();
    expect(chartMocks.setData).toHaveBeenCalledWith([{ time: 1000, value: 0.01140723 }]);

    fireEvent.click(screen.getByRole('button', { name: '1 hour' }));
    expect(onRangeChange).toHaveBeenCalledWith('1h');

    rerender(
      <PriceComparisonChart
        enabledSources={{ binance_spot: true }}
        history={{ binance_spot: [{ time: 1_000_000, value: 0.01140723 }] }}
        onRangeChange={onRangeChange}
        range="1h"
      />,
    );
    expect(chartMocks.fitContent).toHaveBeenCalledTimes(2);

    unmount();
    expect(chartMocks.remove).toHaveBeenCalledOnce();
  });
});
