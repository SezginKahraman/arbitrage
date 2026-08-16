import { useEffect, useState } from 'react';

import type { TransferRouteEvaluation, TransferRouteRequestStatus } from '../app/types';
import { parseTransferRoute, type TransferRouteFetcher } from './useTransferRoute';

export interface TransferRoutesState {
  routes: Record<string, TransferRouteEvaluation>;
  status: TransferRouteRequestStatus;
}

export function transferRouteKey(asset: string, source: string, destination: string) {
  return `${asset}:${source}:${destination}`;
}

export function useTransferRoutes(
  fetcher: TransferRouteFetcher = fetch,
  refreshIntervalMs = 60_000,
  refreshKey = '',
): TransferRoutesState {
  const [state, setState] = useState<TransferRoutesState>({ routes: {}, status: 'loading' });

  useEffect(() => {
    let stopped = false;
    let controller: AbortController | null = null;
    const load = async (initial: boolean) => {
      controller?.abort();
      controller = new AbortController();
      if (initial) setState((current) => ({ ...current, status: 'loading' }));
      try {
        const response = await fetcher('/api/transfer-routes', { signal: controller.signal });
        if (!response.ok) throw new Error('transfer routes unavailable');
        const payload: unknown = await response.json();
        if (!payload || typeof payload !== 'object' || !Array.isArray((payload as { items?: unknown }).items)) {
          throw new Error('invalid transfer routes response');
        }
        const items = (payload as { items: unknown[] }).items.map(parseTransferRoute);
        if (items.some((item) => item === null)) throw new Error('invalid transfer route item');
        const routes = Object.fromEntries((items as TransferRouteEvaluation[]).map((route) => [
          transferRouteKey(route.asset, route.source, route.destination), route,
        ]));
        if (!stopped) setState({ routes, status: 'ready' });
      } catch (error) {
        if (!stopped && !(error instanceof DOMException && error.name === 'AbortError')) {
          setState((current) => ({ ...current, status: 'degraded' }));
        }
      }
    };
    void load(true);
    const timer = window.setInterval(() => void load(false), refreshIntervalMs);
    return () => {
      stopped = true;
      controller?.abort();
      window.clearInterval(timer);
    };
  }, [fetcher, refreshIntervalMs, refreshKey]);

  return state;
}
