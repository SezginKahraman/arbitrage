import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  Ban,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  ChevronsUpDown,
  CircleHelp,
  Network,
  Plus,
  Search,
  SlidersHorizontal,
  X,
} from 'lucide-react';
import { Fragment, useEffect, useMemo, useState } from 'react';

import type {
  ArbitrageOpportunity,
  ScannerState,
  TransferRouteEvaluation,
  TransferRouteStatus,
} from '../../app/types';
import type { MarketCatalogState } from '../../hooks/useMarketCatalog';
import { transferRouteKey, type TransferRoutesState } from '../../hooks/useTransferRoutes';
import { formatPrice } from '../../lib/format';
import { FRESHNESS_WINDOW_MS } from '../../lib/market-state';
import { SOURCES, sourceMeta } from '../../lib/sources';
import { SourceMark } from '../shared/SourceMark';

type MarketFilter = 'all' | 'spot' | 'mixed' | 'futures';
type RouteFilter = 'all' | 'common' | 'ready' | 'check' | 'blocked' | 'unknown';
type SortField = 'symbol' | 'buy' | 'sell' | 'spread' | 'updated';
type SortDirection = 'asc' | 'desc';

interface AllOpportunitiesPageProps {
  state: ScannerState;
  marketCatalog?: MarketCatalogState;
  transferRoutes?: TransferRoutesState;
  enabledSources?: Record<string, boolean>;
  now?: number;
}

const pageSize = 10;

const marketFilters: Array<{ key: MarketFilter; label: string; aria: string }> = [
  { key: 'all', label: 'All', aria: 'Show all market routes' },
  { key: 'spot', label: 'Spot ↔ Spot', aria: 'Show spot to spot routes' },
  { key: 'mixed', label: 'Spot ↔ Futures', aria: 'Show spot to futures routes' },
  { key: 'futures', label: 'Futures ↔ Futures', aria: 'Show futures to futures routes' },
];

const routeFilterLabels: Record<RouteFilter, string> = {
  all: 'All route states',
  common: 'Common network',
  ready: 'Executable · READY',
  check: 'Verify alias · CHECK',
  blocked: 'Blocked',
  unknown: 'Unknown / N/A',
};

function routeMarket(opportunity: ArbitrageOpportunity): Exclude<MarketFilter, 'all'> {
  const buyMarket = sourceMeta(opportunity.buySource).market;
  const sellMarket = sourceMeta(opportunity.sellSource).market;
  if (buyMarket === 'spot' && sellMarket === 'spot') return 'spot';
  if (buyMarket === 'futures' && sellMarket === 'futures') return 'futures';
  return 'mixed';
}

function syntheticRoute(opportunity: ArbitrageOpportunity): TransferRouteEvaluation {
  const isSpot = routeMarket(opportunity) === 'spot';
  return {
    asset: opportunity.symbol.replace(/USDT$/, ''),
    source: opportunity.buySource,
    destination: opportunity.sellSource,
    status: isSpot ? 'unknown' : 'not_applicable',
    reason: isSpot ? 'network metadata is not available for this route' : 'network checks apply to spot-to-spot routes',
    checkedAt: 0,
    networks: [],
    sourceNetworks: [],
    destinationNetworks: [],
  };
}

function routeFor(opportunity: ArbitrageOpportunity, routes: Record<string, TransferRouteEvaluation>) {
  if (routeMarket(opportunity) !== 'spot') return syntheticRoute(opportunity);
  const asset = opportunity.symbol.replace(/USDT$/, '');
  return routes[transferRouteKey(asset, opportunity.buySource, opportunity.sellSource)] ?? syntheticRoute(opportunity);
}

function matchesRouteFilter(status: TransferRouteStatus, filter: RouteFilter) {
  if (filter === 'all') return true;
  if (filter === 'common') return status === 'ready' || status === 'check';
  if (filter === 'unknown') return status === 'unknown' || status === 'not_applicable';
  return status === filter;
}

function compare(left: ArbitrageOpportunity, right: ArbitrageOpportunity, field: SortField, direction: SortDirection) {
  let value: number;
  switch (field) {
    case 'symbol': value = left.symbol.localeCompare(right.symbol); break;
    case 'buy': value = left.buySource.localeCompare(right.buySource); break;
    case 'sell': value = left.sellSource.localeCompare(right.sellSource); break;
    case 'updated': value = left.timestamp - right.timestamp; break;
    case 'spread': value = left.profitPct - right.profitPct; break;
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
  return direction === 'asc'
    ? <ArrowUp aria-hidden="true" className="text-signal-mint" size={13} />
    : <ArrowDown aria-hidden="true" className="text-signal-mint" size={13} />;
}

const routeStyles: Record<TransferRouteStatus, string> = {
  ready: 'border-signal-mint/35 bg-signal-mint/10 text-signal-mint',
  check: 'border-signal-amber/35 bg-signal-amber/10 text-signal-amber',
  blocked: 'border-red-400/30 bg-red-400/10 text-red-300',
  unknown: 'border-terminal-line bg-white/[0.025] text-slate-400',
  not_applicable: 'border-terminal-line bg-white/[0.025] text-slate-500',
};

function RouteIcon({ status }: { status: TransferRouteStatus }) {
  if (status === 'ready') return <CheckCircle2 aria-hidden="true" size={13} />;
  if (status === 'check') return <AlertTriangle aria-hidden="true" size={13} />;
  if (status === 'blocked') return <Ban aria-hidden="true" size={13} />;
  return <CircleHelp aria-hidden="true" size={13} />;
}

function routeLabel(status: TransferRouteStatus) {
  if (status === 'not_applicable') return 'N/A';
  return status.toUpperCase();
}

function RouteDetails({ route }: { route: TransferRouteEvaluation }) {
  return (
    <div className="grid gap-3 p-4 md:grid-cols-[minmax(220px,0.6fr)_1fr]">
      <div>
        <p className="font-data text-[10px] uppercase tracking-[0.18em] text-slate-500">Directional transfer check</p>
        <p className="mt-2 text-sm text-slate-300">{route.reason}</p>
        <p className="mt-2 font-data text-xs text-slate-500">
          {sourceMeta(route.source).label} → {sourceMeta(route.destination).label}
        </p>
      </div>
      <div className="grid gap-2 sm:grid-cols-2">
        {route.networks.length ? route.networks.map((network) => (
          <div className="rounded-lg border border-terminal-line bg-black/15 p-3" key={`${network.networkID}:${network.status}`}>
            <div className="flex items-center justify-between gap-2">
              <span className="font-data text-sm text-terminal-text">{network.name || network.networkID}</span>
              <span className={`rounded border px-1.5 py-0.5 font-data text-[10px] ${routeStyles[network.status]}`}>{routeLabel(network.status)}</span>
            </div>
            <p className="mt-2 text-xs text-slate-500">{network.reason}</p>
            <p className="mt-2 font-data text-[11px] text-slate-400">
              Withdraw {network.sourceWithdrawEnabled ? 'open' : 'closed'} · Deposit {network.destinationDepositEnabled ? 'open' : 'closed'}
              {network.withdrawalFee ? ` · Fee ${network.withdrawalFee}` : ''}
              {network.minimumWithdrawal ? ` · Min ${network.minimumWithdrawal}` : ''}
            </p>
          </div>
        )) : (
          <div className="rounded-lg border border-dashed border-terminal-line p-3 text-xs text-slate-500 sm:col-span-2">
            No matching network legs were reported for this direction.
          </div>
        )}
      </div>
    </div>
  );
}

export function AllOpportunitiesPage({ state, marketCatalog, transferRoutes, enabledSources = {}, now }: AllOpportunitiesPageProps) {
  const [clock, setClock] = useState(() => now ?? Date.now());
  const [query, setQuery] = useState('');
  const [market, setMarket] = useState<MarketFilter>('all');
  const [exchange, setExchange] = useState('all');
  const [routeFilter, setRouteFilter] = useState<RouteFilter>('all');
  const [minimumSpread, setMinimumSpread] = useState('0.05');
  const [sortField, setSortField] = useState<SortField>('spread');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');
  const [page, setPage] = useState(1);
  const [addingMarket, setAddingMarket] = useState(false);
  const [marketQuery, setMarketQuery] = useState('');
  const [expandedRoute, setExpandedRoute] = useState<string | null>(null);

  useEffect(() => {
    if (now !== undefined) return undefined;
    const timer = window.setInterval(() => setClock(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, [now]);

  const currentTime = now ?? clock;
  const routeMap = transferRoutes?.routes ?? {};
  const minimum = Number(minimumSpread);
  const normalizedQuery = query.trim().toLowerCase();
  const freshRoutes = useMemo(
    () => state.opportunities.filter((item) => {
      const age = currentTime - item.timestamp;
      return age >= 0 && age <= FRESHNESS_WINDOW_MS;
    }),
    [currentTime, state.opportunities],
  );
  const enabledRoutes = useMemo(
    () => freshRoutes.filter((item) => enabledSources[item.buySource] !== false && enabledSources[item.sellSource] !== false),
    [enabledSources, freshRoutes],
  );
  const opportunities = useMemo(
    () => enabledRoutes
      .filter((item) => {
        if (Number.isFinite(minimum) && item.profitPct < minimum) return false;
        if (market !== 'all' && routeMarket(item) !== market) return false;
        if (exchange !== 'all' && item.buySource !== exchange && item.sellSource !== exchange) return false;
        if (!matchesRouteFilter(routeFor(item, routeMap).status, routeFilter)) return false;
        if (!normalizedQuery) return true;
        return [item.symbol, item.symbol.replace('USDT', '/USDT'), sourceMeta(item.buySource).label, sourceMeta(item.sellSource).label]
          .join(' ').toLowerCase().includes(normalizedQuery);
      })
      .sort((left, right) => compare(left, right, sortField, sortDirection)),
    [enabledRoutes, exchange, market, minimum, normalizedQuery, routeFilter, routeMap, sortDirection, sortField],
  );

  useEffect(() => setPage(1), [exchange, market, minimumSpread, normalizedQuery, routeFilter]);
  useEffect(() => {
    if (exchange !== 'all' && enabledSources[exchange] === false) setExchange('all');
  }, [enabledSources, exchange]);

  const statusCounts = useMemo(() => {
    const counts = { ready: 0, check: 0, blocked: 0, unknown: 0 };
    for (const opportunity of enabledRoutes) {
      const status = routeFor(opportunity, routeMap).status;
      if (status === 'ready') counts.ready++;
      else if (status === 'check') counts.check++;
      else if (status === 'blocked') counts.blocked++;
      else counts.unknown++;
    }
    return counts;
  }, [enabledRoutes, routeMap]);

  const pageCount = Math.max(1, Math.ceil(opportunities.length / pageSize));
  const visiblePage = Math.min(page, pageCount);
  const visible = opportunities.slice((visiblePage - 1) * pageSize, visiblePage * pageSize);
  const activeMarkets = new Set(marketCatalog?.watchlist ?? []);
  const availableMarkets = (marketCatalog?.candidates ?? [])
    .filter((candidate) => !activeMarkets.has(candidate.symbol))
    .filter((candidate) => !marketQuery.trim() || candidate.symbol.toLowerCase().includes(marketQuery.trim().toLowerCase()))
    .slice(0, 12);

  const updateWatchlist = async (symbols: string[]) => {
    if (!marketCatalog) return;
    try {
      await marketCatalog.replace(symbols);
    } catch {
      // The hook retains the previous committed selection and exposes the sanitized error.
    }
  };

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

      {marketCatalog ? (
        <section aria-label="Market watchlist" className="rounded-xl border border-terminal-line bg-terminal-panel/65 p-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="flex items-center gap-2">
                <Network aria-hidden="true" className="text-signal-mint" size={17} />
                <h2 className="font-medium">Tracked pairs</h2>
                <span className="rounded-full border border-terminal-line px-2 py-0.5 font-data text-[10px] text-slate-400">
                  {marketCatalog.watchlist.length}/{marketCatalog.limit}
                </span>
              </div>
              <p className="mt-1 text-xs text-slate-500">Add or remove pairs here. Each pair automatically subscribes to every supported Spot and Futures feed.</p>
            </div>
            <button
              aria-expanded={addingMarket}
              className="inline-flex h-9 items-center gap-2 rounded-lg border border-signal-mint/60 bg-signal-mint px-3 text-xs font-medium text-terminal-ink transition hover:bg-emerald-300 disabled:opacity-40"
              disabled={marketCatalog.saving || marketCatalog.watchlist.length >= marketCatalog.limit}
              onClick={() => setAddingMarket((value) => !value)}
              type="button"
            >
              {addingMarket ? <X aria-hidden="true" size={14} /> : <Plus aria-hidden="true" size={14} />}
              {addingMarket ? 'Close' : 'Add pair'}
            </button>
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            {marketCatalog.watchlist.map((symbol) => {
              const coverage = marketCatalog.candidates.find((candidate) => candidate.symbol === symbol);
              return (
                <span className="inline-flex items-center gap-2 rounded-lg border border-terminal-line bg-terminal-ink/60 py-1.5 pl-2.5 pr-1.5" key={symbol}>
                  <span className="font-data text-xs text-terminal-text">{symbol.replace('USDT', '/USDT')}</span>
                  <span className="font-data text-[9px] text-slate-500">{coverage?.sources.length ?? 0} feeds</span>
                  <button
                    aria-label={`Remove ${symbol.replace('USDT', '/USDT')}`}
                    className="grid size-6 place-items-center rounded text-slate-500 hover:bg-red-400/10 hover:text-red-300 disabled:opacity-30"
                    disabled={marketCatalog.saving || marketCatalog.watchlist.length <= 1}
                    onClick={() => void updateWatchlist(marketCatalog.watchlist.filter((item) => item !== symbol))}
                    type="button"
                  >
                    <X aria-hidden="true" size={12} />
                  </button>
                </span>
              );
            })}
          </div>
          {marketCatalog.error ? <p className="mt-3 text-xs text-red-300" role="alert">{marketCatalog.error}</p> : null}
          {addingMarket ? (
            <div className="mt-4 rounded-xl border border-terminal-line bg-black/15 p-3">
              <div className="mb-3 flex flex-wrap items-end justify-between gap-2">
                <div>
                  <h3 className="font-medium text-terminal-text">Add a USDT pair</h3>
                  <p className="mt-1 text-xs text-slate-500">Search below, then press <span className="text-signal-mint">+</span>. Only pairs available from at least two scanner feeds are listed.</p>
                </div>
                <span className="font-data text-[10px] text-slate-500">{availableMarkets.length} available</span>
              </div>
              <label className="relative block">
                <span className="sr-only">Search market catalog</span>
                <Search aria-hidden="true" className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" size={15} />
                <input
                  aria-label="Search market catalog"
                  autoFocus
                  className="h-10 w-full rounded-lg border border-terminal-line bg-terminal-ink pl-9 pr-3 text-sm"
                  onChange={(event) => setMarketQuery(event.target.value)}
                  placeholder="Search all common USDT markets…"
                  value={marketQuery}
                />
              </label>
              <div className="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
                {availableMarkets.map((candidate) => (
                  <div className="flex items-center justify-between gap-3 rounded-lg border border-terminal-line bg-terminal-panel/55 p-3" key={candidate.symbol}>
                    <div className="min-w-0">
                      <p className="font-data text-sm">{candidate.symbol.replace('USDT', '/USDT')}</p>
                      <p className="mt-1 truncate text-[10px] text-slate-500">
                        {candidate.spotSources.length} Spot · {candidate.futuresSources.length} Futures · {candidate.sources.length} total
                      </p>
                      <div className="mt-2 flex max-w-[260px] flex-wrap gap-1">
                        {candidate.sources.map((source) => (
                          <span className="inline-flex items-center gap-1 rounded border border-terminal-line px-1.5 py-0.5 text-[9px] text-slate-400" key={source}>
                            <span className="size-1.5 rounded-full" style={{ backgroundColor: sourceMeta(source).color }} />
                            {sourceMeta(source).label}
                          </span>
                        ))}
                      </div>
                    </div>
                    <button
                      aria-label={`Add ${candidate.symbol.replace('USDT', '/USDT')}`}
                      className="grid size-8 shrink-0 place-items-center rounded-lg border border-signal-mint/30 text-signal-mint hover:bg-signal-mint/10"
                      disabled={marketCatalog.saving}
                      onClick={() => void updateWatchlist([...marketCatalog.watchlist, candidate.symbol])}
                      type="button"
                    >
                      <Plus aria-hidden="true" size={14} />
                    </button>
                  </div>
                ))}
                {!availableMarkets.length ? <p className="px-1 py-3 text-xs text-slate-500">No addable common market matches this search.</p> : null}
              </div>
            </div>
          ) : null}
        </section>
      ) : null}

      <section aria-label="Route status summary" className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {([
          ['ready', 'Executable', statusCounts.ready, 'text-signal-mint'],
          ['check', 'Common · verify', statusCounts.check, 'text-signal-amber'],
          ['blocked', 'Blocked', statusCounts.blocked, 'text-red-300'],
          ['unknown', 'Unknown / N/A', statusCounts.unknown, 'text-slate-400'],
        ] as const).map(([key, label, count, color]) => (
          <button
            className={`rounded-xl border border-terminal-line bg-terminal-panel/55 p-3 text-left transition hover:border-slate-600 ${routeFilter === key ? 'ring-1 ring-signal-mint/40' : ''}`}
            key={key}
            onClick={() => setRouteFilter(key)}
            type="button"
          >
            <span className="text-xs text-slate-500">{label}</span>
            <span className={`mt-1 block font-data text-2xl ${color}`}>{count}</span>
          </button>
        ))}
      </section>

      <section aria-label="Opportunity filters" className="rounded-xl border border-terminal-line bg-terminal-panel/65 p-3">
        <div className="grid gap-3 xl:grid-cols-[minmax(220px,1fr)_auto_minmax(160px,0.42fr)_minmax(170px,0.42fr)_150px]">
          <label className="relative">
            <span className="sr-only">Search opportunities</span>
            <Search aria-hidden="true" className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" size={17} />
            <input aria-label="Search opportunities" className="h-11 w-full rounded-lg border border-terminal-line bg-terminal-ink/70 pl-10 pr-3 text-sm placeholder:text-slate-600" onChange={(event) => setQuery(event.target.value)} placeholder="Search pair or exchange" type="search" value={query} />
          </label>
          <div aria-label="Opportunity market type" className="flex overflow-x-auto rounded-lg border border-terminal-line bg-terminal-ink/70 p-1" role="group">
            {marketFilters.map((filter) => (
              <button aria-label={filter.aria} aria-pressed={market === filter.key} className={`shrink-0 rounded-md px-3 py-2 text-xs transition ${market === filter.key ? 'bg-signal-mint/12 text-signal-mint shadow-[inset_0_0_0_1px_rgba(39,229,140,0.2)]' : 'text-slate-500 hover:text-terminal-text'}`} key={filter.key} onClick={() => setMarket(filter.key)} type="button">{filter.label}</button>
            ))}
          </div>
          <label>
            <span className="sr-only">Filter by exchange</span>
            <select aria-label="Filter by exchange" className="h-11 w-full rounded-lg border border-terminal-line bg-terminal-ink/70 px-3 text-sm" onChange={(event) => setExchange(event.target.value)} value={exchange}>
              <option value="all">All exchanges</option>
              {SOURCES.filter((source) => source.market !== 'oracle' && enabledSources[source.key] !== false).map((source) => <option key={source.key} value={source.key}>{source.label}</option>)}
            </select>
          </label>
          <label>
            <span className="sr-only">Filter by transfer route</span>
            <select aria-label="Filter by transfer route" className="h-11 w-full rounded-lg border border-terminal-line bg-terminal-ink/70 px-3 text-sm" onChange={(event) => setRouteFilter(event.target.value as RouteFilter)} value={routeFilter}>
              {(Object.entries(routeFilterLabels) as Array<[RouteFilter, string]>).map(([key, label]) => <option key={key} value={key}>{label}</option>)}
            </select>
          </label>
          <label className="relative">
            <span className="absolute left-3 top-1.5 flex items-center gap-1 text-[10px] uppercase tracking-wide text-slate-500"><SlidersHorizontal aria-hidden="true" size={11} /> Min gross</span>
            <input aria-label="Minimum gross spread" className="h-11 w-full rounded-lg border border-terminal-line bg-terminal-ink/70 px-3 pt-3 font-data text-sm" min="0" onChange={(event) => setMinimumSpread(event.target.value)} step="0.05" type="number" value={minimumSpread} />
            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 font-data text-xs text-slate-500">%</span>
          </label>
        </div>
      </section>

      <section className="overflow-hidden rounded-xl border border-terminal-line bg-terminal-panel/65">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[1180px] text-left text-sm">
            <thead className="bg-black/10 text-[11px] uppercase tracking-wide text-slate-500">
              <tr className="border-b border-terminal-line">
                <th className="w-12 px-4 py-3 text-center font-medium">#</th>
                {columns.slice(0, 3).map((column) => (
                  <th aria-sort={sortField === column.field ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'} className={`px-4 py-3 font-medium ${column.className ?? ''}`} key={column.field}>
                    <button aria-label={`Sort by ${column.aria}`} className="inline-flex items-center gap-1.5 hover:text-terminal-text" onClick={() => changeSort(column.field)} type="button">{column.label}<SortIcon active={sortField === column.field} direction={sortDirection} /></button>
                  </th>
                ))}
                <th className="px-4 py-3 font-medium">Market</th>
                {columns.slice(3, 4).map((column) => (
                  <th aria-sort={sortField === column.field ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'} className="px-4 py-3 font-medium" key={column.field}>
                    <button aria-label={`Sort by ${column.aria}`} className="inline-flex items-center gap-1.5 hover:text-terminal-text" onClick={() => changeSort(column.field)} type="button">{column.label}<SortIcon active={sortField === column.field} direction={sortDirection} /></button>
                  </th>
                ))}
                <th className="px-4 py-3 font-medium">Transfer route</th>
                {columns.slice(4).map((column) => (
                  <th aria-sort={sortField === column.field ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'} className="px-4 py-3 text-right font-medium" key={column.field}>
                    <button aria-label={`Sort by ${column.aria}`} className="ml-auto inline-flex items-center gap-1.5 hover:text-terminal-text" onClick={() => changeSort(column.field)} type="button">{column.label}<SortIcon active={sortField === column.field} direction={sortDirection} /></button>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {visible.length ? visible.map((opportunity, index) => {
                const route = routeFor(opportunity, routeMap);
                const expanded = expandedRoute === opportunity.id;
                return (
                  <Fragment key={opportunity.id}>
                    <tr className="group border-b border-terminal-line/70 hover:bg-white/[0.025]">
                      <td className="px-4 py-4 text-center font-data text-xs text-slate-600">{(visiblePage - 1) * pageSize + index + 1}</td>
                      <td className="px-4 py-4 font-data font-medium">{opportunity.symbol.replace('USDT', '/USDT')}</td>
                      <td className="px-4 py-4"><SourceMark source={opportunity.buySource} /><p className="mt-1 font-data text-xs text-slate-400">${formatPrice(opportunity.buyPrice)}</p></td>
                      <td className="px-4 py-4"><SourceMark source={opportunity.sellSource} /><p className="mt-1 font-data text-xs text-slate-400">${formatPrice(opportunity.sellPrice)}</p></td>
                      <td className="px-4 py-4"><span className="rounded-md border border-terminal-line bg-black/15 px-2 py-1 text-[11px] text-slate-400">{routeMarket(opportunity) === 'spot' ? 'Spot ↔ Spot' : routeMarket(opportunity) === 'futures' ? 'Futures ↔ Futures' : 'Spot ↔ Futures'}</span></td>
                      <td className="px-4 py-4 font-data text-base font-medium text-signal-mint">+{opportunity.profitPct.toFixed(2)}%</td>
                      <td className="px-4 py-4">
                        <button aria-expanded={expanded} aria-label={`Show transfer route for ${opportunity.symbol.replace('USDT', '/USDT')}`} className={`inline-flex items-center gap-1.5 rounded-md border px-2 py-1 font-data text-[11px] ${routeStyles[route.status]}`} onClick={() => setExpandedRoute(expanded ? null : opportunity.id)} type="button">
                          <RouteIcon status={route.status} />{routeLabel(route.status)}{expanded ? <ChevronDown aria-hidden="true" size={12} /> : <ChevronRight aria-hidden="true" size={12} />}
                        </button>
                      </td>
                      <td className="px-4 py-4 text-right font-data text-xs text-slate-400">{relativeTime(opportunity.timestamp, currentTime)}</td>
                    </tr>
                    {expanded ? <tr className="border-b border-terminal-line/70 bg-black/10"><td colSpan={8}><RouteDetails route={route} /></td></tr> : null}
                  </Fragment>
                );
              }) : (
                <tr><td className="px-5 py-20 text-center text-slate-500" colSpan={8}>No live routes match these filters.</td></tr>
              )}
            </tbody>
          </table>
        </div>
        <footer className="flex flex-wrap items-center justify-between gap-3 border-t border-terminal-line px-4 py-3 text-xs text-slate-500">
          <span>{opportunities.length ? `Showing ${(visiblePage - 1) * pageSize + 1}–${Math.min(visiblePage * pageSize, opportunities.length)} of ${opportunities.length}` : 'No routes to show'}</span>
          <div className="flex items-center gap-1">
            {Array.from({ length: pageCount }, (_, index) => index + 1).map((pageNumber) => (
              <button aria-label={`Open opportunity page ${pageNumber}`} aria-current={visiblePage === pageNumber ? 'page' : undefined} className={`grid size-8 place-items-center rounded-md border font-data ${visiblePage === pageNumber ? 'border-signal-mint/50 bg-signal-mint/10 text-signal-mint' : 'border-transparent text-slate-500 hover:border-terminal-line hover:text-terminal-text'}`} key={pageNumber} onClick={() => setPage(pageNumber)} type="button">{pageNumber}</button>
            ))}
          </div>
        </footer>
      </section>
    </div>
  );
}
