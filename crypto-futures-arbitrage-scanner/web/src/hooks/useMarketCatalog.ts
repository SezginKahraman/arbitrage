import { useCallback, useEffect, useState } from 'react';

import {
  isSymbolName,
  type MarketCandidate,
  type MarketCatalogSource,
  type MarketCatalogSourceStatus,
  type SymbolName,
} from '../app/types';

export type MarketCatalogFetcher = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
type MarketCatalogStatus = 'loading' | 'ready' | 'degraded';

export interface MarketCatalogState {
  candidates: MarketCandidate[];
  sources: MarketCatalogSource[];
  watchlist: SymbolName[];
  limit: number;
  status: MarketCatalogStatus;
  saving: boolean;
  error: string | null;
  replace: (symbols: SymbolName[]) => Promise<void>;
  retry: () => void;
}

const sourceStatuses = new Set<MarketCatalogSourceStatus>(['loading', 'ready', 'stale', 'unavailable']);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringArray(value: unknown): string[] | null {
  return Array.isArray(value) && value.every((item) => typeof item === 'string') ? value : null;
}

function normalizeCandidate(value: unknown): MarketCandidate | null {
  if (!isRecord(value) || !isSymbolName(value.symbol) || typeof value.base !== 'string') return null;
  const spotSources = stringArray(value.spotSources);
  const futuresSources = stringArray(value.futuresSources);
  const sources = stringArray(value.sources);
  if (!spotSources || !futuresSources || !sources || sources.length < 2) return null;
  return { symbol: value.symbol, base: value.base, spotSources, futuresSources, sources };
}

function normalizeSource(value: unknown): MarketCatalogSource | null {
  if (!isRecord(value) || typeof value.source !== 'string' || !value.source) return null;
  if ((value.market !== 'spot' && value.market !== 'futures') || typeof value.status !== 'string' || !sourceStatuses.has(value.status as MarketCatalogSourceStatus)) return null;
  const symbols = stringArray(value.symbols);
  if (!symbols || !symbols.every(isSymbolName) || typeof value.checkedAt !== 'number' || !Number.isFinite(value.checkedAt)) return null;
  if (value.errorCode !== undefined && typeof value.errorCode !== 'string') return null;
  return {
    source: value.source,
    market: value.market,
    status: value.status as MarketCatalogSourceStatus,
    symbols,
    checkedAt: value.checkedAt,
    errorCode: value.errorCode,
  };
}

function normalizeMarkets(value: unknown): { candidates: MarketCandidate[]; sources: MarketCatalogSource[]; limit: number } {
  if (!isRecord(value) || !Array.isArray(value.items) || !Array.isArray(value.sources)) throw new Error('Invalid market catalog');
  const candidates = value.items.map(normalizeCandidate);
  const sources = value.sources.map(normalizeSource);
  if (candidates.some((item) => !item) || sources.some((item) => !item) || !Number.isInteger(value.maxWatchlist)) {
    throw new Error('Invalid market catalog');
  }
  return { candidates: candidates as MarketCandidate[], sources: sources as MarketCatalogSource[], limit: value.maxWatchlist as number };
}

function normalizeWatchlist(value: unknown): { symbols: SymbolName[]; limit: number } {
  if (!isRecord(value) || !Array.isArray(value.symbols) || !value.symbols.every(isSymbolName) || !Number.isInteger(value.limit)) {
    throw new Error('Invalid watchlist');
  }
  return { symbols: value.symbols as SymbolName[], limit: value.limit as number };
}

async function responseError(response: Response): Promise<string> {
  try {
    const payload: unknown = await response.json();
    if (isRecord(payload) && typeof payload.error === 'string' && payload.error) return payload.error;
  } catch {
    // Fall through to a stable status-only message.
  }
  return `Watchlist request failed: ${response.status}`;
}

export function useMarketCatalog(fetcher: MarketCatalogFetcher = fetch): MarketCatalogState {
  const [candidates, setCandidates] = useState<MarketCandidate[]>([]);
  const [sources, setSources] = useState<MarketCatalogSource[]>([]);
  const [watchlist, setWatchlist] = useState<SymbolName[]>([]);
  const [limit, setLimit] = useState(20);
  const [status, setStatus] = useState<MarketCatalogStatus>('loading');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [retryVersion, setRetryVersion] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setStatus('loading');
    setError(null);
    void Promise.all([
      fetcher('/api/markets', { signal: controller.signal }),
      fetcher('/api/watchlist', { signal: controller.signal }),
    ]).then(async ([marketResponse, watchlistResponse]) => {
      if (!marketResponse.ok || !watchlistResponse.ok) throw new Error('Market catalog unavailable');
      const marketData = normalizeMarkets(await marketResponse.json());
      const watchlistData = normalizeWatchlist(await watchlistResponse.json());
      if (controller.signal.aborted) return;
      setCandidates(marketData.candidates);
      setSources(marketData.sources);
      setWatchlist(watchlistData.symbols);
      setLimit(Math.min(marketData.limit, watchlistData.limit));
      setStatus('ready');
    }).catch((reason: unknown) => {
      if (controller.signal.aborted || (reason instanceof DOMException && reason.name === 'AbortError')) return;
      setError(reason instanceof Error ? reason.message : 'Market catalog unavailable');
      setStatus('degraded');
    });
    return () => controller.abort();
  }, [fetcher, retryVersion]);

  const replace = useCallback(async (symbols: SymbolName[]) => {
    setSaving(true);
    setError(null);
    try {
      const response = await fetcher('/api/watchlist', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ symbols }),
      });
      if (!response.ok) throw new Error(await responseError(response));
      const payload = normalizeWatchlist(await response.json());
      setWatchlist(payload.symbols);
      setLimit(payload.limit);
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : 'Could not update watchlist';
      setError(message);
      throw reason;
    } finally {
      setSaving(false);
    }
  }, [fetcher]);

  return {
    candidates, sources, watchlist, limit, status, saving, error, replace,
    retry: () => setRetryVersion((value) => value + 1),
  };
}
