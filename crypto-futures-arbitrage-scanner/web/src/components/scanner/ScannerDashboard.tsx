import { useCallback, useEffect, useMemo, useState, type Dispatch, type SetStateAction } from 'react';

import type { ArbitrageOpportunity, OpportunitySortField, ScannerState, UiPreferences } from '../../app/types';
import type { AppPage } from '../../app/navigation';
import type { OpportunityHistoryState } from '../../hooks/useOpportunityHistory';
import { countFreshSources, countSourceConnections, FRESHNESS_WINDOW_MS } from '../../lib/market-state';
import { routeMatchesComparisonMode, sourceMatchesComparisonMode } from '../../lib/sources';
import { AppShell } from '../layout/AppShell';
import { TopBar } from '../layout/TopBar';
import { SettingsDrawer } from '../settings/SettingsDrawer';
import { ExecutionChecks } from './ExecutionChecks';
import { MetricStrip } from './MetricStrip';
import { MarketControls } from './MarketControls';
import { LiveFeedTerminal } from './LiveFeedTerminal';
import { OpportunitiesTable } from './OpportunitiesTable';
import { OpportunityRoute } from './OpportunityRoute';
import { PriceComparisonChart } from './PriceComparisonChart';

interface ScannerDashboardProps {
  state: ScannerState;
  preferences: UiPreferences;
  history: OpportunityHistoryState;
  onPreferencesChange: Dispatch<SetStateAction<UiPreferences>>;
  now?: number;
  onNavigate?: (page: AppPage) => void;
}

function routeKey(opportunity: ScannerState['opportunities'][number]): string {
  return `${opportunity.symbol}:${opportunity.buySource}:${opportunity.sellSource}`;
}

function compareOpportunities(
  left: ArbitrageOpportunity,
  right: ArbitrageOpportunity,
  sort: UiPreferences['sort'],
): number {
  let comparison: number;
  switch (sort.field) {
    case 'symbol':
      comparison = left.symbol.localeCompare(right.symbol);
      break;
    case 'buy_source':
      comparison = left.buySource.localeCompare(right.buySource);
      break;
    case 'sell_source':
      comparison = left.sellSource.localeCompare(right.sellSource);
      break;
    case 'timestamp':
      comparison = left.timestamp - right.timestamp;
      break;
    case 'profit':
      comparison = left.profitPct - right.profitPct;
      break;
  }
  const directed = sort.direction === 'asc' ? comparison : -comparison;
  return directed || left.id.localeCompare(right.id);
}

export function ScannerDashboard({
  state,
  preferences,
  history,
  onPreferencesChange,
  now,
  onNavigate,
}: ScannerDashboardProps) {
  const [settingsOpen, setSettingsOpen] = useState(false);
  const openSettings = useCallback(() => setSettingsOpen(true), []);
  const closeSettings = useCallback(() => setSettingsOpen(false), []);
  const [liveNow, setLiveNow] = useState(() => now ?? Date.now());
  useEffect(() => {
    if (now !== undefined) return;
    const timer = window.setInterval(() => setLiveNow(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, [now]);
  const currentNow = now ?? liveNow;
  const visibleSources = useMemo(
    () => Object.fromEntries(
      Object.keys(preferences.enabledSources).map((source) => [
        source,
        preferences.enabledSources[source] !== false && sourceMatchesComparisonMode(source, preferences.comparisonMode),
      ]),
    ),
    [preferences.comparisonMode, preferences.enabledSources],
  );
  const sourceIsVisible = (source: string) =>
    visibleSources[source] ??
    (preferences.enabledSources[source] !== false && sourceMatchesComparisonMode(source, preferences.comparisonMode));
  const filteredLiveOpportunities = state.opportunities
    .filter(
      (item) =>
        item.symbol === preferences.symbol &&
        currentNow - item.timestamp <= FRESHNESS_WINDOW_MS &&
        item.profitPct >= preferences.minSpread &&
        sourceIsVisible(item.buySource) &&
        sourceIsVisible(item.sellSource) &&
        routeMatchesComparisonMode(item.buySource, item.sellSource, preferences.comparisonMode),
    );
  const opportunity = [...filteredLiveOpportunities].sort((left, right) => right.profitPct - left.profitPct)[0] ?? null;
  const liveRoutes = new Set(filteredLiveOpportunities.map(routeKey));
  const latestHistoryByRoute = new Map<string, ArbitrageOpportunity>();
  for (const item of history.items) {
    if (
      item.symbol !== preferences.symbol ||
      item.profitPct < preferences.minSpread ||
      !sourceIsVisible(item.buySource) ||
      !sourceIsVisible(item.sellSource) ||
      !routeMatchesComparisonMode(item.buySource, item.sellSource, preferences.comparisonMode)
    ) {
      continue;
    }
    const key = routeKey(item);
    const current = latestHistoryByRoute.get(key);
    if (!current || item.timestamp > current.timestamp) latestHistoryByRoute.set(key, item);
  }
  const displayedOpportunities = [
    ...filteredLiveOpportunities,
    ...[...latestHistoryByRoute.values()].filter((item) => !liveRoutes.has(routeKey(item))),
  ].sort((left, right) => compareOpportunities(left, right, preferences.sort));
  const changeSort = (field: OpportunitySortField) => {
    onPreferencesChange((current) => ({
      ...current,
      sort: {
        field,
        direction:
          current.sort.field === field
            ? current.sort.direction === 'asc' ? 'desc' : 'asc'
            : field === 'profit' || field === 'timestamp' ? 'desc' : 'asc',
      },
    }));
  };
  const feedConnections = countSourceConnections(state, preferences.symbol, visibleSources);
  const freshBooks = countFreshSources(state, preferences.symbol, currentNow, visibleSources);

  return (
    <AppShell
      activePage="scanner"
      onNavigate={onNavigate}
      onOpenSettings={openSettings}
      topBar={
        <TopBar
          connection={state.connection}
          lastUpdatedAt={state.lastUpdatedAt}
          onOpenSettings={openSettings}
          onPreferencesChange={onPreferencesChange}
          preferences={preferences}
        />
      }
    >
      <MarketControls
        comparisonMode={preferences.comparisonMode}
        dashboardLayout={preferences.dashboardLayout}
        enabledSources={preferences.enabledSources}
        onComparisonModeChange={(comparisonMode) => onPreferencesChange((current) => ({ ...current, comparisonMode }))}
        onDashboardLayoutChange={(dashboardLayout) => onPreferencesChange((current) => ({ ...current, dashboardLayout }))}
        onSourceToggle={(source) => onPreferencesChange((current) => ({
          ...current,
          enabledSources: { ...current.enabledSources, [source]: current.enabledSources[source] === false },
        }))}
      />
      <OpportunityRoute connection={state.connection} opportunity={opportunity} symbol={preferences.symbol} />
      <MetricStrip
        activeOpportunities={filteredLiveOpportunities.length}
        bestSpread={opportunity?.profitPct ?? null}
        connectedFeeds={feedConnections.connected}
        freshBooks={freshBooks}
        minSpread={preferences.minSpread}
        totalBooks={feedConnections.total}
        totalFeeds={feedConnections.total}
      />

      <div className={`grid gap-4 ${preferences.dashboardLayout === 'split' ? 'xl:grid-cols-[minmax(0,1.05fr)_minmax(520px,0.95fr)]' : 'grid-cols-1'}`}>
        <OpportunitiesTable
          collapsed={preferences.opportunitiesCollapsed}
          historyStatus={history.status}
          onRetryHistory={history.retry}
          onSort={changeSort}
          onToggleCollapsed={() => onPreferencesChange((current) => ({
            ...current,
            opportunitiesCollapsed: !current.opportunitiesCollapsed,
          }))}
          liveCount={filteredLiveOpportunities.length}
          opportunities={displayedOpportunities}
          sort={preferences.sort}
        />
        <div className="space-y-4">
          <PriceComparisonChart
            enabledSources={visibleSources}
            history={state.history[preferences.symbol] ?? {}}
            onRangeChange={(chartRange) => onPreferencesChange((current) => ({ ...current, chartRange }))}
            range={preferences.chartRange}
          />
          <ExecutionChecks />
        </div>
      </div>

      <LiveFeedTerminal
        collapsed={preferences.feedTerminalCollapsed}
        events={state.feedEvents ?? []}
        onCollapsedChange={(feedTerminalCollapsed) => onPreferencesChange((current) => ({
          ...current,
          feedTerminalCollapsed,
        }))}
        symbol={preferences.symbol}
      />

      <SettingsDrawer
        onClose={closeSettings}
        onPreferencesChange={onPreferencesChange}
        open={settingsOpen}
        preferences={preferences}
      />
    </AppShell>
  );
}
