import { beforeEach, describe, expect, it } from 'vitest';

import {
  DEFAULT_PREFERENCES,
  PREFERENCES_KEY,
  loadPreferences,
  savePreferences,
} from './preferences';

describe('scanner preferences', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('returns safe defaults when storage is empty or corrupt', () => {
    expect(loadPreferences(localStorage)).toEqual(DEFAULT_PREFERENCES);

    localStorage.setItem(PREFERENCES_KEY, '{bad json');
    expect(loadPreferences(localStorage)).toEqual(DEFAULT_PREFERENCES);
  });

  it('validates fields independently instead of discarding valid values', () => {
    localStorage.setItem(
      PREFERENCES_KEY,
      JSON.stringify({
        symbol: 'COTIUSDT',
        minSpread: -4,
        chartRange: '1h',
        navigationCollapsed: true,
        enabledSources: { binance_spot: false, gate_futures: 'wrong' },
      }),
    );

    expect(loadPreferences(localStorage)).toMatchObject({
      symbol: 'COTIUSDT',
      minSpread: DEFAULT_PREFERENCES.minSpread,
      chartRange: '1h',
      navigationCollapsed: true,
      enabledSources: {
        ...DEFAULT_PREFERENCES.enabledSources,
        binance_spot: false,
        gate_futures: true,
      },
      comparisonMode: 'spot',
      dashboardLayout: 'split',
      opportunitiesCollapsed: false,
      feedTerminalCollapsed: false,
    });
  });

  it('falls back independently for invalid comparison and layout controls', () => {
    localStorage.setItem(
      PREFERENCES_KEY,
      JSON.stringify({
        comparisonMode: 'anything',
        dashboardLayout: 'diagonal',
        opportunitiesCollapsed: 'sometimes',
      }),
    );

    expect(loadPreferences(localStorage)).toMatchObject({
      comparisonMode: 'spot',
      dashboardLayout: 'split',
      opportunitiesCollapsed: false,
    });
  });

  it('migrates the legacy source selection only after saving v1 preferences', () => {
    localStorage.setItem('enabledSources', JSON.stringify({ binance_spot: false }));

    const preferences = loadPreferences(localStorage);

    expect(preferences.enabledSources.binance_spot).toBe(false);
    expect(JSON.parse(localStorage.getItem(PREFERENCES_KEY) ?? '{}')).toEqual(preferences);
    expect(localStorage.getItem('enabledSources')).toBeNull();
  });

  it('round-trips a complete preference object', () => {
    const preferences = {
      ...DEFAULT_PREFERENCES,
      symbol: 'SOLUSDT' as const,
      minSpread: 0.42,
      chartRange: '4h' as const,
      comparisonMode: 'futures' as const,
      dashboardLayout: 'stacked' as const,
      opportunitiesCollapsed: true,
      feedTerminalCollapsed: true,
    };

    savePreferences(localStorage, preferences);

    expect(loadPreferences(localStorage)).toEqual(preferences);
  });
});
