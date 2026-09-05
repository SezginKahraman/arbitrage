export const SYMBOLS = ['BTCUSDT', 'ETHUSDT', 'XRPUSDT', 'SOLUSDT', 'COTIUSDT'] as const;
export type SymbolName = string;

export function isSymbolName(value: unknown): value is SymbolName {
  return typeof value === 'string' && /^[A-Z0-9]{1,24}USDT$/.test(value) && value !== 'USDTUSDT';
}
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

export type TransferRouteStatus = 'ready' | 'blocked' | 'check' | 'unknown' | 'not_applicable';
export type TransferRouteRequestStatus = 'idle' | 'loading' | 'ready' | 'degraded';

export interface VenueAssetNetwork {
  asset: string;
  networkID: string;
  rawNetworkID: string;
  name: string;
  contractAddress: string;
  depositEnabled: boolean;
  withdrawEnabled: boolean;
  withdrawalFee: string;
  minimumWithdrawal: string;
  confirmations: number;
  checkedAt: number;
}

export interface TransferNetworkMatch {
  networkID: string;
  name: string;
  status: TransferRouteStatus;
  reason: string;
  sourceWithdrawEnabled: boolean;
  destinationDepositEnabled: boolean;
  withdrawalFee: string;
  minimumWithdrawal: string;
  contractAddress: string;
}

export interface TransferRouteEvaluation {
  asset: string;
  source: string;
  destination: string;
  status: TransferRouteStatus;
  reason: string;
  checkedAt: number;
  networks: TransferNetworkMatch[];
  sourceNetworks: VenueAssetNetwork[];
  destinationNetworks: VenueAssetNetwork[];
}

export interface MarketCandidate {
  symbol: SymbolName;
  base: string;
  spotSources: string[];
  futuresSources: string[];
  sources: string[];
}

export type MarketCatalogSourceStatus = 'loading' | 'ready' | 'stale' | 'unavailable';

export interface MarketCatalogSource {
  source: string;
  market: 'spot' | 'futures';
  status: MarketCatalogSourceStatus;
  symbols: SymbolName[];
  checkedAt: number;
  errorCode?: string;
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

export type FuturesInterval = '15m' | '1h' | '4h';
export type FuturesStrategy = 'trend_pullback' | 'breakout' | 'mean_reversion';
export type FuturesScenarioStatus = 'ready' | 'watch' | 'no_trade';
export type FuturesResourceStatus = 'idle' | 'loading' | 'ready' | 'error';

export interface FuturesAnalyzeInput {
  contract: string;
  interval: FuturesInterval;
  strategy: FuturesStrategy;
  accountBalance: number;
  riskPercent: number;
  tradeNotional: number;
  minNetSpreadPct: number;
}

export type NeutralRouteStatus = 'executable' | 'watch' | 'rejected';

export interface FuturesVenueMarket {
  venue: string;
  symbol: string;
  indexPrice: number;
  markPrice: number;
  fundingRate: number;
  nextFundingAt: number;
}

export interface NeutralRoute {
  status: NeutralRouteStatus;
  reasonCode: string;
  reasons: string[];
  symbol: string;
  longVenue: string;
  shortVenue: string;
  longEntry: number;
  shortEntry: number;
  baseQuantity: number;
  longContracts: number;
  shortContracts: number;
  longNotional: number;
  shortNotional: number;
  grossSpreadPct: number;
  openFees: number;
  estimatedCloseFees: number;
  netConvergenceSpreadPct: number;
  nextFundingCarryPct: number;
  indexDivergencePct: number;
}

export interface FuturesLiveSnapshot {
  capturedAt: number;
  candle: FuturesCandle;
  gate: FuturesVenueMarket;
  binance: FuturesVenueMarket;
  route: NeutralRoute;
}

export type FuturesExecutionStatus = 'opened' | 'failed' | 'compensated' | 'exposed';

export interface FuturesOrderFill {
  venue: string;
  orderId: string;
  filledContracts: number;
  averagePrice: number;
}

export interface FuturesExecutionResult {
  status: FuturesExecutionStatus;
  reasonCode: string;
  route: NeutralRoute;
  fills: FuturesOrderFill[];
  compensation: FuturesOrderFill | null;
  compensations: FuturesOrderFill[];
}

export interface FuturesContract {
  symbol: string;
  status: string;
  quantoMultiplier: number;
  markPrice: number;
  indexPrice: number;
  lastPrice: number;
  makerFeeRate: number;
  takerFeeRate: number;
  orderSizeMin: number;
  orderSizeMax: number;
  priceIncrement: number;
  leverageMin: number;
  leverageMax: number;
  fundingRate: number;
  fundingInterval: number;
}

export interface FuturesCandle {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

export interface FuturesBookLevel {
  price: number;
  size: number;
  notional: number;
}

export interface FuturesIndicators {
  ema20: number;
  ema50: number;
  atr14: number;
  rsi14: number;
  mean20: number;
  upperBand: number;
  lowerBand: number;
  swingHigh: number;
  swingLow: number;
  marketBias: 'bullish' | 'bearish' | 'neutral';
}

export interface FuturesScenario {
  direction: 'long' | 'short';
  status: FuturesScenarioStatus;
  score: number;
  entry: number;
  stop: number;
  targets: number[];
  riskReward: number;
  riskAmount: number;
  contracts: number;
  baseQuantity: number;
  notional: number;
  estimatedFees: number;
  liquidationNote: string;
  reasons: string[];
}

export interface FuturesAnalysis {
  contract: FuturesContract;
  interval: FuturesInterval;
  strategy: FuturesStrategy;
  analyzedAt: number;
  candles: FuturesCandle[];
  indicators: FuturesIndicators;
  liquidity: {
    bidWalls: FuturesBookLevel[];
    askWalls: FuturesBookLevel[];
  };
  long: FuturesScenario;
  short: FuturesScenario;
  neutralRoute: NeutralRoute | null;
}
