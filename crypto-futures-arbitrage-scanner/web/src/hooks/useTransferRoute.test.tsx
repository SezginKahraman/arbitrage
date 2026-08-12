import { renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { useTransferRoute } from './useTransferRoute';

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('useTransferRoute', () => {
  it('loads and validates a directional spot transfer route', async () => {
    const fetcher = vi.fn(async () => jsonResponse({
      asset: 'COTI',
      source: 'gate_spot',
      destination: 'kucoin_spot',
      status: 'check',
      reason: 'common network requires verification',
      checked_at: 20_000,
      networks: [{
        network_id: 'coti_evm', name: 'COTI', status: 'check', reason: 'network alias requires verification',
        source_withdraw_enabled: true, destination_deposit_enabled: true,
        withdrawal_fee: '', minimum_withdrawal: '0.9827044', contract_address: '',
      }],
      source_networks: [{
        asset: 'COTI', network_id: 'coti_evm', raw_network_id: 'COTI', name: 'COTI',
        deposit_enabled: true, withdraw_enabled: true, minimum_withdrawal: '0.9827044', checked_at: 20_000,
      }],
      destination_networks: [{
        asset: 'COTI', network_id: 'coti_evm', raw_network_id: 'cotievm', name: 'COTI',
        deposit_enabled: true, withdraw_enabled: true, withdrawal_fee: '150', minimum_withdrawal: '300', checked_at: 21_000,
      }],
    }));

    const { result } = renderHook(() => useTransferRoute({
      symbol: 'COTIUSDT', source: 'gate_spot', destination: 'kucoin_spot', fetcher, refreshIntervalMs: 60_000,
    }));

    await waitFor(() => expect(result.current.status).toBe('ready'));
    expect(fetcher).toHaveBeenCalledWith(
      '/api/transfer-route?asset=COTI&source=gate_spot&destination=kucoin_spot',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(result.current.route?.status).toBe('check');
    expect(result.current.route?.sourceNetworks[0].withdrawEnabled).toBe(true);
    expect(result.current.route?.destinationNetworks[0].withdrawalFee).toBe('150');
  });

  it('does not call the network API for spot-futures routes', () => {
    const fetcher = vi.fn();
    const { result } = renderHook(() => useTransferRoute({
      symbol: 'COTIUSDT', source: 'gate_futures', destination: 'kucoin_spot', fetcher,
    }));

    expect(fetcher).not.toHaveBeenCalled();
    expect(result.current.status).toBe('ready');
    expect(result.current.route?.status).toBe('not_applicable');
  });

  it('degrades safely when the exchange metadata payload is malformed', async () => {
    const fetcher = vi.fn(async () => jsonResponse({ status: 'ready', networks: 'not-an-array' }));
    const { result } = renderHook(() => useTransferRoute({
      symbol: 'COTIUSDT', source: 'gate_spot', destination: 'kucoin_spot', fetcher,
    }));

    await waitFor(() => expect(result.current.status).toBe('degraded'));
    expect(result.current.route).toBeNull();
  });
});
