import { useCallback, useEffect, useState, type Dispatch, type SetStateAction } from 'react';

import type { ArbitrageOpportunity, OpportunitySortField, ScannerState, UiPreferences } from '../../app/types';
import type { OpportunityHistoryState } from '../../hooks/useOpportunityHistory';
import { countFreshSources, FRESHNESS_WINDOW_MS, selectBestOpportunity } from '../../lib/market-state';
import { AppShell } from '../layout/AppShell';
import { TopBar } from '../layout/TopBar';
import { SettingsDrawer } from '../settings/SettingsDrawer';
import { ExecutionChecks } from './ExecutionChecks';
import { MetricStrip } from './MetricStrip';
import { OpportunitiesTable } from './OpportunitiesTable';
import { OpportunityRoute } from './OpportunityRoute';
import { PriceComparisonChart } from './PriceComparisonChart';

interface ScannerDashboardProps {
  state: ScannerState;
  preferences: UiPreferences;
  history: OpportunityHistoryState;
  onPreferencesChange: Dispatch<SetStateAction<UiPreferences>>;
  now?: number;
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
  const opportunity = selectBestOpportunity(
    state,
    preferences.symbol,
    preferences.enabledSources,
    currentNow,
    preferences.minSpread,
  );
  const filteredLiveOpportunities = state.opportunities
    .filter(
      (item) =>
        item.symbol === preferences.symbol &&
        currentNow - item.timestamp <= FRESHNESS_WINDOW_MS &&
        item.profitPct >= preferences.minSpread &&
        preferences.enabledSources[item.buySource] !== false &&
        preferences.enabledSources[item.sellSource] !== false,
    );
  const liveRoutes = new Set(filteredLiveOpportunities.map(routeKey));
  const displayedOpportunities = [
    ...filteredLiveOpportunities,
    ...history.items.filter(
      (item) =>
        item.symbol === preferences.symbol &&
        item.profitPct >= preferences.minSpread &&
        preferences.enabledSources[item.buySource] !== false &&
        preferences.enabledSources[item.sellSource] !== false &&
        !liveRoutes.has(routeKey(item)),
    ),
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
  const totalSources = Object.keys(state.prices[preferences.symbol] ?? {}).filter(
    (source) => preferences.enabledSources[source] !== false,
  ).length;
  const freshSources = countFreshSources(state, preferences.symbol, currentNow, preferences.enabledSources);

  return (
    <AppShell
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
      <OpportunityRoute connection={state.connection} opportunity={opportunity} symbol={preferences.symbol} />
      <MetricStrip
        activeOpportunities={filteredLiveOpportunities.length}
        bestSpread={opportunity?.profitPct ?? null}
        freshSources={freshSources}
        minSpread={preferences.minSpread}
        totalSources={totalSources}
      />

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(520px,0.95fr)]">
        <OpportunitiesTable
          historyStatus={history.status}
          onRetryHistory={history.retry}
          onSort={changeSort}
          opportunities={displayedOpportunities}
          sort={preferences.sort}
        />
        <div className="space-y-4">
          <PriceComparisonChart
            enabledSources={preferences.enabledSources}
            history={state.history[preferences.symbol] ?? {}}
            onRangeChange={(chartRange) => onPreferencesChange((current) => ({ ...current, chartRange }))}
            range={preferences.chartRange}
          />
          <ExecutionChecks />
        </div>
      </div>

      <SettingsDrawer
        onClose={closeSettings}
        onPreferencesChange={onPreferencesChange}
        open={settingsOpen}
        preferences={preferences}
      />
    </AppShell>
  );
}
