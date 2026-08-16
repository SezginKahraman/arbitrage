import { X } from 'lucide-react';
import { useEffect, useRef, type Dispatch, type KeyboardEvent as ReactKeyboardEvent, type SetStateAction } from 'react';

import { SYMBOLS, type UiPreferences } from '../../app/types';
import { SOURCES } from '../../lib/sources';

interface SettingsDrawerProps {
  open: boolean;
  preferences: UiPreferences;
  onClose: () => void;
  onPreferencesChange: Dispatch<SetStateAction<UiPreferences>>;
  symbols?: string[];
}

export function SettingsDrawer({ open, preferences, onClose, onPreferencesChange, symbols = [...SYMBOLS] }: SettingsDrawerProps) {
  const panelRef = useRef<HTMLElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!open) return;

    previousFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    closeButtonRef.current?.focus();
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('keydown', handleEscape);
      previousFocusRef.current?.focus();
    };
  }, [onClose, open]);

  const containFocus = (event: ReactKeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Tab') return;
    const focusable = panelRef.current?.querySelectorAll<HTMLElement>(
      'button:not([disabled]), select:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])',
    );
    if (!focusable?.length) return;

    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/55">
      <aside
        aria-labelledby="scanner-settings-title"
        aria-modal="true"
        className="h-full w-full max-w-md overflow-y-auto border-l border-terminal-line bg-terminal-panel p-6 shadow-2xl"
        onKeyDown={containFocus}
        ref={panelRef}
        role="dialog"
      >
        <div className="flex items-center justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.18em] text-signal-mint">Workspace</p>
            <h2 className="mt-1 text-2xl font-semibold" id="scanner-settings-title">Scanner settings</h2>
          </div>
          <button aria-label="Close settings" className="grid size-10 place-items-center rounded-lg border border-terminal-line" onClick={onClose} ref={closeButtonRef} type="button">
            <X aria-hidden="true" size={18} />
          </button>
        </div>

        <label className="mt-8 block text-sm text-slate-300">
          Trading pair
          <select
            className="mt-2 h-11 w-full rounded-lg border border-terminal-line bg-terminal-ink px-3"
            onChange={(event) => onPreferencesChange((current) => ({ ...current, symbol: event.target.value as UiPreferences['symbol'] }))}
            value={preferences.symbol}
          >
            {symbols.map((symbol) => <option key={symbol}>{symbol}</option>)}
          </select>
        </label>

        <label className="mt-5 block text-sm text-slate-300">
          Minimum spread (%)
          <input
            className="mt-2 h-11 w-full rounded-lg border border-terminal-line bg-terminal-ink px-3 font-data"
            min="0"
            onChange={(event) => {
              const value = Number(event.target.value);
              if (Number.isFinite(value) && value >= 0) onPreferencesChange((current) => ({ ...current, minSpread: value }));
            }}
            step="0.01"
            type="number"
            value={preferences.minSpread}
          />
        </label>

        <fieldset className="mt-7">
          <legend className="text-sm font-medium">Market sources</legend>
          <p className="mt-1 text-xs text-slate-500">Turn off venues you cannot trade on. The choice applies to Scanner, charts, and Opportunities.</p>
          <div className="mt-3 space-y-2">
            {SOURCES.map((source) => (
              <label className="flex items-center justify-between rounded-lg border border-terminal-line bg-terminal-ink/45 px-3 py-3 text-sm" key={source.key}>
                <span className="flex items-center gap-2"><span className="size-2 rounded-full" style={{ backgroundColor: source.color }} />{source.label}</span>
                <input
                  aria-label={source.label}
                  checked={preferences.enabledSources[source.key] !== false}
                  className="size-4 accent-[#27e58c]"
                  onChange={(event) => onPreferencesChange((current) => ({
                    ...current,
                    enabledSources: { ...current.enabledSources, [source.key]: event.target.checked },
                  }))}
                  type="checkbox"
                />
              </label>
            ))}
          </div>
        </fieldset>
      </aside>
    </div>
  );
}
