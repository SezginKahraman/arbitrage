import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { createInitialScannerState } from '../../lib/market-state';
import { AlertsPage } from './AlertsPage';

function jsonResponse(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('AlertsPage', () => {
  it('loads persisted rules and recent triggers', async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL) => {
      const target = String(input);
      if (target.startsWith('/api/alert-rules')) {
        return jsonResponse({ items: [{
          id: 1, name: 'COTI gap', symbol: 'COTIUSDT', market_mode: 'spot', buy_source: '', sell_source: '',
          min_spread_pct: 0.8, cooldown_seconds: 300, enabled: true, browser_enabled: true,
          created_at_ms: 1_000, updated_at_ms: 1_000, last_triggered_at_ms: 20_000,
        }] });
      }
      return jsonResponse({ items: [{
        id: 7, rule_id: 1, rule_name: 'COTI gap', symbol: 'COTIUSDT', buy_source: 'gate_spot',
        sell_source: 'binance_spot', buy_price: 0.011, sell_price: 0.012,
        gross_spread_pct: 0.82, triggered_at_ms: 20_000,
      }] });
    });

    render(<AlertsPage fetcher={fetcher} state={createInitialScannerState()} />);

    expect(await screen.findAllByText('COTI gap')).toHaveLength(2);
    expect(screen.getByText('Recent triggers')).toBeInTheDocument();
    expect(screen.getByText('+0.82%')).toBeInTheDocument();
    expect(screen.getByText('1 active rule')).toBeInTheDocument();
  });

  it('creates a browser alert rule and updates the rule list', async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const target = String(input);
      if (init?.method === 'POST') {
        const body = JSON.parse(String(init.body));
        return jsonResponse({ id: 9, ...body, created_at_ms: 30_000, updated_at_ms: 30_000, last_triggered_at_ms: null }, 201);
      }
      if (target.startsWith('/api/alert-rules')) return jsonResponse({ items: [] });
      return jsonResponse({ items: [] });
    });
    render(<AlertsPage fetcher={fetcher} state={createInitialScannerState()} />);
    await screen.findByText('No alert rules yet.');

    fireEvent.change(screen.getByLabelText('Rule name'), { target: { value: 'SOL mixed gap' } });
    fireEvent.change(screen.getByLabelText('Alert pair'), { target: { value: 'SOLUSDT' } });
    fireEvent.change(screen.getByLabelText('Alert market type'), { target: { value: 'mixed' } });
    fireEvent.change(screen.getByLabelText('Minimum alert spread'), { target: { value: '0.65' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save alert' }));

    expect(await screen.findByText('SOL mixed gap')).toBeInTheDocument();
    await waitFor(() => expect(fetcher).toHaveBeenCalledWith('/api/alert-rules', expect.objectContaining({ method: 'POST' })));
  });

  it('mutes an enabled rule through the persisted API', async () => {
    const rule = {
      id: 1, name: 'BTC gap', symbol: 'BTCUSDT', market_mode: 'all', buy_source: '', sell_source: '',
      min_spread_pct: 0.5, cooldown_seconds: 60, enabled: true, browser_enabled: true,
      created_at_ms: 1_000, updated_at_ms: 1_000, last_triggered_at_ms: null,
    };
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === 'PUT') return jsonResponse({ ...rule, enabled: false, updated_at_ms: 2_000 });
      if (String(input).startsWith('/api/alert-rules')) return jsonResponse({ items: [rule] });
      return jsonResponse({ items: [] });
    });
    render(<AlertsPage fetcher={fetcher} state={createInitialScannerState()} />);

    const toggle = await screen.findByRole('switch', { name: 'Mute BTC gap' });
    fireEvent.click(toggle);

    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'false'));
    expect(fetcher).toHaveBeenCalledWith('/api/alert-rules/1', expect.objectContaining({ method: 'PUT' }));
  });
});
