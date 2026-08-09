import { describe, expect, it } from 'vitest';

import { DEFAULT_ENABLED_SOURCES, SOURCE_BY_KEY, routeMatchesComparisonMode } from './sources';

describe('KuCoin market sources', () => {
  it('exposes independently selectable Spot and Futures sources', () => {
    expect(SOURCE_BY_KEY.kucoin_spot).toMatchObject({ label: 'KuCoin Spot', market: 'spot' });
    expect(SOURCE_BY_KEY.kucoin_futures).toMatchObject({ label: 'KuCoin Futures', market: 'futures' });
    expect(DEFAULT_ENABLED_SOURCES.kucoin_spot).toBe(true);
    expect(DEFAULT_ENABLED_SOURCES.kucoin_futures).toBe(true);
  });

  it('includes KuCoin in matching market comparison modes', () => {
    expect(routeMatchesComparisonMode('kucoin_spot', 'binance_spot', 'spot')).toBe(true);
    expect(routeMatchesComparisonMode('kucoin_futures', 'gate_futures', 'futures')).toBe(true);
    expect(routeMatchesComparisonMode('kucoin_spot', 'kucoin_futures', 'mixed')).toBe(true);
  });
});
