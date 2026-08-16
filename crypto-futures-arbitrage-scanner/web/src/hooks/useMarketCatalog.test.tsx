import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { useMarketCatalog } from './useMarketCatalog';

const marketPayload = {
  items: [{
    symbol: 'LINKUSDT', base: 'LINK', spotSources: ['binance_spot', 'gate_spot'],
    futuresSources: ['kucoin_futures'], sources: ['binance_spot', 'gate_spot', 'kucoin_futures'],
  }],
  sources: [{ source: 'binance_spot', market: 'spot', status: 'ready', symbols: ['LINKUSDT'], checkedAt: 10_000 }],
  maxWatchlist: 20,
};

function jsonResponse(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('useMarketCatalog', () => {
  it('loads the server catalog and persists replacements', async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input) === '/api/markets') return jsonResponse(marketPayload);
      if (String(input) === '/api/watchlist' && init?.method === 'PUT') {
        return jsonResponse({ symbols: ['LINKUSDT'], limit: 20 });
      }
      return jsonResponse({ symbols: ['BTCUSDT'], limit: 20 });
    });
    const { result } = renderHook(() => useMarketCatalog(fetcher));

    await waitFor(() => expect(result.current.status).toBe('ready'));
    expect(result.current.watchlist).toEqual(['BTCUSDT']);
    expect(result.current.candidates[0].symbol).toBe('LINKUSDT');

    await act(async () => result.current.replace(['LINKUSDT']));
    expect(result.current.watchlist).toEqual(['LINKUSDT']);
    expect(fetcher).toHaveBeenCalledWith('/api/watchlist', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ symbols: ['LINKUSDT'] }),
    }));
  });

  it('keeps the current selection when the server rejects an update', async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input) === '/api/markets') return jsonResponse(marketPayload);
      if (init?.method === 'PUT') return jsonResponse({ error: 'unsupported' }, 400);
      return jsonResponse({ symbols: ['BTCUSDT'], limit: 20 });
    });
    const { result } = renderHook(() => useMarketCatalog(fetcher));
    await waitFor(() => expect(result.current.status).toBe('ready'));

    await expect(act(async () => result.current.replace(['LINKUSDT']))).rejects.toThrow('unsupported');
    expect(result.current.watchlist).toEqual(['BTCUSDT']);
  });
});
