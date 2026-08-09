import {
  ColorType,
  LineSeries,
  createChart,
  type IChartApi,
  type ISeriesApi,
  type LineData,
  type UTCTimestamp,
} from 'lightweight-charts';
import { useEffect, useRef } from 'react';

import type { ChartRange, PricePoint } from '../../app/types';
import { precisionForPrice } from '../../lib/format';
import { sourceMeta } from '../../lib/sources';
import { SourceMark } from '../shared/SourceMark';

const RANGE_MS: Record<ChartRange, number> = {
  '15m': 15 * 60 * 1_000,
  '1h': 60 * 60 * 1_000,
  '4h': 4 * 60 * 60 * 1_000,
};

export function priceFormatForValue(value: number) {
  const precision = precisionForPrice(value);
  return { type: 'price' as const, precision, minMove: 10 ** -precision };
}

export function toChartData(points: PricePoint[], range: ChartRange): LineData<UTCTimestamp>[] {
  if (!points.length) return [];

  const latest = Math.max(...points.map((point) => point.time));
  const cutoff = latest - RANGE_MS[range];
  const bySecond = new Map<number, number>();

  for (const point of points) {
    if (point.time < cutoff || !Number.isFinite(point.value)) continue;
    bySecond.set(Math.floor(point.time / 1_000), point.value);
  }

  return [...bySecond.entries()]
    .sort(([left], [right]) => left - right)
    .map(([time, value]) => ({ time: time as UTCTimestamp, value }));
}

interface PriceComparisonChartProps {
  history: Record<string, PricePoint[]>;
  enabledSources: Record<string, boolean>;
  range: ChartRange;
  onRangeChange: (range: ChartRange) => void;
}

const RANGE_LABELS: Record<ChartRange, string> = {
  '15m': '15 minutes',
  '1h': '1 hour',
  '4h': '4 hours',
};

export function PriceComparisonChart({ history, enabledSources, range, onRangeChange }: PriceComparisonChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef(new Map<string, ISeriesApi<'Line'>>());
  const fittedRangeRef = useRef<ChartRange | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      autoSize: true,
      height: 310,
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: '#8a9aa3',
        fontFamily: 'IBM Plex Mono, ui-monospace, monospace',
        fontSize: 11,
        attributionLogo: false,
      },
      grid: {
        vertLines: { color: 'rgba(27,43,51,0.72)' },
        horzLines: { color: 'rgba(27,43,51,0.72)' },
      },
      rightPriceScale: { borderColor: '#1b2b33' },
      timeScale: { borderColor: '#1b2b33', timeVisible: true, secondsVisible: false },
      crosshair: { vertLine: { color: '#56666e' }, horzLine: { color: '#56666e' } },
    });

    chartRef.current = chart;
    return () => {
      seriesRef.current.clear();
      chart.remove();
      chartRef.current = null;
    };
  }, []);

  useEffect(() => {
    const chart = chartRef.current;
    if (!chart) return;

    let hasData = false;
    for (const [source, points] of Object.entries(history)) {
      let series = seriesRef.current.get(source);
      if (!series) {
        const latestValue = points.at(-1)?.value ?? 1;
        series = chart.addSeries(LineSeries, {
          color: sourceMeta(source).color,
          lineWidth: 2,
          lastValueVisible: true,
          priceLineVisible: false,
          crosshairMarkerVisible: true,
          priceFormat: priceFormatForValue(latestValue),
          title: sourceMeta(source).label,
        });
        seriesRef.current.set(source, series);
      }

      const data = enabledSources[source] === false ? [] : toChartData(points, range);
      series.setData(data);
      hasData ||= data.length > 0;
    }

    if (hasData && fittedRangeRef.current !== range) {
      chart.timeScale().fitContent();
      fittedRangeRef.current = range;
    }
  }, [enabledSources, history, range]);

  const activeSources = Object.keys(history).filter((source) => enabledSources[source] !== false);

  return (
    <section className="overflow-hidden rounded-xl border border-terminal-line bg-terminal-panel/65">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-terminal-line px-5 py-4">
        <div>
          <h2 className="font-display text-lg font-medium">Price comparison</h2>
          <div className="mt-2 flex flex-wrap gap-4 text-xs text-slate-400">
            {activeSources.map((source) => <SourceMark compact key={source} source={source} />)}
          </div>
        </div>
        <div aria-label="Chart range" className="flex rounded-lg border border-terminal-line bg-terminal-ink/60 p-1">
          {(Object.keys(RANGE_LABELS) as ChartRange[]).map((chartRange) => (
            <button
              aria-label={RANGE_LABELS[chartRange]}
              aria-pressed={range === chartRange}
              className={`rounded-md px-3 py-1.5 font-data text-xs ${range === chartRange ? 'bg-white/10 text-terminal-text' : 'text-slate-500 hover:text-terminal-text'}`}
              key={chartRange}
              onClick={() => onRangeChange(chartRange)}
              type="button"
            >
              {chartRange}
            </button>
          ))}
        </div>
      </header>
      <div className="relative min-h-[310px] px-2 py-3">
        <div className="h-[310px] w-full" ref={containerRef} />
        {!activeSources.length && (
          <div className="pointer-events-none absolute inset-0 grid place-items-center text-sm text-slate-500">
            Enable a source to draw price history.
          </div>
        )}
      </div>
      <footer className="border-t border-terminal-line px-5 py-2 text-right text-[10px] text-slate-600">
        Charts by <a className="underline hover:text-slate-400" href="https://www.tradingview.com/" rel="noreferrer" target="_blank">TradingView</a>
      </footer>
    </section>
  );
}
