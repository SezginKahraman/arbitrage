import { useCallback, useEffect, useMemo, useState } from 'react';

import type {
  SymbolName,
  TransferNetworkMatch,
  TransferRouteEvaluation,
  TransferRouteRequestStatus,
  TransferRouteStatus,
  VenueAssetNetwork,
} from '../app/types';

export type TransferRouteFetcher = typeof fetch;

interface UseTransferRouteOptions {
  symbol: SymbolName;
  source?: string;
  destination?: string;
  fetcher?: TransferRouteFetcher;
  refreshIntervalMs?: number;
}

interface TransferRouteState {
  route: TransferRouteEvaluation | null;
  status: TransferRouteRequestStatus;
  retry: () => void;
}

const transferRouteStatuses = new Set<TransferRouteStatus>(['ready', 'blocked', 'check', 'unknown', 'not_applicable']);
const defaultFetcher: TransferRouteFetcher = (input, init) => fetch(input, init);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringValue(record: Record<string, unknown>, key: string): string | null {
  const value = record[key];
  return typeof value === 'string' ? value : null;
}

function optionalString(record: Record<string, unknown>, key: string): string {
  const value = record[key];
  return typeof value === 'string' ? value : '';
}

function numberValue(record: Record<string, unknown>, key: string, fallback = 0): number | null {
  const value = record[key];
  if (value === undefined) return fallback;
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

function parseVenueNetwork(value: unknown): VenueAssetNetwork | null {
  if (!isRecord(value)) return null;
  const asset = stringValue(value, 'asset');
  const networkID = stringValue(value, 'network_id');
  const rawNetworkID = stringValue(value, 'raw_network_id');
  const name = stringValue(value, 'name');
  const confirmations = numberValue(value, 'confirmations');
  const checkedAt = numberValue(value, 'checked_at');
  if (
    asset === null || networkID === null || rawNetworkID === null || name === null ||
    typeof value.deposit_enabled !== 'boolean' || typeof value.withdraw_enabled !== 'boolean' ||
    confirmations === null || checkedAt === null
  ) return null;
  return {
    asset,
    networkID,
    rawNetworkID,
    name,
    contractAddress: optionalString(value, 'contract_address'),
    depositEnabled: value.deposit_enabled,
    withdrawEnabled: value.withdraw_enabled,
    withdrawalFee: optionalString(value, 'withdrawal_fee'),
    minimumWithdrawal: optionalString(value, 'minimum_withdrawal'),
    confirmations,
    checkedAt,
  };
}

function parseNetworkMatch(value: unknown): TransferNetworkMatch | null {
  if (!isRecord(value)) return null;
  const networkID = stringValue(value, 'network_id');
  const name = stringValue(value, 'name');
  const status = stringValue(value, 'status');
  const reason = stringValue(value, 'reason');
  if (
    networkID === null || name === null || status === null || !transferRouteStatuses.has(status as TransferRouteStatus) ||
    reason === null || typeof value.source_withdraw_enabled !== 'boolean' ||
    typeof value.destination_deposit_enabled !== 'boolean'
  ) return null;
  return {
    networkID,
    name,
    status: status as TransferRouteStatus,
    reason,
    sourceWithdrawEnabled: value.source_withdraw_enabled,
    destinationDepositEnabled: value.destination_deposit_enabled,
    withdrawalFee: optionalString(value, 'withdrawal_fee'),
    minimumWithdrawal: optionalString(value, 'minimum_withdrawal'),
    contractAddress: optionalString(value, 'contract_address'),
  };
}

function parseArray<T>(value: unknown, parser: (item: unknown) => T | null): T[] | null {
  if (!Array.isArray(value)) return null;
  const result: T[] = [];
  for (const item of value) {
    const parsed = parser(item);
    if (parsed === null) return null;
    result.push(parsed);
  }
  return result;
}

export function parseTransferRoute(value: unknown): TransferRouteEvaluation | null {
  if (!isRecord(value)) return null;
  const asset = stringValue(value, 'asset');
  const source = stringValue(value, 'source');
  const destination = stringValue(value, 'destination');
  const status = stringValue(value, 'status');
  const reason = stringValue(value, 'reason');
  const checkedAt = numberValue(value, 'checked_at');
  const networks = parseArray(value.networks, parseNetworkMatch);
  const sourceNetworks = parseArray(value.source_networks, parseVenueNetwork);
  const destinationNetworks = parseArray(value.destination_networks, parseVenueNetwork);
  if (
    asset === null || source === null || destination === null || status === null ||
    !transferRouteStatuses.has(status as TransferRouteStatus) || reason === null || checkedAt === null ||
    networks === null || sourceNetworks === null || destinationNetworks === null
  ) return null;
  return {
    asset,
    source,
    destination,
    status: status as TransferRouteStatus,
    reason,
    checkedAt,
    networks,
    sourceNetworks,
    destinationNetworks,
  };
}

function notApplicableRoute(symbol: SymbolName, source: string, destination: string): TransferRouteEvaluation {
  return {
    asset: symbol.replace(/USDT$/, ''),
    source,
    destination,
    status: 'not_applicable',
    reason: 'network checks apply to spot-to-spot routes',
    checkedAt: 0,
    networks: [],
    sourceNetworks: [],
    destinationNetworks: [],
  };
}

export function useTransferRoute({
  symbol,
  source,
  destination,
  fetcher = defaultFetcher,
  refreshIntervalMs = 60_000,
}: UseTransferRouteOptions): TransferRouteState {
  const isComplete = Boolean(source && destination);
  const isSpotRoute = Boolean(source?.endsWith('_spot') && destination?.endsWith('_spot'));
  const syntheticRoute = useMemo(
    () => isComplete && !isSpotRoute ? notApplicableRoute(symbol, source!, destination!) : null,
    [destination, isComplete, isSpotRoute, source, symbol],
  );
  const [result, setResult] = useState<Omit<TransferRouteState, 'retry'>>(() => ({
    route: syntheticRoute,
    status: !isComplete ? 'idle' : syntheticRoute ? 'ready' : 'loading',
  }));
  const [retryVersion, setRetryVersion] = useState(0);
  const retry = useCallback(() => setRetryVersion((value) => value + 1), []);

  useEffect(() => {
    if (!isComplete) {
      setResult({ route: null, status: 'idle' });
      return;
    }
    if (syntheticRoute) {
      setResult({ route: syntheticRoute, status: 'ready' });
      return;
    }

    let stopped = false;
    let activeController: AbortController | null = null;
    const asset = symbol.replace(/USDT$/, '');
    const endpoint = `/api/transfer-route?asset=${encodeURIComponent(asset)}&source=${encodeURIComponent(source!)}&destination=${encodeURIComponent(destination!)}`;
    const load = async (initial: boolean) => {
      activeController?.abort();
      const controller = new AbortController();
      activeController = controller;
      if (initial) setResult({ route: null, status: 'loading' });
      try {
        const response = await fetcher(endpoint, { signal: controller.signal });
        if (!response.ok) throw new Error('transfer route request failed');
        const route = parseTransferRoute(await response.json());
        if (route === null) throw new Error('invalid transfer route response');
        if (!stopped) setResult({ route, status: 'ready' });
      } catch (error) {
        if (!stopped && !(error instanceof DOMException && error.name === 'AbortError')) {
          setResult({ route: null, status: 'degraded' });
        }
      }
    };

    void load(true);
    const timer = window.setInterval(() => void load(false), refreshIntervalMs);
    return () => {
      stopped = true;
      activeController?.abort();
      window.clearInterval(timer);
    };
  }, [destination, fetcher, isComplete, refreshIntervalMs, retryVersion, source, symbol, syntheticRoute]);

  return { ...result, retry };
}
