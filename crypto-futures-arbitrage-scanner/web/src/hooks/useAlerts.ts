import { useCallback, useEffect, useState } from 'react';

import {
	  isSymbolName,
  type AlertMarketMode,
  type AlertRule,
  type AlertRuleInput,
  type AlertTrigger,
  type SymbolName,
} from '../app/types';

export type AlertFetcher = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
type AlertStatus = 'loading' | 'ready' | 'degraded';

interface AlertState {
  rules: AlertRule[];
  triggers: AlertTrigger[];
  status: AlertStatus;
  createRule: (input: AlertRuleInput) => Promise<AlertRule>;
  updateRule: (id: number, input: AlertRuleInput) => Promise<AlertRule>;
  retry: () => void;
}

const marketModes = new Set<AlertMarketMode>(['all', 'spot', 'mixed', 'futures']);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function finite(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function normalizeRule(value: unknown): AlertRule | null {
  if (!isRecord(value)) return null;
  const symbol = value.symbol;
  const marketMode = value.market_mode;
  if (
    !Number.isInteger(value.id) || typeof value.name !== 'string' || !value.name ||
    typeof symbol !== 'string' || (symbol !== '' && !isSymbolName(symbol)) ||
    typeof marketMode !== 'string' || !marketModes.has(marketMode as AlertMarketMode) ||
    typeof value.buy_source !== 'string' || typeof value.sell_source !== 'string' ||
    !finite(value.min_spread_pct) || !Number.isInteger(value.cooldown_seconds) ||
    typeof value.enabled !== 'boolean' || typeof value.browser_enabled !== 'boolean' ||
    !finite(value.created_at_ms) || !finite(value.updated_at_ms) ||
    !(value.last_triggered_at_ms === null || value.last_triggered_at_ms === undefined || finite(value.last_triggered_at_ms))
  ) return null;
  return {
    id: value.id as number,
    name: value.name,
    symbol: symbol as SymbolName | '',
    marketMode: marketMode as AlertMarketMode,
    buySource: value.buy_source,
    sellSource: value.sell_source,
    minSpreadPct: value.min_spread_pct,
    cooldownSeconds: value.cooldown_seconds as number,
    enabled: value.enabled,
    browserEnabled: value.browser_enabled,
    createdAtMS: value.created_at_ms,
    updatedAtMS: value.updated_at_ms,
    lastTriggeredAtMS: (value.last_triggered_at_ms as number | null | undefined) ?? null,
  };
}

function normalizeTrigger(value: unknown): AlertTrigger | null {
  if (!isRecord(value) || !isSymbolName(value.symbol)) return null;
  if (
    !Number.isInteger(value.id) || !Number.isInteger(value.rule_id) ||
    typeof value.rule_name !== 'string' || typeof value.buy_source !== 'string' || typeof value.sell_source !== 'string' ||
    !finite(value.buy_price) || !finite(value.sell_price) || !finite(value.gross_spread_pct) || !finite(value.triggered_at_ms)
  ) return null;
  return {
    id: value.id as number,
    ruleID: value.rule_id as number,
    ruleName: value.rule_name,
    symbol: value.symbol as SymbolName,
    buySource: value.buy_source,
    sellSource: value.sell_source,
    buyPrice: value.buy_price,
    sellPrice: value.sell_price,
    grossSpreadPct: value.gross_spread_pct,
    triggeredAtMS: value.triggered_at_ms,
  };
}

function normalizeEnvelope<T>(value: unknown, normalize: (item: unknown) => T | null): T[] {
  if (!isRecord(value) || !Array.isArray(value.items)) throw new Error('Invalid alert API payload');
  const items = value.items.map(normalize);
  if (items.some((item) => item === null)) throw new Error('Invalid alert API item');
  return items as T[];
}

function serializeRule(input: AlertRuleInput) {
  return JSON.stringify({
    name: input.name,
    symbol: input.symbol,
    market_mode: input.marketMode,
    buy_source: input.buySource,
    sell_source: input.sellSource,
    min_spread_pct: input.minSpreadPct,
    cooldown_seconds: input.cooldownSeconds,
    enabled: input.enabled,
    browser_enabled: input.browserEnabled,
  });
}

function mergeTriggers(current: AlertTrigger[], incoming: AlertTrigger[]) {
  const byID = new Map([...incoming, ...current].map((item) => [item.id, item]));
  return [...byID.values()].sort((left, right) => right.triggeredAtMS - left.triggeredAtMS).slice(0, 100);
}

export function useAlerts(liveTriggers: AlertTrigger[], fetcher: AlertFetcher = fetch): AlertState {
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [triggers, setTriggers] = useState<AlertTrigger[]>([]);
  const [status, setStatus] = useState<AlertStatus>('loading');
  const [retryVersion, setRetryVersion] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setStatus('loading');
    void Promise.all([
      fetcher('/api/alert-rules', { signal: controller.signal }),
      fetcher('/api/alert-triggers?limit=100', { signal: controller.signal }),
    ]).then(async ([ruleResponse, triggerResponse]) => {
      if (!ruleResponse.ok || !triggerResponse.ok) throw new Error('Alert API unavailable');
      const nextRules = normalizeEnvelope(await ruleResponse.json(), normalizeRule);
      const nextTriggers = normalizeEnvelope(await triggerResponse.json(), normalizeTrigger);
      if (!controller.signal.aborted) {
        setRules(nextRules);
        setTriggers((current) => mergeTriggers(current, nextTriggers));
        setStatus('ready');
      }
    }).catch((error: unknown) => {
      if (controller.signal.aborted || (error instanceof DOMException && error.name === 'AbortError')) return;
      setStatus('degraded');
    });
    return () => controller.abort();
  }, [fetcher, retryVersion]);

  useEffect(() => {
    if (liveTriggers.length) setTriggers((current) => mergeTriggers(current, liveTriggers));
  }, [liveTriggers]);

  const createRule = useCallback(async (input: AlertRuleInput) => {
    const response = await fetcher('/api/alert-rules', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: serializeRule(input),
    });
    if (!response.ok) throw new Error('Could not create alert');
    const rule = normalizeRule(await response.json());
    if (!rule) throw new Error('Invalid created alert');
    setRules((current) => [rule, ...current]);
    return rule;
  }, [fetcher]);

  const updateRule = useCallback(async (id: number, input: AlertRuleInput) => {
    const response = await fetcher(`/api/alert-rules/${id}`, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: serializeRule(input),
    });
    if (!response.ok) throw new Error('Could not update alert');
    const rule = normalizeRule(await response.json());
    if (!rule) throw new Error('Invalid updated alert');
    setRules((current) => current.map((item) => item.id === id ? rule : item));
    return rule;
  }, [fetcher]);

  return { rules, triggers, status, createRule, updateRule, retry: () => setRetryVersion((value) => value + 1) };
}
