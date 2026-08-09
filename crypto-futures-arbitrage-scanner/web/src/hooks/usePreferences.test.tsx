import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';

import { PREFERENCES_KEY } from '../lib/preferences';
import { usePreferences } from './usePreferences';

describe('usePreferences', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('persists functional preference updates', () => {
    const { result } = renderHook(() => usePreferences());

    act(() => {
      result.current[1]((current) => ({ ...current, symbol: 'SOLUSDT', minSpread: 0.4 }));
    });

    expect(result.current[0]).toMatchObject({ symbol: 'SOLUSDT', minSpread: 0.4 });
    expect(JSON.parse(localStorage.getItem(PREFERENCES_KEY) ?? '{}')).toMatchObject({
      symbol: 'SOLUSDT',
      minSpread: 0.4,
    });
  });
});
