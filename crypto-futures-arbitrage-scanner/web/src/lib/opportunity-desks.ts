import type { MarketQuote, PricePoint, ScannerState, SymbolName } from '../app/types';

export const FUTURES_SCAN_SOURCES = [
  'binance_futures',
  'gate_futures',
  'kucoin_futures',
] as const;

export type FuturesScanSource = (typeof FUTURES_SCAN_SOURCES)[number];

export interface FuturesConvergenceCandidate {
  id: string;
  symbol: SymbolName;
  longSource: FuturesScanSource;
  shortSource: FuturesScanSource;
  longEntry: number;
  shortEntry: number;
  grossSpreadPct: number;
  estimatedRoundTripFeePct: number;
  estimatedNetSpreadPct: number;
  timestamp: number;
  support: 'analyze' | 'scan_only';
}

export interface AdaptiveConvergenceSignal extends FuturesConvergenceCandidate {
  baselineMedianPct: number | null;
  deviationFromNormalPct: number | null;
  zScore: number | null;
  sampleCount: number;
  status: 'qualified' | 'watch' | 'insufficient_history';
}

const quoteFreshnessMS = 15_000;

// Conservative screening assumptions, not account-specific exchange fee tiers.
const takerFeeRates: Record<FuturesScanSource, number> = {
  binance_futures: 0.0005,
  gate_futures: 0.00075,
  kucoin_futures: 0.0006,
};

function isFreshQuote(quote: MarketQuote | undefined, now: number): quote is MarketQuote {
  if (!quote || quote.bestBid <= 0 || quote.bestAsk <= 0 || quote.bestBid > quote.bestAsk) return false;
  const age = now - quote.timestamp;
  return age >= 0 && age <= quoteFreshnessMS;
}

function isScanSource(source: string): source is FuturesScanSource {
  return FUTURES_SCAN_SOURCES.includes(source as FuturesScanSource);
}

export function buildFuturesConvergenceCandidates(
  quotes: ScannerState['quotes'],
  enabledSources: Record<string, boolean>,
  now: number,
): FuturesConvergenceCandidate[] {
  const candidates: FuturesConvergenceCandidate[] = [];

  for (const [symbol, quoteMap] of Object.entries(quotes)) {
    const available = Object.entries(quoteMap)
      .filter(([source, quote]) => isScanSource(source) && enabledSources[source] !== false && isFreshQuote(quote, now))
      .map(([, quote]) => quote as MarketQuote & { source: FuturesScanSource });
    if (available.length < 2) continue;

    const longQuote = available.reduce((best, quote) => quote.bestAsk < best.bestAsk ? quote : best);
    const shortQuote = available
      .filter((quote) => quote.source !== longQuote.source)
      .reduce<MarketQuote & { source: FuturesScanSource } | null>(
        (best, quote) => !best || quote.bestBid > best.bestBid ? quote : best,
        null,
      );
    if (!shortQuote) continue;

    const grossSpreadPct = ((shortQuote.bestBid / longQuote.bestAsk) - 1) * 100;
    const estimatedRoundTripFeePct = 2 * (
      takerFeeRates[longQuote.source] + takerFeeRates[shortQuote.source]
    ) * 100;
    const pair = new Set([longQuote.source, shortQuote.source]);
    const support = pair.has('binance_futures') && pair.has('gate_futures') ? 'analyze' : 'scan_only';

    candidates.push({
      id: `${symbol}:${longQuote.source}:${shortQuote.source}`,
      symbol,
      longSource: longQuote.source,
      shortSource: shortQuote.source,
      longEntry: longQuote.bestAsk,
      shortEntry: shortQuote.bestBid,
      grossSpreadPct,
      estimatedRoundTripFeePct,
      estimatedNetSpreadPct: grossSpreadPct - estimatedRoundTripFeePct,
      timestamp: Math.min(longQuote.timestamp, shortQuote.timestamp),
      support,
    });
  }

  return candidates.sort((left, right) => (
    right.estimatedNetSpreadPct - left.estimatedNetSpreadPct || left.symbol.localeCompare(right.symbol)
  ));
}

function median(values: number[]): number {
  const sorted = [...values].sort((left, right) => left - right);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
}

function pairedSpreadHistory(longPoints: PricePoint[], shortPoints: PricePoint[]): number[] {
  const shortByTime = new Map(shortPoints.map((point) => [point.time, point.value]));
  return longPoints.flatMap((longPoint) => {
    const shortValue = shortByTime.get(longPoint.time);
    if (!shortValue || longPoint.value <= 0) return [];
    return [((shortValue / longPoint.value) - 1) * 100];
  });
}

export function buildAdaptiveConvergenceSignals(
  candidates: FuturesConvergenceCandidate[],
  history: ScannerState['history'],
): AdaptiveConvergenceSignal[] {
  return candidates.map((candidate) => {
    const routeHistory = history[candidate.symbol] ?? {};
    const spreads = pairedSpreadHistory(
      routeHistory[candidate.longSource] ?? [],
      routeHistory[candidate.shortSource] ?? [],
    ).slice(-720);
    if (spreads.length < 720) {
      return {
        ...candidate,
        baselineMedianPct: null,
        deviationFromNormalPct: null,
        zScore: null,
        sampleCount: spreads.length,
        status: 'insufficient_history' as const,
      };
    }

    const baselineMedianPct = median(spreads);
    const absoluteDeviations = spreads.map((spread) => Math.abs(spread - baselineMedianPct));
    const robustDeviation = Math.max(median(absoluteDeviations) * 1.4826, 0.01);
    const deviationFromNormalPct = candidate.grossSpreadPct - baselineMedianPct;
    const zScore = deviationFromNormalPct / robustDeviation;
    const qualified = deviationFromNormalPct >= 0.4 && zScore >= 2 && candidate.estimatedNetSpreadPct > 0;

    return {
      ...candidate,
      baselineMedianPct,
      deviationFromNormalPct,
      zScore,
      sampleCount: spreads.length,
      status: qualified ? 'qualified' as const : 'watch' as const,
    };
  }).sort((left, right) => (
    (right.deviationFromNormalPct ?? Number.NEGATIVE_INFINITY) -
    (left.deviationFromNormalPct ?? Number.NEGATIVE_INFINITY)
  ));
}
