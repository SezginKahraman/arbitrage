import { useCallback, useState, type Dispatch, type SetStateAction } from 'react';

import type { UiPreferences } from '../app/types';
import { loadPreferences, savePreferences } from '../lib/preferences';

export function usePreferences(): [UiPreferences, Dispatch<SetStateAction<UiPreferences>>] {
  const [preferences, setPreferences] = useState<UiPreferences>(() => loadPreferences(window.localStorage));

  const updatePreferences = useCallback<Dispatch<SetStateAction<UiPreferences>>>((update) => {
    setPreferences((current) => {
      const next = typeof update === 'function' ? update(current) : update;
      savePreferences(window.localStorage, next);
      return next;
    });
  }, []);

  return [preferences, updatePreferences];
}
