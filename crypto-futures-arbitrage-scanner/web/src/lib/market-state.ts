import {
  SYMBOLS,
  type ArbitrageOpportunity,
  type AlertTrigger,
  type FeedEvent,
  type MarketQuote,
  type PricePoint,
  type ScannerState,
  type SymbolName,
} from '../app/types';

const OPPORTUNITY_LIMIT = 250;
const ALERT_TRIGGER_LIMIT = 100;
const FEED_EVENT_LIMIT = 120;
const HISTORY_WINDOW_MS = 4 * 60 * 60 * 1_000;
const HISTORY_BUCKET_MS = 5_000;
export const FRESHNESS_WINDOW_MS = 15_000;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isFinitePositive(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0;
}

function isSymbol(value: unknown): value is SymbolName {
  return typeof value === 'string' && SYMBOLS.includes(value as SymbolName);
}

export function createInitialScannerState(): ScannerState {
  return {
    connection: 'connecting',
    prices: {},
    quotes: {},
    spreads: {},
    history: {},
    opportunities: [],
    alertTriggers: [],
    connections: {},
    feedEvents: [],
    lastUpdatedAt: null,
  };
}

function parseSourceStatus(value: unknown): {
  source: string;
  connected: boolean;
  symbols: SymbolName[];
  timestamp: number;
} | null {
  if (!isRecord(value) || typeof value.source !== 'string' || !value.source) return null;
  if (typeof value.connected !== 'boolean' || !Array.isArray(value.symbols)) return null;
  if (typeof value.timestamp !== 'number' || !Number.isFinite(value.timestamp)) return null;
  if (!value.symbols.every(isSymbol)) return null;
  return {
    source: value.source,
    connected: value.connected,
    symbols: value.symbols as SymbolName[],
    timestamp: value.timestamp,
  };
}

function appendFeedEvent(state: ScannerState, event: FeedEvent): FeedEvent[] {
  return [event, ...state.feedEvents].slice(0, FEED_EVENT_LIMIT);
}

function parseQuote(value: unknown): MarketQuote | null {
  if (!isRecord(value) || !isSymbol(value.symbol) || typeof value.source !== 'string' || !value.source) return null;
  if (!isFinitePositive(value.best_bid) || !isFinitePositive(value.best_ask) || value.best_bid > value.best_ask) return null;
  if (typeof value.timestamp !== 'number' || !Number.isFinite(value.timestamp)) return null;

  return {
    symbol: value.symbol,
    source: value.source,
    bestBid: value.best_bid,
    bestAsk: value.best_ask,
    timestamp: value.timestamp,
  };
}

function parsePriceUpdate(value: unknown): { symbol: SymbolName; source: string; price: number; timestamp: number } | null {
  if (!isRecord(value) || !isSymbol(value.symbol) || typeof value.source !== 'string' || !value.source) return null;
  if (!isFinitePositive(value.price) || typeof value.timestamp !== 'number' || !Number.isFinite(value.timestamp)) return null;
  return { symbol: value.symbol, source: value.source, price: value.price, timestamp: value.timestamp };
}

function parsePrices(value: unknown): Record<string, Record<string, number>> | null {
  if (!isRecord(value)) return null;

  const prices: Record<string, Record<string, number>> = {};
  for (const [symbol, rawSources] of Object.entries(value)) {
    if (!isSymbol(symbol) || !isRecord(rawSources)) return null;

    const sources: Record<string, number> = {};
    for (const [source, price] of Object.entries(rawSources)) {
      if (!source || !isFinitePositive(price)) return null;
      sources[source] = price;
    }
    prices[symbol] = sources;
  }
  return prices;
}

function parseSpreads(value: unknown): Record<string, Record<string, number>> | null {
  if (!isRecord(value)) return null;

  const spreads: Record<string, Record<string, number>> = {};
  for (const [buySource, rawSellSources] of Object.entries(value)) {
    if (!buySource || !isRecord(rawSellSources)) return null;

    const sellSources: Record<string, number> = {};
    for (const [sellSource, spread] of Object.entries(rawSellSources)) {
      if (!sellSource || typeof spread !== 'number' || !Number.isFinite(spread)) return null;
      sellSources[sellSource] = spread;
    }
    spreads[buySource] = sellSources;
  }
  return spreads;
}

function parseOpportunity(value: unknown): ArbitrageOpportunity | null {
  if (!isRecord(value) || !isSymbol(value.symbol)) return null;

  const buySource = value.buy_source;
  const sellSource = value.sell_source;
  const timestamp = value.timestamp;
  const profitPct = value.profit_pct;

  if (
    typeof buySource !== 'string' ||
    typeof sellSource !== 'string' ||
    !isFinitePositive(value.buy_price) ||
    !isFinitePositive(value.sell_price) ||
    typeof profitPct !== 'number' ||
    !Number.isFinite(profitPct) ||
    typeof timestamp !== 'number' ||
    !Number.isFinite(timestamp)
  ) {
    return null;
  }

  return {
    id: `${value.symbol}:${buySource}:${sellSource}:${timestamp}`,
    symbol: value.symbol,
    buySource,
    sellSource,
    buyPrice: value.buy_price,
    sellPrice: value.sell_price,
    profitPct,
    timestamp,
  };
}

function parseAlertTrigger(value: unknown): AlertTrigger | null {
  if (!isRecord(value) || !isSymbol(value.symbol)) return null;
  if (
    !Number.isInteger(value.id) ||
    !Number.isInteger(value.rule_id) ||
    typeof value.rule_name !== 'string' ||
    !value.rule_name ||
    typeof value.buy_source !== 'string' ||
    !value.buy_source ||
    typeof value.sell_source !== 'string' ||
    !value.sell_source ||
    !isFinitePositive(value.buy_price) ||
    !isFinitePositive(value.sell_price) ||
    typeof value.gross_spread_pct !== 'number' ||
    !Number.isFinite(value.gross_spread_pct) ||
    typeof value.triggered_at_ms !== 'number' ||
    !Number.isFinite(value.triggered_at_ms)
  ) return null;
  return {
    id: value.id as number,
    ruleID: value.rule_id as number,
    ruleName: value.rule_name,
    symbol: value.symbol,
    buySource: value.buy_source,
    sellSource: value.sell_source,
    buyPrice: value.buy_price,
    sellPrice: value.sell_price,
    grossSpreadPct: value.gross_spread_pct,
    triggeredAtMS: value.triggered_at_ms,
  };
}

function appendHistoryPoint(points: PricePoint[], timestamp: number, value: number): PricePoint[] {
  const bucketTime = Math.floor(timestamp / HISTORY_BUCKET_MS) * HISTORY_BUCKET_MS;
  const cutoff = bucketTime - HISTORY_WINDOW_MS;
  const last = points.at(-1);

  if (last?.time === bucketTime) return points;

  let firstRetained = 0;
  while (firstRetained < points.length && points[firstRetained].time < cutoff) firstRetained += 1;
  const retained = firstRetained === 0 ? points : points.slice(firstRetained);

  if (!last || last.time < bucketTime) return [...retained, { time: bucketTime, value }];

  const byTime = new Map(retained.map((point) => [point.time, point.value]));
  byTime.set(bucketTime, value);
  return [...byTime.entries()].sort(([left], [right]) => left - right).map(([time, pointValue]) => ({ time, value: pointValue }));
}

function reduceSourcePrice(
  state: ScannerState,
  symbol: SymbolName,
  source: string,
  price: number,
  observedAt: number,
  receivedAt: number,
): ScannerState {
  const currentHistory = state.history[symbol]?.[source] ?? [];
  const nextHistory = appendHistoryPoint(currentHistory, observedAt, price);
  return {
    ...state,
    prices: {
      ...state.prices,
      [symbol]: { ...(state.prices[symbol] ?? {}), [source]: { price, updatedAt: observedAt } },
    },
    history:
      nextHistory === currentHistory
        ? state.history
        : {
            ...state.history,
            [symbol]: { ...(state.history[symbol] ?? {}), [source]: nextHistory },
          },
    lastUpdatedAt: receivedAt,
  };
}

function reduceLegacyPrices(
  state: ScannerState,
  prices: Record<string, Record<string, number>>,
  receivedAt: number,
): ScannerState {
  let next = state;
  for (const [symbol, sourcePrices] of Object.entries(prices)) {
    for (const [source, price] of Object.entries(sourcePrices)) {
      const current = next.prices[symbol]?.[source];
      if (current?.price === price) continue;
      next = reduceSourcePrice(next, symbol as SymbolName, source, price, receivedAt, receivedAt);
    }
  }
  return next;
}

export function reduceScannerMessage(state: ScannerState, message: unknown, now = Date.now()): ScannerState {
  if (!isRecord(message) || typeof message.type !== 'string') return state;

  if (message.type === 'prices') {
    const prices = parsePrices(message.prices);
    return prices ? reduceLegacyPrices(state, prices, now) : state;
  }

  if (message.type === 'price_update' && message.version === 1) {
    const price = parsePriceUpdate(message.price);
    if (!price) return state;
    const next = reduceSourcePrice(state, price.symbol, price.source, price.price, price.timestamp, now);
    return {
      ...next,
      feedEvents: appendFeedEvent(next, {
        id: `price:${price.source}:${price.symbol}:${price.timestamp}:${now}`,
        kind: 'price', source: price.source, symbol: price.symbol, price: price.price,
        timestamp: price.timestamp, receivedAt: now,
      }),
    };
  }

  if (message.type === 'quote_update' && message.version === 1) {
    const quote = parseQuote(message.quote);
    if (!quote) return state;
    const withPrice = reduceSourcePrice(
      state,
      quote.symbol,
      quote.source,
      (quote.bestBid + quote.bestAsk) / 2,
      quote.timestamp,
      now,
    );
    return {
      ...withPrice,
      quotes: {
        ...state.quotes,
        [quote.symbol]: { ...(state.quotes[quote.symbol] ?? {}), [quote.source]: quote },
      },
      feedEvents: appendFeedEvent(withPrice, {
        id: `quote:${quote.source}:${quote.symbol}:${quote.timestamp}:${now}`,
        kind: 'quote', source: quote.source, symbol: quote.symbol,
        bestBid: quote.bestBid, bestAsk: quote.bestAsk, timestamp: quote.timestamp, receivedAt: now,
      }),
    };
  }

  if (message.type === 'source_status' && message.version === 1) {
    const status = parseSourceStatus(message.status);
    if (!status) return state;
    return {
      ...state,
      connections: {
        ...state.connections,
        [status.source]: {
          source: status.source, connected: status.connected, symbols: status.symbols, updatedAt: status.timestamp,
        },
      },
      feedEvents: appendFeedEvent(state, {
        id: `connection:${status.source}:${status.connected}:${status.timestamp}`,
        kind: 'connection', source: status.source, symbols: status.symbols, connected: status.connected,
        timestamp: status.timestamp, receivedAt: now,
      }),
      lastUpdatedAt: now,
    };
  }

  if (message.type === 'spreads' && isSymbol(message.symbol)) {
    const spreads = parseSpreads(message.spreads);
    if (!spreads) return state;
    return {
      ...state,
      spreads: { ...state.spreads, [message.symbol]: spreads },
      lastUpdatedAt: now,
    };
  }

  if (message.type === 'arbitrage') {
    const opportunity = parseOpportunity(message.opportunity);
    if (!opportunity) return state;
    const opportunities = [
      opportunity,
      ...state.opportunities.filter(
        (item) =>
          item.symbol !== opportunity.symbol ||
          item.buySource !== opportunity.buySource ||
          item.sellSource !== opportunity.sellSource,
      ),
    ].slice(0, OPPORTUNITY_LIMIT);
    return {
      ...state,
      opportunities,
      feedEvents: appendFeedEvent(state, {
        id: `opportunity:${opportunity.id}:${now}`,
        kind: 'opportunity', symbol: opportunity.symbol, buySource: opportunity.buySource,
        sellSource: opportunity.sellSource, profitPct: opportunity.profitPct,
        timestamp: opportunity.timestamp, receivedAt: now,
      }),
      lastUpdatedAt: now,
    };
  }

  if (message.type === 'opportunities_snapshot' && message.version === 1 && isSymbol(message.symbol)) {
    if (!Array.isArray(message.opportunities)) return state;
    const opportunities = message.opportunities.map(parseOpportunity);
    if (opportunities.some((item) => item === null || item.symbol !== message.symbol)) return state;
    return {
      ...state,
      opportunities: [
        ...(opportunities as ArbitrageOpportunity[]),
        ...state.opportunities.filter((item) => item.symbol !== message.symbol),
      ].slice(0, OPPORTUNITY_LIMIT),
      lastUpdatedAt: now,
    };
  }

  if (message.type === 'alert_trigger' && message.version === 1) {
    const trigger = parseAlertTrigger(message.trigger);
    if (!trigger) return state;
    return {
      ...state,
      alertTriggers: [trigger, ...state.alertTriggers.filter((item) => item.id !== trigger.id)].slice(0, ALERT_TRIGGER_LIMIT),
      feedEvents: appendFeedEvent(state, {
        id: `alert:${trigger.id}:${now}`, kind: 'alert', symbol: trigger.symbol,
        buySource: trigger.buySource, sellSource: trigger.sellSource, profitPct: trigger.grossSpreadPct,
        timestamp: trigger.triggeredAtMS, receivedAt: now,
      }),
      lastUpdatedAt: now,
    };
  }

  return state;
}

export function selectBestOpportunity(
  state: ScannerState,
  symbol: string,
  enabledSources: Record<string, boolean>,
  now = Date.now(),
  minSpread = 0,
): ArbitrageOpportunity | null {
  return (
    state.opportunities
      .filter(
        (opportunity) =>
          opportunity.symbol === symbol &&
          opportunity.profitPct >= minSpread &&
          enabledSources[opportunity.buySource] !== false &&
          enabledSources[opportunity.sellSource] !== false &&
          now - opportunity.timestamp >= 0 &&
          now - opportunity.timestamp <= FRESHNESS_WINDOW_MS,
      )
      .sort((left, right) => right.profitPct - left.profitPct)[0] ?? null
  );
}

export function countFreshSources(
  state: ScannerState,
  symbol: string,
  now = Date.now(),
  enabledSources?: Record<string, boolean>,
): number {
  return Object.entries(state.prices[symbol] ?? {}).filter(
    ([source, sourcePrice]) => {
      const age = now - sourcePrice.updatedAt;
      return enabledSources?.[source] !== false && age >= 0 && age <= FRESHNESS_WINDOW_MS;
    },
  ).length;
}

export function countSourceConnections(
  state: ScannerState,
  symbol: string,
  enabledSources?: Record<string, boolean>,
): { connected: number; total: number } {
  const matching = Object.values(state.connections ?? {}).filter(
    (connection) => connection.symbols.includes(symbol as SymbolName) && enabledSources?.[connection.source] !== false,
  );
  return {
    connected: matching.filter((connection) => connection.connected).length,
    total: matching.length,
  };
}
