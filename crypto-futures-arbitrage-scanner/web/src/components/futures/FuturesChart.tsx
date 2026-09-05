import {
  CandlestickSeries,
  ColorType,
  HistogramSeries,
  LineStyle,
  createChart,
  type CandlestickData,
  type HistogramData,
  type ISeriesApi,
  type UTCTimestamp,
} from 'lightweight-charts';
import { useEffect, useRef } from 'react';

import type { FuturesAnalysis, FuturesCandle } from '../../app/types';
import { precisionForPrice } from '../../lib/format';

export function toFuturesChartData(candles: FuturesCandle[]): {
  candles: CandlestickData<UTCTimestamp>[];
  volume: HistogramData<UTCTimestamp>[];
} {
  return {
    candles: candles.map(({ time, open, high, low, close }) => ({
      time: time as UTCTimestamp, open, high, low, close,
    })),
    volume: candles.map(({ time, open, close, volume }) => ({
      time: time as UTCTimestamp,
      value: volume,
      color: close >= open ? 'rgba(39, 229, 140, 0.34)' : 'rgba(255, 107, 107, 0.32)',
    })),
  };
}

interface FuturesChartProps {
  analysis: FuturesAnalysis;
  direction: 'long' | 'short';
  liveCandle?: FuturesCandle | null;
}

export function FuturesChart({ analysis, direction, liveCandle = null }: FuturesChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const candleSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<'Histogram'> | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    const precision = precisionForPrice(analysis.contract.markPrice);
    const chart = createChart(containerRef.current, {
      autoSize: true,
      height: 500,
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: '#52645d',
        fontFamily: 'IBM Plex Mono, ui-monospace, monospace',
        fontSize: 11,
        attributionLogo: false,
      },
      grid: {
        vertLines: { color: 'rgba(15, 56, 45, 0.09)' },
        horzLines: { color: 'rgba(15, 56, 45, 0.09)' },
      },
      rightPriceScale: { borderColor: '#d5e0dc', scaleMargins: { top: 0.08, bottom: 0.22 } },
      timeScale: { borderColor: '#d5e0dc', timeVisible: true, secondsVisible: false },
      crosshair: { vertLine: { color: '#82938d' }, horzLine: { color: '#82938d' } },
    });
    const candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#087a4c', downColor: '#c2413b', wickUpColor: '#087a4c', wickDownColor: '#c2413b',
      borderVisible: false,
      priceFormat: { type: 'price', precision, minMove: 10 ** -precision },
    });
    const volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: { type: 'volume' }, priceScaleId: 'volume',
    });
    candleSeriesRef.current = candleSeries;
    volumeSeriesRef.current = volumeSeries;
    chart.priceScale('volume').applyOptions({ scaleMargins: { top: 0.82, bottom: 0 } });
    const data = toFuturesChartData(analysis.candles);
    candleSeries.setData(data.candles);
    volumeSeries.setData(data.volume);

    const scenario = analysis[direction];
    const directionColor = direction === 'long' ? '#087a4c' : '#c2413b';
    candleSeries.createPriceLine({
      price: scenario.entry, color: directionColor, lineWidth: 2, lineStyle: LineStyle.Dashed,
      axisLabelVisible: true, title: `${direction.toUpperCase()} ENTRY`,
    });
    candleSeries.createPriceLine({
      price: scenario.stop, color: '#c2413b', lineWidth: 1, lineStyle: LineStyle.Dashed,
      axisLabelVisible: true, title: 'STOP',
    });
    scenario.targets.forEach((target, index) => candleSeries.createPriceLine({
      price: target, color: '#2563eb', lineWidth: 1, lineStyle: LineStyle.Dashed,
      axisLabelVisible: true, title: `TARGET ${index + 1}`,
    }));
    chart.timeScale().fitContent();
    return () => {
      candleSeriesRef.current = null;
      volumeSeriesRef.current = null;
      chart.remove();
    };
  }, [analysis, direction]);

  useEffect(() => {
    if (!liveCandle || !candleSeriesRef.current || !volumeSeriesRef.current) return;
    candleSeriesRef.current.update({
      time: liveCandle.time as UTCTimestamp,
      open: liveCandle.open, high: liveCandle.high, low: liveCandle.low, close: liveCandle.close,
    });
    volumeSeriesRef.current.update({
      time: liveCandle.time as UTCTimestamp,
      value: liveCandle.volume,
      color: liveCandle.close >= liveCandle.open ? 'rgba(39, 229, 140, 0.34)' : 'rgba(255, 107, 107, 0.32)',
    });
  }, [liveCandle]);

  return (
    <section className="overflow-hidden rounded-2xl border border-terminal-line bg-terminal-panel/60">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-terminal-line px-5 py-4">
        <div>
          <p className="font-data text-[10px] uppercase tracking-[0.22em] text-slate-500">Price structure</p>
          <h2 className="mt-1 text-lg font-medium">{analysis.contract.symbol.replace('_', '/')} · {analysis.interval}</h2>
        </div>
        <div className="flex gap-4 font-data text-[11px] text-slate-400">
          <span><i className="mr-2 inline-block size-2 rounded-full bg-signal-mint" />Long</span>
          <span><i className="mr-2 inline-block size-2 rounded-full bg-[#ff6b6b]" />Short / stop</span>
          <span><i className="mr-2 inline-block size-2 rounded-full bg-[#6ea8ff]" />Target</span>
        </div>
      </header>
      <div className="h-[500px] w-full px-1 py-2" ref={containerRef} />
      <footer className="border-t border-terminal-line px-5 py-2 text-right text-[10px] text-slate-600">
        Charts by <a className="underline hover:text-slate-400" href="https://www.tradingview.com/" rel="noreferrer" target="_blank">TradingView</a>
      </footer>
    </section>
  );
}
