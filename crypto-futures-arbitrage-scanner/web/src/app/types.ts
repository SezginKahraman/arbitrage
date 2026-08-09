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
  feedTerminalCollapsed: boolean;
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

export interface SourceConnection {
  source: string;
  connected: boolean;
  symbols: SymbolName[];
  updatedAt: number;
}

export type FeedEventKind = 'connection' | 'quote' | 'price' | 'opportunity' | 'alert';

export interface FeedEvent {
  id: string;
  kind: FeedEventKind;
  source?: string;
  symbol?: SymbolName;
  symbols?: SymbolName[];
  timestamp: number;
  receivedAt: number;
  connected?: boolean;
  bestBid?: number;
  bestAsk?: number;
  price?: number;
  profitPct?: number;
  buySource?: string;
  sellSource?: string;
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

export type AlertMarketMode = 'all' | 'spot' | 'mixed' | 'futures';

export interface AlertRuleInput {
  name: string;
  symbol: SymbolName | '';
  marketMode: AlertMarketMode;
  buySource: string;
  sellSource: string;
  minSpreadPct: number;
  cooldownSeconds: number;
  enabled: boolean;
  browserEnabled: boolean;
}

export interface AlertRule extends AlertRuleInput {
  id: number;
  createdAtMS: number;
  updatedAtMS: number;
  lastTriggeredAtMS: number | null;
}

export interface AlertTrigger {
  id: number;
  ruleID: number;
  ruleName: string;
  symbol: SymbolName;
  buySource: string;
  sellSource: string;
  buyPrice: number;
  sellPrice: number;
  grossSpreadPct: number;
  triggeredAtMS: number;
}

export interface ScannerState {
  connection: ConnectionStatus;
  prices: Record<string, Record<string, SourcePrice>>;
  quotes: Record<string, Record<string, MarketQuote>>;
  spreads: Record<string, Record<string, Record<string, number>>>;
  history: Record<string, Record<string, PricePoint[]>>;
  opportunities: ArbitrageOpportunity[];
  alertTriggers: AlertTrigger[];
  connections: Record<string, SourceConnection>;
  feedEvents: FeedEvent[];
  lastUpdatedAt: number | null;
}
