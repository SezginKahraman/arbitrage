import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { SymbolName } from '../app/types';
import { useOpportunityHistory } from './useOpportunityHistory';

const payload = {
  items: [
    {
      id: 7,
      symbol: 'COTIUSDT',
      buy_source: 'gate_futures',
      sell_source: 'binance_spot',
      buy_price: 0.01131,
      sell_price: 0.01140723,
      first_spread_pct: 0.8,
      latest_spread_pct: 0.62,
      peak_spread_pct: 0.9,
      started_at_ms: 1_000,
      last_seen_at_ms: 5_000,
      ended_at_ms: 5_000,
    },
  ],
};

describe('useOpportunityHistory', () => {
  it('loads and normalizes filtered history', async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify(payload), { status: 200 }));
    const { result } = renderHook(() =>
      useOpportunityHistory({ symbol: 'COTIUSDT', minSpread: 0.5, fetcher }),
    );

    await waitFor(() => expect(result.current.status).toBe('ready'));

    expect(fetcher).toHaveBeenCalledWith(
      '/api/opportunities?symbol=COTIUSDT&minSpread=0.5&limit=100',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(result.current.items[0]).toMatchObject({
      id: 'history:7',
      profitPct: 0.62,
      peakProfitPct: 0.9,
      historical: true,
    });
  });

  it('keeps a degraded state on HTTP or malformed payload errors and can retry', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce(new Response('nope', { status: 503 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(payload), { status: 200 }));
    const { result } = renderHook(() => useOpportunityHistory({ symbol: 'COTIUSDT', minSpread: 0, fetcher }));

    await waitFor(() => expect(result.current.status).toBe('degraded'));
    act(() => result.current.retry());
    await waitFor(() => expect(result.current.status).toBe('ready'));

    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it('aborts the previous request when filters change', () => {
    const signals: AbortSignal[] = [];
    const fetcher = vi.fn((_url: string, init?: RequestInit) => {
      signals.push(init?.signal as AbortSignal);
      return new Promise<Response>(() => undefined);
    });
    const { rerender, unmount } = renderHook(
      ({ symbol }: { symbol: SymbolName }) => useOpportunityHistory({ symbol, minSpread: 0, fetcher }),
      { initialProps: { symbol: 'COTIUSDT' as SymbolName } },
    );

    rerender({ symbol: 'BTCUSDT' });
    expect(signals[0].aborted).toBe(true);
    unmount();
    expect(signals[1].aborted).toBe(true);
  });

  it('clears results immediately when the active symbol changes', async () => {
    const requests: Array<(response: Response) => void> = [];
    const fetcher = vi.fn(
      () => new Promise<Response>((resolve) => requests.push(resolve)),
    );
    const { result, rerender } = renderHook(
      ({ symbol }: { symbol: SymbolName }) => useOpportunityHistory({ symbol, minSpread: 0, fetcher }),
      { initialProps: { symbol: 'COTIUSDT' as SymbolName } },
    );

    act(() => requests[0](new Response(JSON.stringify(payload), { status: 200 })));
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    rerender({ symbol: 'BTCUSDT' });
    expect(result.current.items).toEqual([]);
  });
});
