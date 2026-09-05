import { useCallback, useEffect, useState } from 'react';

import type {
  FuturesAnalysis,
  FuturesAnalyzeInput,
  FuturesBookLevel,
  FuturesContract,
  FuturesExecutionResult,
  FuturesLiveSnapshot,
  FuturesVenueMarket,
  NeutralRoute,
  FuturesResourceStatus,
  FuturesScenario,
} from '../app/types';

interface ContractWire {
  symbol: string;
  status: string;
  quanto_multiplier: number;
  mark_price: number;
  index_price: number;
  last_price: number;
  maker_fee_rate: number;
  taker_fee_rate: number;
  order_size_min: number;
  order_size_max: number;
  price_increment: number;
  leverage_min: number;
  leverage_max: number;
  funding_rate: number;
  funding_interval: number;
}

interface ScenarioWire {
  direction: 'long' | 'short';
  status: 'ready' | 'watch' | 'no_trade';
  score: number;
  entry: number;
  stop: number;
  targets: number[];
  risk_reward: number;
  risk_amount: number;
  contracts: number;
  base_quantity: number;
  notional: number;
  estimated_fees: number;
  liquidation_note: string;
  reasons: string[];
}

interface AnalysisWire {
  contract: ContractWire;
  interval: FuturesAnalysis['interval'];
  strategy: FuturesAnalysis['strategy'];
  analyzed_at: number;
  candles: FuturesAnalysis['candles'];
  indicators: {
    ema20: number;
    ema50: number;
    atr14: number;
    rsi14: number;
    mean20: number;
    upper_band: number;
    lower_band: number;
    swing_high: number;
    swing_low: number;
    market_bias: FuturesAnalysis['indicators']['marketBias'];
  };
  liquidity: { bid_walls: FuturesBookLevel[]; ask_walls: FuturesBookLevel[] };
  long: ScenarioWire;
  short: ScenarioWire;
  neutral_route?: NeutralRouteWire;
}

interface VenueMarketWire {
  venue?: string;
  symbol?: string;
  index_price?: number;
  mark_price?: number;
  funding_rate?: number;
  next_funding_at?: number;
}

interface NeutralRouteWire {
  status: NeutralRoute['status'];
  reason_code?: string;
  reasons?: string[];
  symbol?: string;
  long_venue?: string;
  short_venue?: string;
  long_entry?: number;
  short_entry?: number;
  base_quantity?: number;
  long_contracts?: number;
  short_contracts?: number;
  long_notional?: number;
  short_notional?: number;
  gross_spread_pct?: number;
  open_fees?: number;
  estimated_close_fees?: number;
  net_convergence_spread_pct?: number;
  next_funding_carry_pct?: number;
  index_divergence_pct?: number;
}

interface LiveSnapshotWire {
  captured_at: number;
  candle: FuturesLiveSnapshot['candle'];
  gate: VenueMarketWire;
  binance: VenueMarketWire;
  route: NeutralRouteWire;
}

interface ExecutionResultWire {
  status: FuturesExecutionResult['status'];
  reason_code?: string;
  route: NeutralRouteWire;
  fills: Array<{ venue: string; order_id: string; filled_contracts: number; average_price: number }>;
  compensation?: { venue: string; order_id: string; filled_contracts: number; average_price: number };
  compensations?: Array<{ venue: string; order_id: string; filled_contracts: number; average_price: number }>;
}

function mapContract(item: ContractWire): FuturesContract {
  return {
    symbol: item.symbol, status: item.status, quantoMultiplier: item.quanto_multiplier,
    markPrice: item.mark_price, indexPrice: item.index_price, lastPrice: item.last_price,
    makerFeeRate: item.maker_fee_rate, takerFeeRate: item.taker_fee_rate,
    orderSizeMin: item.order_size_min, orderSizeMax: item.order_size_max,
    priceIncrement: item.price_increment, leverageMin: item.leverage_min,
    leverageMax: item.leverage_max, fundingRate: item.funding_rate, fundingInterval: item.funding_interval,
  };
}

function mapScenario(item: ScenarioWire): FuturesScenario {
  return {
    direction: item.direction, status: item.status, score: item.score, entry: item.entry,
    stop: item.stop, targets: item.targets, riskReward: item.risk_reward,
    riskAmount: item.risk_amount, contracts: item.contracts, baseQuantity: item.base_quantity,
    notional: item.notional, estimatedFees: item.estimated_fees,
    liquidationNote: item.liquidation_note, reasons: item.reasons,
  };
}

function mapAnalysis(item: AnalysisWire): FuturesAnalysis {
  return {
    contract: mapContract(item.contract), interval: item.interval, strategy: item.strategy,
    analyzedAt: item.analyzed_at, candles: item.candles,
    indicators: {
      ema20: item.indicators.ema20, ema50: item.indicators.ema50, atr14: item.indicators.atr14,
      rsi14: item.indicators.rsi14, mean20: item.indicators.mean20,
      upperBand: item.indicators.upper_band, lowerBand: item.indicators.lower_band,
      swingHigh: item.indicators.swing_high, swingLow: item.indicators.swing_low,
      marketBias: item.indicators.market_bias,
    },
    liquidity: { bidWalls: item.liquidity.bid_walls, askWalls: item.liquidity.ask_walls },
    long: mapScenario(item.long), short: mapScenario(item.short),
    neutralRoute: item.neutral_route ? mapNeutralRoute(item.neutral_route) : null,
  };
}

function mapVenueMarket(item: VenueMarketWire): FuturesVenueMarket {
  return {
    venue: item.venue ?? '', symbol: item.symbol ?? '', indexPrice: item.index_price ?? 0,
    markPrice: item.mark_price ?? 0, fundingRate: item.funding_rate ?? 0,
    nextFundingAt: item.next_funding_at ?? 0,
  };
}

function mapNeutralRoute(item: NeutralRouteWire): NeutralRoute {
  return {
    status: item.status, reasonCode: item.reason_code ?? '', reasons: item.reasons ?? [],
    symbol: item.symbol ?? '', longVenue: item.long_venue ?? '', shortVenue: item.short_venue ?? '',
    longEntry: item.long_entry ?? 0, shortEntry: item.short_entry ?? 0,
    baseQuantity: item.base_quantity ?? 0, longContracts: item.long_contracts ?? 0,
    shortContracts: item.short_contracts ?? 0, longNotional: item.long_notional ?? 0,
    shortNotional: item.short_notional ?? 0, grossSpreadPct: item.gross_spread_pct ?? 0,
    openFees: item.open_fees ?? 0, estimatedCloseFees: item.estimated_close_fees ?? 0,
    netConvergenceSpreadPct: item.net_convergence_spread_pct ?? 0,
    nextFundingCarryPct: item.next_funding_carry_pct ?? 0,
    indexDivergencePct: item.index_divergence_pct ?? 0,
  };
}

function mapLiveSnapshot(item: LiveSnapshotWire): FuturesLiveSnapshot {
  return {
    capturedAt: item.captured_at, candle: item.candle,
    gate: mapVenueMarket(item.gate), binance: mapVenueMarket(item.binance),
    route: mapNeutralRoute(item.route),
  };
}

function mapExecutionResult(item: ExecutionResultWire): FuturesExecutionResult {
  const mapFill = (fill: ExecutionResultWire['fills'][number]) => ({
    venue: fill.venue, orderId: fill.order_id, filledContracts: fill.filled_contracts, averagePrice: fill.average_price,
  });
  return {
    status: item.status, reasonCode: item.reason_code ?? '', route: mapNeutralRoute(item.route), fills: item.fills.map(mapFill),
    compensation: item.compensation ? mapFill(item.compensation) : null,
    compensations: (item.compensations ?? []).map(mapFill),
  };
}

async function readJSON<T>(response: Response): Promise<T> {
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    throw new Error('Futures service returned an unreadable response.');
  }
  if (!response.ok) {
    const message = typeof payload === 'object' && payload && 'error' in payload && typeof payload.error === 'string'
      ? payload.error
      : 'Futures request failed.';
    throw new Error(message);
  }
  return payload as T;
}

export function useFuturesWorkspace(fetcher: typeof fetch = fetch, liveRefreshMs = 2_000) {
  const [contracts, setContracts] = useState<FuturesContract[]>([]);
  const [contractsStatus, setContractsStatus] = useState<FuturesResourceStatus>('loading');
  const [analysis, setAnalysis] = useState<FuturesAnalysis | null>(null);
  const [analysisStatus, setAnalysisStatus] = useState<FuturesResourceStatus>('idle');
  const [error, setError] = useState<string | null>(null);
  const [liveSnapshot, setLiveSnapshot] = useState<FuturesLiveSnapshot | null>(null);
  const [liveStatus, setLiveStatus] = useState<FuturesResourceStatus>('idle');
  const [liveInput, setLiveInput] = useState<FuturesAnalyzeInput | null>(null);
  const [executionStatus, setExecutionStatus] = useState<FuturesResourceStatus>('idle');
  const [executionResult, setExecutionResult] = useState<FuturesExecutionResult | null>(null);

  const loadContracts = useCallback(async () => {
    setContractsStatus('loading');
    try {
      const payload = await readJSON<{ items: ContractWire[] }>(await fetcher('/api/futures/contracts'));
      setContracts(payload.items.map(mapContract));
      setContractsStatus('ready');
    } catch (reason) {
      setContractsStatus('error');
      setError(reason instanceof Error ? reason.message : 'Could not load Gate futures contracts.');
    }
  }, [fetcher]);

  useEffect(() => {
    void loadContracts();
  }, [loadContracts]);

  const analyze = useCallback(async (input: FuturesAnalyzeInput) => {
    setAnalysisStatus('loading');
    setLiveSnapshot(null);
    setLiveStatus('idle');
    setExecutionResult(null);
    setExecutionStatus('idle');
    setError(null);
    try {
      const response = await fetcher('/api/futures/analyze', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          contract: input.contract, interval: input.interval, strategy: input.strategy,
          account_balance: input.accountBalance, risk_percent: input.riskPercent,
          trade_notional: input.tradeNotional, min_net_spread_pct: input.minNetSpreadPct,
        }),
      });
      const payload = await readJSON<AnalysisWire>(response);
      setAnalysis(mapAnalysis(payload));
      setLiveInput(input);
      setAnalysisStatus('ready');
    } catch (reason) {
      setAnalysisStatus('error');
      setError(reason instanceof Error ? reason.message : 'Could not analyze this contract.');
    }
  }, [fetcher]);

  useEffect(() => {
    if (!liveInput) return;
    let cancelled = false;
    const query = new URLSearchParams({
      contract: liveInput.contract, interval: liveInput.interval,
      trade_notional: String(liveInput.tradeNotional), min_net_spread_pct: String(liveInput.minNetSpreadPct),
    });
    async function refresh() {
      try {
        const payload = await readJSON<LiveSnapshotWire>(await fetcher(`/api/futures/live?${query.toString()}`));
        if (!cancelled) {
          setLiveSnapshot(mapLiveSnapshot(payload));
          setLiveStatus('ready');
        }
      } catch (reason) {
        if (!cancelled) {
          setLiveStatus('error');
          setError(reason instanceof Error ? reason.message : 'Could not refresh the live futures market.');
        }
      }
    }
    setLiveStatus('loading');
    void refresh();
    const timer = window.setInterval(() => void refresh(), Math.max(liveRefreshMs, 10));
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [fetcher, liveInput, liveRefreshMs]);

  const executeRoute = useCallback(async () => {
    const route = liveSnapshot?.route ?? analysis?.neutralRoute;
    if (!analysis || !liveInput || !route || route.status !== 'executable') {
      setError('Analyze an executable Binance/Gate route before sending live orders.');
      return;
    }
    if (executionResult?.status === 'opened') {
      setError('This analyzed route has already been executed. Analyze again before opening another position.');
      return;
    }
    setExecutionStatus('loading');
    setError(null);
    try {
      const response = await fetcher('/api/futures/execute', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          contract: analysis.contract.symbol, trade_notional: liveInput.tradeNotional,
          min_net_spread_pct: liveInput.minNetSpreadPct,
          expected_long_venue: route.longVenue, expected_short_venue: route.shortVenue,
          confirm_live: true,
        }),
      });
      let payload: ExecutionResultWire | { error?: string; result?: ExecutionResultWire };
      try {
        payload = await response.json() as ExecutionResultWire | { error?: string; result?: ExecutionResultWire };
      } catch {
        throw new Error('Live futures execution returned an unreadable response.');
      }
      if (!response.ok) {
        if ('result' in payload && payload.result) {
          setExecutionResult(mapExecutionResult(payload.result));
        }
        throw new Error('error' in payload && payload.error ? payload.error : 'Live futures execution failed.');
      }
      setExecutionResult(mapExecutionResult(payload as ExecutionResultWire));
      setExecutionStatus('ready');
    } catch (reason) {
      setExecutionStatus('error');
      setError(reason instanceof Error ? reason.message : 'Live futures execution failed.');
    }
  }, [analysis, executionResult, fetcher, liveInput, liveSnapshot]);

  return {
    contracts, contractsStatus, analysis, analysisStatus, liveSnapshot, liveStatus,
    executionStatus, executionResult, error, analyze, executeRoute, retryContracts: loadContracts,
  };
}

export type FuturesWorkspace = ReturnType<typeof useFuturesWorkspace>;
