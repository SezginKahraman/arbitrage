import {
  isSymbolName,
  type ChartRange,
  type ComparisonMode,
  type DashboardLayout,
  type OpportunitySortField,
  type SortDirection,
  type UiPreferences,
} from '../app/types';
import { DEFAULT_ENABLED_SOURCES } from './sources';

export const PREFERENCES_KEY = 'arbitrage.ui.preferences.v1';

export const DEFAULT_PREFERENCES: UiPreferences = {
  symbol: 'COTIUSDT',
  enabledSources: { ...DEFAULT_ENABLED_SOURCES },
  minSpread: 0.05,
  sort: { field: 'profit', direction: 'desc' },
  chartRange: '15m',
  comparisonMode: 'spot',
  dashboardLayout: 'split',
  opportunitiesCollapsed: false,
  feedTerminalCollapsed: false,
  navigationCollapsed: false,
};

const CHART_RANGES: ChartRange[] = ['15m', '1h', '4h'];
const COMPARISON_MODES: ComparisonMode[] = ['spot', 'futures', 'mixed'];
const DASHBOARD_LAYOUTS: DashboardLayout[] = ['split', 'stacked'];
const SORT_FIELDS: OpportunitySortField[] = ['symbol', 'profit', 'buy_source', 'sell_source', 'timestamp'];
const SORT_DIRECTIONS: SortDirection[] = ['asc', 'desc'];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseObject(value: string | null): Record<string, unknown> | null {
  if (!value) return null;

  try {
    const parsed: unknown = JSON.parse(value);
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function parseEnabledSources(value: unknown): Record<string, boolean> {
  const enabledSources = { ...DEFAULT_ENABLED_SOURCES };
  if (!isRecord(value)) return enabledSources;

  for (const source of Object.keys(enabledSources)) {
    if (typeof value[source] === 'boolean') enabledSources[source] = value[source];
  }
  return enabledSources;
}

function normalizePreferences(value: Record<string, unknown> | null): UiPreferences {
  const sort = isRecord(value?.sort) ? value.sort : null;

  return {
    symbol: isSymbolName(value?.symbol) ? value.symbol : DEFAULT_PREFERENCES.symbol,
    enabledSources: parseEnabledSources(value?.enabledSources),
    minSpread:
      typeof value?.minSpread === 'number' && Number.isFinite(value.minSpread) && value.minSpread >= 0
        ? value.minSpread
        : DEFAULT_PREFERENCES.minSpread,
    sort: {
      field: SORT_FIELDS.includes(sort?.field as OpportunitySortField)
        ? (sort?.field as OpportunitySortField)
        : DEFAULT_PREFERENCES.sort.field,
      direction: SORT_DIRECTIONS.includes(sort?.direction as SortDirection)
        ? (sort?.direction as SortDirection)
        : DEFAULT_PREFERENCES.sort.direction,
    },
    chartRange: CHART_RANGES.includes(value?.chartRange as ChartRange)
      ? (value?.chartRange as ChartRange)
      : DEFAULT_PREFERENCES.chartRange,
    comparisonMode: COMPARISON_MODES.includes(value?.comparisonMode as ComparisonMode)
      ? (value?.comparisonMode as ComparisonMode)
      : DEFAULT_PREFERENCES.comparisonMode,
    dashboardLayout: DASHBOARD_LAYOUTS.includes(value?.dashboardLayout as DashboardLayout)
      ? (value?.dashboardLayout as DashboardLayout)
      : DEFAULT_PREFERENCES.dashboardLayout,
    opportunitiesCollapsed:
      typeof value?.opportunitiesCollapsed === 'boolean'
        ? value.opportunitiesCollapsed
        : DEFAULT_PREFERENCES.opportunitiesCollapsed,
    feedTerminalCollapsed:
      typeof value?.feedTerminalCollapsed === 'boolean'
        ? value.feedTerminalCollapsed
        : DEFAULT_PREFERENCES.feedTerminalCollapsed,
    navigationCollapsed:
      typeof value?.navigationCollapsed === 'boolean'
        ? value.navigationCollapsed
        : DEFAULT_PREFERENCES.navigationCollapsed,
  };
}

export function savePreferences(storage: Storage, preferences: UiPreferences): void {
  storage.setItem(PREFERENCES_KEY, JSON.stringify(preferences));
}

export function loadPreferences(storage: Storage): UiPreferences {
  const current = parseObject(storage.getItem(PREFERENCES_KEY));
  if (current) return normalizePreferences(current);

  const legacySources = parseObject(storage.getItem('enabledSources'));
  const preferences = normalizePreferences(legacySources ? { enabledSources: legacySources } : null);

  if (legacySources) {
    savePreferences(storage, preferences);
    storage.removeItem('enabledSources');
  }

  return preferences;
}
