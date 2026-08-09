import { useCallback, useEffect, useState } from 'react';

import { SYMBOLS, type ArbitrageOpportunity, type SymbolName } from '../app/types';

type HistoryStatus = 'loading' | 'ready' | 'degraded';
type HistoryFetcher = (input: string, init?: RequestInit) => Promise<Response>;

interface UseOpportunityHistoryOptions {
  symbol: SymbolName;
  minSpread: number;
  fetcher?: HistoryFetcher;
}

export interface OpportunityHistoryState {
  items: ArbitrageOpportunity[];
  status: HistoryStatus;
  retry: () => void;
}

interface OpportunityPayload {
  id: number;
  symbol: string;
  buy_source: string;
  sell_source: string;
  buy_price: number;
  sell_price: number;
  latest_spread_pct: number;
  peak_spread_pct: number;
  started_at_ms: number;
  last_seen_at_ms: number;
  ended_at_ms: number | null;
}

const symbolNames = new Set<string>(SYMBOLS);

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function normalizeOpportunity(value: unknown): ArbitrageOpportunity | null {
  if (!value || typeof value !== 'object') return null;

  const item = value as Partial<OpportunityPayload>;
  if (
    !Number.isInteger(item.id) ||
    typeof item.symbol !== 'string' ||
    !symbolNames.has(item.symbol) ||
    typeof item.buy_source !== 'string' ||
    typeof item.sell_source !== 'string' ||
    !isFiniteNumber(item.buy_price) ||
    !isFiniteNumber(item.sell_price) ||
    !isFiniteNumber(item.latest_spread_pct) ||
    !isFiniteNumber(item.peak_spread_pct) ||
    !isFiniteNumber(item.started_at_ms) ||
    !isFiniteNumber(item.last_seen_at_ms) ||
    !(item.ended_at_ms === null || item.ended_at_ms === undefined || isFiniteNumber(item.ended_at_ms))
  ) {
    return null;
  }

  return {
    id: `history:${item.id}`,
    symbol: item.symbol as SymbolName,
    buySource: item.buy_source,
    sellSource: item.sell_source,
    buyPrice: item.buy_price,
    sellPrice: item.sell_price,
    profitPct: item.latest_spread_pct,
    peakProfitPct: item.peak_spread_pct,
    timestamp: item.last_seen_at_ms,
    startedAt: item.started_at_ms,
    endedAt: item.ended_at_ms ?? null,
    historical: true,
  };
}

function normalizePayload(value: unknown): ArbitrageOpportunity[] {
  if (!value || typeof value !== 'object' || !Array.isArray((value as { items?: unknown }).items)) {
    throw new Error('Invalid opportunity history payload');
  }

  const items = (value as { items: unknown[] }).items.map(normalizeOpportunity);
  if (items.some((item) => item === null)) throw new Error('Invalid opportunity history item');
  return items as ArbitrageOpportunity[];
}

export function useOpportunityHistory({
  symbol,
  minSpread,
  fetcher = fetch,
}: UseOpportunityHistoryOptions): OpportunityHistoryState {
  const [items, setItems] = useState<ArbitrageOpportunity[]>([]);
  const [status, setStatus] = useState<HistoryStatus>('loading');
  const [retryVersion, setRetryVersion] = useState(0);
  const retry = useCallback(() => setRetryVersion((current) => current + 1), []);

  useEffect(() => {
    const controller = new AbortController();
    const query = new URLSearchParams({
      symbol,
      minSpread: String(minSpread),
      limit: '100',
    });

    setItems([]);
    setStatus('loading');
    void fetcher(`/api/opportunities?${query.toString()}`, { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) throw new Error(`Opportunity history request failed: ${response.status}`);
        const payload: unknown = await response.json();
        if (!controller.signal.aborted) {
          setItems(normalizePayload(payload));
          setStatus('ready');
        }
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || (error instanceof DOMException && error.name === 'AbortError')) return;
        setStatus('degraded');
      });

    return () => controller.abort();
  }, [fetcher, minSpread, retryVersion, symbol]);

  return { items, status, retry };
}
