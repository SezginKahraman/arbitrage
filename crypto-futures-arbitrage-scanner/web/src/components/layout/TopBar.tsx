import { Activity, RefreshCw, Search, Settings } from 'lucide-react';
import type { Dispatch, SetStateAction } from 'react';

import { SYMBOLS, type ConnectionStatus, type UiPreferences } from '../../app/types';

interface TopBarProps {
  connection: ConnectionStatus;
  lastUpdatedAt: number | null;
  preferences: UiPreferences;
  onPreferencesChange: Dispatch<SetStateAction<UiPreferences>>;
  onOpenSettings: () => void;
  showPairSelector?: boolean;
  symbols?: string[];
}

const connectionCopy: Record<ConnectionStatus, string> = {
  connecting: 'Connecting',
  live: 'Live',
  reconnecting: 'Reconnecting',
  offline: 'Offline',
};

export function TopBar({
  connection,
  lastUpdatedAt,
  preferences,
  onPreferencesChange,
  onOpenSettings,
  showPairSelector = true,
  symbols = [...SYMBOLS],
}: TopBarProps) {
  return (
    <header className="flex min-h-20 flex-wrap items-center gap-4 border-b border-terminal-line px-4 py-3 md:px-7">
      <div className="flex min-w-fit items-center gap-3 lg:hidden">
        <span className="grid size-10 place-items-center rounded-xl border border-signal-mint/60 text-signal-mint">
          <Activity aria-hidden="true" size={21} />
        </span>
        <h1 className="font-display text-xl font-semibold">Arbitrage Scanner</h1>
      </div>

      {showPairSelector ? <label className="relative order-3 flex min-w-56 flex-1 items-center md:order-none md:ml-auto md:max-w-md">
        <span className="sr-only">Selected trading pair</span>
        <Search aria-hidden="true" className="pointer-events-none absolute left-4 text-slate-500" size={18} />
        <select
          aria-label="Selected trading pair"
          className="h-11 w-full appearance-none rounded-xl border border-terminal-line bg-terminal-panel pl-11 pr-4 text-sm text-terminal-text"
          onChange={(event) =>
            onPreferencesChange((current) => ({ ...current, symbol: event.target.value as UiPreferences['symbol'] }))
          }
          value={preferences.symbol}
        >
          {symbols.map((symbol) => (
            <option key={symbol} value={symbol}>
              {symbol.replace('USDT', '/USDT')}
            </option>
          ))}
        </select>
      </label> : <div className="hidden flex-1 md:block" />}

      <div
        aria-label="Live market connection"
        className={`ml-auto flex items-center gap-2 text-sm md:ml-0 ${connection === 'live' ? 'text-signal-mint' : 'text-signal-amber'}`}
        role="status"
      >
        <span className={`size-2 rounded-full ${connection === 'live' ? 'bg-signal-mint' : 'bg-signal-amber'}`} />
        {connectionCopy[connection]}
      </div>

      <div className="hidden items-center gap-2 text-xs text-slate-500 xl:flex">
        <span>Updated</span>
        <time className="font-data text-slate-300">
          {lastUpdatedAt ? new Date(lastUpdatedAt).toLocaleTimeString([], { hour12: false }) : '—'}
        </time>
        <RefreshCw aria-hidden="true" size={15} />
      </div>

      <button
        aria-label="Open settings"
        className="grid size-10 place-items-center rounded-lg border border-terminal-line text-slate-400 hover:text-terminal-text lg:hidden"
        onClick={onOpenSettings}
        type="button"
      >
        <Settings aria-hidden="true" size={19} />
      </button>
    </header>
  );
}
