export const SYMBOLS = ['BTCUSDT', 'ETHUSDT', 'XRPUSDT', 'SOLUSDT', 'COTIUSDT'] as const;
export type SymbolName = (typeof SYMBOLS)[number];
export type ChartRange = '15m' | '1h' | '4h';
export type ComparisonMode = 'spot' | 'futures' | 'mixed';
export type DashboardLayout = 'split' | 'stacked';
export type SortDirection = 'asc' | 'desc';
export type OpportunitySortField = 'symbol' | 'profit' | 'buy_source' | 'sell_source' | 'timestamp';

export interface UiPreferences {
  symbol: SymbolName;
  enabledSources: Record<string, boolean>;
  minSpread: number;
  sort: {
    field: OpportunitySortField;
    direction: SortDirection;
  };
  chartRange: ChartRange;
  comparisonMode: ComparisonMode;
  dashboardLayout: DashboardLayout;
  opportunitiesCollapsed: boolean;
  navigationCollapsed: boolean;
}

export type ConnectionStatus = 'connecting' | 'live' | 'reconnecting' | 'offline';

export interface SourcePrice {
  price: number;
  updatedAt: number;
}

export interface PricePoint {
  time: number;
  value: number;
}

export interface MarketQuote {
  symbol: SymbolName;
  source: string;
  bestBid: number;
  bestAsk: number;
  timestamp: number;
}

export interface ArbitrageOpportunity {
  id: string;
  symbol: SymbolName;
  buySource: string;
  sellSource: string;
  buyPrice: number;
  sellPrice: number;
  profitPct: number;
  peakProfitPct?: number;
  timestamp: number;
  startedAt?: number;
  endedAt?: number | null;
  historical?: boolean;
}

export interface ScannerState {
  connection: ConnectionStatus;
  prices: Record<string, Record<string, SourcePrice>>;
  quotes: Record<string, Record<string, MarketQuote>>;
  spreads: Record<string, Record<string, Record<string, number>>>;
  history: Record<string, Record<string, PricePoint[]>>;
  opportunities: ArbitrageOpportunity[];
  lastUpdatedAt: number | null;
}
