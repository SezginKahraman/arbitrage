import { ArrowDown, ArrowUp, ChevronsUpDown, Search, SlidersHorizontal } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';

import type { ArbitrageOpportunity, ScannerState } from '../../app/types';
import { formatPrice } from '../../lib/format';
import { FRESHNESS_WINDOW_MS } from '../../lib/market-state';
import { SOURCES, sourceMeta } from '../../lib/sources';
import { SourceMark } from '../shared/SourceMark';

type MarketFilter = 'all' | 'spot' | 'mixed' | 'futures';
type SortField = 'symbol' | 'buy' | 'sell' | 'spread' | 'updated';
type SortDirection = 'asc' | 'desc';

interface AllOpportunitiesPageProps {
  state: ScannerState;
  now?: number;
}

const pageSize = 10;

const marketFilters: Array<{ key: MarketFilter; label: string; aria: string }> = [
  { key: 'all', label: 'All', aria: 'Show all market routes' },
  { key: 'spot', label: 'Spot ↔ Spot', aria: 'Show spot to spot routes' },
  { key: 'mixed', label: 'Spot ↔ Futures', aria: 'Show spot to futures routes' },
  { key: 'futures', label: 'Futures ↔ Futures', aria: 'Show futures to futures routes' },
];

function routeMarket(opportunity: ArbitrageOpportunity): Exclude<MarketFilter, 'all'> {
  const buyMarket = sourceMeta(opportunity.buySource).market;
  const sellMarket = sourceMeta(opportunity.sellSource).market;
  if (buyMarket === 'spot' && sellMarket === 'spot') return 'spot';
  if (buyMarket === 'futures' && sellMarket === 'futures') return 'futures';
  return 'mixed';
}

function compare(left: ArbitrageOpportunity, right: ArbitrageOpportunity, field: SortField, direction: SortDirection) {
  let value: number;
  switch (field) {
    case 'symbol':
      value = left.symbol.localeCompare(right.symbol);
      break;
    case 'buy':
      value = left.buySource.localeCompare(right.buySource);
      break;
    case 'sell':
      value = left.sellSource.localeCompare(right.sellSource);
      break;
    case 'updated':
      value = left.timestamp - right.timestamp;
      break;
    case 'spread':
      value = left.profitPct - right.profitPct;
      break;
  }
  return (direction === 'asc' ? value : -value) || left.id.localeCompare(right.id);
}

function relativeTime(timestamp: number, now: number) {
  const seconds = Math.max(0, Math.floor((now - timestamp) / 1_000));
  if (seconds < 2) return 'now';
  if (seconds < 60) return `${seconds}s ago`;
  return `${Math.floor(seconds / 60)}m ago`;
}

function SortIcon({ active, direction }: { active: boolean; direction: SortDirection }) {
  if (!active) return <ChevronsUpDown aria-hidden="true" className="opacity-40" size={13} />;
  return direction === 'asc' ? (
    <ArrowUp aria-hidden="true" className="text-signal-mint" size={13} />
  ) : (
    <ArrowDown aria-hidden="true" className="text-signal-mint" size={13} />
  );
}

export function AllOpportunitiesPage({ state, now }: AllOpportunitiesPageProps) {
  const [clock, setClock] = useState(() => now ?? Date.now());
  const [query, setQuery] = useState('');
  const [market, setMarket] = useState<MarketFilter>('all');
  const [exchange, setExchange] = useState('all');
  const [minimumSpread, setMinimumSpread] = useState('0.05');
  const [sortField, setSortField] = useState<SortField>('spread');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');
  const [page, setPage] = useState(1);

  useEffect(() => {
    if (now !== undefined) return undefined;
    const timer = window.setInterval(() => setClock(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, [now]);

  const currentTime = now ?? clock;

  const minimum = Number(minimumSpread);
  const normalizedQuery = query.trim().toLowerCase();
  const opportunities = useMemo(
    () => state.opportunities
      .filter((item) => {
        const age = currentTime - item.timestamp;
        if (age < 0 || age > FRESHNESS_WINDOW_MS) return false;
        if (Number.isFinite(minimum) && item.profitPct < minimum) return false;
        if (market !== 'all' && routeMarket(item) !== market) return false;
        if (exchange !== 'all' && item.buySource !== exchange && item.sellSource !== exchange) return false;
        if (!normalizedQuery) return true;
        const searchable = [
          item.symbol,
          item.symbol.replace('USDT', '/USDT'),
          sourceMeta(item.buySource).label,
          sourceMeta(item.sellSource).label,
        ].join(' ').toLowerCase();
        return searchable.includes(normalizedQuery);
      })
      .sort((left, right) => compare(left, right, sortField, sortDirection)),
    [currentTime, exchange, market, minimum, normalizedQuery, sortDirection, sortField, state.opportunities],
  );

  useEffect(() => setPage(1), [exchange, market, minimumSpread, normalizedQuery]);

  const pageCount = Math.max(1, Math.ceil(opportunities.length / pageSize));
  const visiblePage = Math.min(page, pageCount);
  const visible = opportunities.slice((visiblePage - 1) * pageSize, visiblePage * pageSize);

  const changeSort = (field: SortField) => {
    if (field === sortField) {
      setSortDirection((current) => (current === 'asc' ? 'desc' : 'asc'));
      return;
    }
    setSortField(field);
    setSortDirection(field === 'spread' || field === 'updated' ? 'desc' : 'asc');
  };

  const columns: Array<{ field: SortField; label: string; aria: string; className?: string }> = [
    { field: 'symbol', label: 'Pair', aria: 'pair' },
    { field: 'buy', label: 'Buy venue', aria: 'buy venue' },
    { field: 'sell', label: 'Sell venue', aria: 'sell venue' },
    { field: 'spread', label: 'Gross spread', aria: 'gross spread' },
    { field: 'updated', label: 'Updated', aria: 'updated time', className: 'text-right' },
  ];

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="font-data text-[11px] uppercase tracking-[0.22em] text-signal-mint">Market-wide tape</p>
          <h1 className="mt-1 font-display text-3xl font-semibold tracking-tight">Opportunities</h1>
          <p className="mt-1 text-sm text-slate-400">All live routes across every tracked pair.</p>
        </div>
        <div className="flex items-center gap-2 rounded-full border border-signal-mint/20 bg-signal-mint/[0.07] px-3 py-1.5 font-data text-xs text-signal-mint">
          <span className="size-1.5 animate-pulse rounded-full bg-signal-mint" />
          {opportunities.length} live routes
        </div>
      </header>

      <section aria-label="Opportunity filters" className="rounded-xl border border-terminal-line bg-terminal-panel/65 p-3">
        <div className="grid gap-3 xl:grid-cols-[minmax(240px,1fr)_auto_minmax(180px,0.45fr)_160px]">
          <label className="relative">
            <span className="sr-only">Search opportunities</span>
            <Search aria-hidden="true" className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" size={17} />
            <input
              aria-label="Search opportunities"
              className="h-11 w-full rounded-lg border border-terminal-line bg-terminal-ink/70 pl-10 pr-3 text-sm placeholder:text-slate-600"
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search pair or exchange"
              type="search"
              value={query}
            />
          </label>

          <div aria-label="Opportunity market type" className="flex overflow-x-auto rounded-lg border border-terminal-line bg-terminal-ink/70 p-1" role="group">
            {marketFilters.map((filter) => (
              <button
                aria-label={filter.aria}
                aria-pressed={market === filter.key}
                className={`shrink-0 rounded-md px-3 py-2 text-xs transition ${market === filter.key ? 'bg-signal-mint/12 text-signal-mint shadow-[inset_0_0_0_1px_rgba(39,229,140,0.2)]' : 'text-slate-500 hover:text-terminal-text'}`}
                key={filter.key}
                onClick={() => setMarket(filter.key)}
                type="button"
              >
                {filter.label}
              </button>
            ))}
          </div>

          <label>
            <span className="sr-only">Filter by exchange</span>
            <select
              aria-label="Filter by exchange"
              className="h-11 w-full rounded-lg border border-terminal-line bg-terminal-ink/70 px-3 text-sm"
              onChange={(event) => setExchange(event.target.value)}
              value={exchange}
            >
              <option value="all">All exchanges</option>
              {SOURCES.filter((source) => source.market !== 'oracle').map((source) => (
                <option key={source.key} value={source.key}>{source.label}</option>
              ))}
            </select>
          </label>

          <label className="relative">
            <span className="absolute left-3 top-1.5 flex items-center gap-1 text-[10px] uppercase tracking-wide text-slate-500">
              <SlidersHorizontal aria-hidden="true" size={11} /> Min gross
            </span>
            <input
              aria-label="Minimum gross spread"
              className="h-11 w-full rounded-lg border border-terminal-line bg-terminal-ink/70 px-3 pt-3 font-data text-sm"
              min="0"
              onChange={(event) => setMinimumSpread(event.target.value)}
              step="0.05"
              type="number"
              value={minimumSpread}
            />
            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 font-data text-xs text-slate-500">%</span>
          </label>
        </div>
      </section>

      <section className="overflow-hidden rounded-xl border border-terminal-line bg-terminal-panel/65">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[1040px] text-left text-sm">
            <thead className="bg-black/10 text-[11px] uppercase tracking-wide text-slate-500">
              <tr className="border-b border-terminal-line">
                <th className="w-12 px-4 py-3 text-center font-medium">#</th>
                {columns.slice(0, 3).map((column) => (
                  <th
                    aria-sort={sortField === column.field ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                    className={`px-4 py-3 font-medium ${column.className ?? ''}`}
                    key={column.field}
                  >
                    <button
                      aria-label={`Sort by ${column.aria}`}
                      className={`inline-flex items-center gap-1.5 hover:text-terminal-text ${column.className === 'text-right' ? 'ml-auto' : ''}`}
                      onClick={() => changeSort(column.field)}
                      type="button"
                    >
                      {column.label}
                      <SortIcon active={sortField === column.field} direction={sortDirection} />
                    </button>
                  </th>
                ))}
                <th className="px-4 py-3 font-medium">Market</th>
                {columns.slice(3).map((column) => (
                  <th
                    aria-sort={sortField === column.field ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                    className={`px-4 py-3 font-medium ${column.className ?? ''}`}
                    key={column.field}
                  >
                    <button
                      aria-label={`Sort by ${column.aria}`}
                      className={`inline-flex items-center gap-1.5 hover:text-terminal-text ${column.className === 'text-right' ? 'ml-auto' : ''}`}
                      onClick={() => changeSort(column.field)}
                      type="button"
                    >
                      {column.label}
                      <SortIcon active={sortField === column.field} direction={sortDirection} />
                    </button>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {visible.length ? visible.map((opportunity, index) => (
                <tr className="group border-b border-terminal-line/70 last:border-0 hover:bg-white/[0.025]" key={opportunity.id}>
                  <td className="px-4 py-4 text-center font-data text-xs text-slate-600">
                    {(visiblePage - 1) * pageSize + index + 1}
                  </td>
                  <td className="px-4 py-4 font-data font-medium">{opportunity.symbol.replace('USDT', '/USDT')}</td>
                  <td className="px-4 py-4">
                    <SourceMark source={opportunity.buySource} />
                    <p className="mt-1 font-data text-xs text-slate-400">${formatPrice(opportunity.buyPrice)}</p>
                  </td>
                  <td className="px-4 py-4">
                    <SourceMark source={opportunity.sellSource} />
                    <p className="mt-1 font-data text-xs text-slate-400">${formatPrice(opportunity.sellPrice)}</p>
                  </td>
                  <td className="px-4 py-4">
                    <span className="rounded-md border border-terminal-line bg-black/15 px-2 py-1 text-[11px] text-slate-400">
                      {routeMarket(opportunity) === 'spot' ? 'Spot ↔ Spot' : routeMarket(opportunity) === 'futures' ? 'Futures ↔ Futures' : 'Spot ↔ Futures'}
                    </span>
                  </td>
                  <td className="px-4 py-4 font-data text-base font-medium text-signal-mint">+{opportunity.profitPct.toFixed(2)}%</td>
                  <td className="px-4 py-4 text-right font-data text-xs text-slate-400">{relativeTime(opportunity.timestamp, currentTime)}</td>
                </tr>
              )) : (
                <tr>
                  <td className="px-5 py-20 text-center text-slate-500" colSpan={7}>No live routes match these filters.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <footer className="flex flex-wrap items-center justify-between gap-3 border-t border-terminal-line px-4 py-3 text-xs text-slate-500">
          <span>
            {opportunities.length ? `Showing ${(visiblePage - 1) * pageSize + 1}–${Math.min(visiblePage * pageSize, opportunities.length)} of ${opportunities.length}` : 'No routes to show'}
          </span>
          <div className="flex items-center gap-1">
            {Array.from({ length: pageCount }, (_, index) => index + 1).map((pageNumber) => (
              <button
                aria-label={`Open opportunity page ${pageNumber}`}
                aria-current={visiblePage === pageNumber ? 'page' : undefined}
                className={`grid size-8 place-items-center rounded-md border font-data ${visiblePage === pageNumber ? 'border-signal-mint/50 bg-signal-mint/10 text-signal-mint' : 'border-transparent text-slate-500 hover:border-terminal-line hover:text-terminal-text'}`}
                key={pageNumber}
                onClick={() => setPage(pageNumber)}
                type="button"
              >
                {pageNumber}
              </button>
            ))}
          </div>
        </footer>
      </section>
    </div>
  );
}
