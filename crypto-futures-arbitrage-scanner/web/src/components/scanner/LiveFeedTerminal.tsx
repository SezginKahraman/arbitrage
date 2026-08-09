import { ChevronDown, ChevronUp, Circle, Radio, Trash2 } from 'lucide-react';
import { memo, useMemo, useState } from 'react';

import type { FeedEvent, SymbolName } from '../../app/types';
import { formatPrice } from '../../lib/format';
import { sourceMeta } from '../../lib/sources';

interface LiveFeedTerminalProps {
  collapsed: boolean;
  events: FeedEvent[];
  onCollapsedChange: (collapsed: boolean) => void;
  symbol: SymbolName;
}

function eventMatchesSymbol(event: FeedEvent, symbol: SymbolName): boolean {
  return event.symbol === symbol || (event.kind === 'connection' && event.symbols?.includes(symbol) === true);
}

function eventSource(event: FeedEvent): string {
  if (event.source) return sourceMeta(event.source).label;
  if (event.buySource && event.sellSource) {
    return `${sourceMeta(event.buySource).label} → ${sourceMeta(event.sellSource).label}`;
  }
  return 'Scanner';
}

function eventDetail(event: FeedEvent): string {
  switch (event.kind) {
    case 'quote':
      return `bid ${formatPrice(event.bestBid ?? Number.NaN)} · ask ${formatPrice(event.bestAsk ?? Number.NaN)}`;
    case 'price':
      return `reference ${formatPrice(event.price ?? Number.NaN)}`;
    case 'connection':
      return event.connected ? 'feed connected' : 'feed disconnected · reconnecting';
    case 'opportunity':
      return `spread +${(event.profitPct ?? 0).toFixed(2)}% detected`;
    case 'alert':
      return `alert triggered at +${(event.profitPct ?? 0).toFixed(2)}%`;
  }
}

function eventLabel(event: FeedEvent): string {
  switch (event.kind) {
    case 'quote': return 'BOOK';
    case 'price': return 'PRICE';
    case 'connection': return event.connected ? 'ONLINE' : 'OFFLINE';
    case 'opportunity': return 'ROUTE';
    case 'alert': return 'ALERT';
  }
}

function formatEventTime(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString('en-GB', {
    hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3,
  });
}

export const LiveFeedTerminal = memo(function LiveFeedTerminal({ collapsed, events, onCollapsedChange, symbol }: LiveFeedTerminalProps) {
  const [hiddenEventIDs, setHiddenEventIDs] = useState<Set<string>>(() => new Set());
  const visibleEvents = useMemo(
    () => events.filter((event) => eventMatchesSymbol(event, symbol) && !hiddenEventIDs.has(event.id)),
    [events, hiddenEventIDs, symbol],
  );

  return (
    <section className="overflow-hidden rounded-xl border border-terminal-line bg-[#061014] shadow-[inset_0_1px_rgba(255,255,255,0.025)]" aria-label="Live feed terminal">
      <header className="flex min-h-14 items-center justify-between gap-3 border-b border-terminal-line px-4 py-3">
        <div className="flex min-w-0 items-center gap-3">
          <span className="grid size-8 shrink-0 place-items-center rounded-lg border border-signal-mint/20 bg-signal-mint/10 text-signal-mint">
            <Radio size={16} />
          </span>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="font-semibold text-slate-100">Live feed terminal</h2>
              <span className="flex items-center gap-1.5 font-data text-[10px] uppercase tracking-[0.16em] text-signal-mint">
                <Circle className="fill-current" size={7} /> streaming
              </span>
            </div>
            <p className="truncate font-data text-[11px] text-slate-500">5-minute buffer · {symbol.replace('USDT', '/USDT')} · public market-data activity</p>
          </div>
        </div>
        <div className="flex items-center gap-1">
          {!collapsed && (
            <button
              aria-label="Clear live feed terminal"
              className="rounded-lg p-2 text-slate-500 transition hover:bg-white/5 hover:text-slate-200"
              onClick={() => setHiddenEventIDs(new Set(events.map((event) => event.id)))}
              type="button"
            >
              <Trash2 size={16} />
            </button>
          )}
          <button
            aria-expanded={!collapsed}
            aria-label={collapsed ? 'Expand live feed terminal' : 'Collapse live feed terminal'}
            className="rounded-lg border border-terminal-line p-2 text-slate-400 transition hover:border-slate-600 hover:text-slate-100"
            onClick={() => onCollapsedChange(!collapsed)}
            type="button"
          >
            {collapsed ? <ChevronDown size={17} /> : <ChevronUp size={17} />}
          </button>
        </div>
      </header>

      {!collapsed && (
        <div className="max-h-64 min-h-36 overflow-auto font-data text-xs" aria-live="off">
          {visibleEvents.length ? (
            <ol className="divide-y divide-terminal-line/55">
              {visibleEvents.map((event) => (
                <li className="grid gap-2 px-4 py-2.5 text-slate-400 sm:grid-cols-[96px_76px_minmax(140px,220px)_1fr]" key={event.id}>
                  <time className="text-slate-600">{formatEventTime(event.receivedAt)}</time>
                  <span className={event.connected === false ? 'text-amber-400' : 'text-signal-mint'}>{eventLabel(event)}</span>
                  <span className="truncate text-slate-200">{eventSource(event)}</span>
                  <span className="min-w-0 break-words text-slate-400">{eventDetail(event)}</span>
                </li>
              ))}
            </ol>
          ) : (
            <div className="grid min-h-36 place-items-center px-4 text-center text-slate-600">
              Waiting for the next feed event…
            </div>
          )}
        </div>
      )}
    </section>
  );
});
