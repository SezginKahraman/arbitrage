export interface SourceMeta {
  key: string;
  label: string;
  shortLabel: string;
  market: 'spot' | 'futures' | 'oracle';
  color: string;
}

export const SOURCES: readonly SourceMeta[] = [
  { key: 'binance_futures', label: 'Binance Futures', shortLabel: 'BIN-F', market: 'futures', color: '#f4b72a' },
  { key: 'binance_spot', label: 'Binance Spot', shortLabel: 'BIN-S', market: 'spot', color: '#f4b72a' },
  { key: 'bybit_futures', label: 'Bybit Futures', shortLabel: 'BYB-F', market: 'futures', color: '#8b6cf6' },
  { key: 'bybit_spot', label: 'Bybit Spot', shortLabel: 'BYB-S', market: 'spot', color: '#fb923c' },
  { key: 'gate_futures', label: 'Gate.io Futures', shortLabel: 'GAT-F', market: 'futures', color: '#3b82f6' },
  { key: 'kraken_futures', label: 'Kraken Futures', shortLabel: 'KRK-F', market: 'futures', color: '#7c6cff' },
  { key: 'hyperliquid_futures', label: 'Hyperliquid Futures', shortLabel: 'HYP-F', market: 'futures', color: '#6ee7d2' },
  { key: 'okx_futures', label: 'OKX Futures', shortLabel: 'OKX-F', market: 'futures', color: '#60a5fa' },
  { key: 'paradex_futures', label: 'Paradex Futures', shortLabel: 'PDX-F', market: 'futures', color: '#fb7185' },
  { key: 'pyth', label: 'Pyth Oracle', shortLabel: 'PYTH', market: 'oracle', color: '#27e58c' },
];

export const SOURCE_BY_KEY = Object.fromEntries(SOURCES.map((source) => [source.key, source])) as Record<
  string,
  SourceMeta
>;

export const DEFAULT_ENABLED_SOURCES = Object.fromEntries(SOURCES.map((source) => [source.key, true]));

export function sourceMeta(source: string): SourceMeta {
  return (
    SOURCE_BY_KEY[source] ?? {
      key: source,
      label: source.replaceAll('_', ' '),
      shortLabel: source.slice(0, 5).toUpperCase(),
      market: 'futures',
      color: '#8a9aa3',
    }
  );
}
