import { renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { useTransferRoutes } from './useTransferRoutes';

describe('useTransferRoutes', () => {
  it('indexes batch route evaluations by asset and direction', async () => {
    const fetcher = vi.fn(async () => new Response(JSON.stringify({ items: [{
      asset: 'COTI', source: 'gate_spot', destination: 'kucoin_spot', status: 'check',
      reason: 'common network requires verification', checked_at: 10_000,
      networks: [], source_networks: [], destination_networks: [],
    }] }), { status: 200 }));
    const { result } = renderHook(() => useTransferRoutes(fetcher, 60_000));

    await waitFor(() => expect(result.current.status).toBe('ready'));
    expect(result.current.routes['COTI:gate_spot:kucoin_spot']?.status).toBe('check');
    expect(fetcher).toHaveBeenCalledWith('/api/transfer-routes', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  });
});
