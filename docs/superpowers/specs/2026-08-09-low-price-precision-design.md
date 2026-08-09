# Low-Price Precision Design

## Goal

Display low-priced markets such as COTI with enough precision to distinguish
exchange prices visually. Prices below 1 USDT will use eight decimal places in
chart labels and all existing price text surfaces.

## Selected Approach

Use adaptive value-based precision across all symbols:

| Price | Decimal places |
|---:|---:|
| `>= 1000` | 2 |
| `>= 100` | 3 |
| `>= 10` | 4 |
| `>= 1` | 5 |
| `< 1` | 8 |

This keeps BTC, ETH, and SOL readable while displaying a value such as
`0.01140723` without rounding it to `0.01`. A fixed eight-decimal format for
every asset was rejected because it would add noise to high-priced markets. A
COTI-only exception was rejected because the same issue applies to any future
low-priced symbol.

## Components

- Add a small browser-compatible price-formatting module containing the pure
  precision, text-formatting, and Lightweight Charts `priceFormat` rules.
- Load that module before `app.js`.
- Make the existing `formatPrice` method delegate to the shared formatter so
  source prices and opportunity buy/sell prices use eight decimals below 1.
- Apply the matching `priceFormat` (`precision: 8`, `minMove: 0.00000001`) to
  each chart series when it receives a low price.
- Cache the last applied precision per source so a chart series is not
  reconfigured on every market update.
- Clear the precision cache when the selected symbol changes, allowing the
  next symbol's first price to configure the series correctly.

## Testing

- Node's built-in test runner will exercise the real pure formatting module.
- Literal cases will cover every precision boundary and require
  `0.01140723` to remain `0.01140723`.
- Tests will verify the chart format for low prices is exactly
  `{ type: "price", precision: 8, minMove: 0.00000001 }`.
- Existing Go tests, vet, and build will remain green.
- After restarting the scanner, the UI will continue to return HTTP 200 and
  live COTI WebSocket data will remain available from at least two sources.

## Scope Boundary

- Do not redesign the chart, spread matrix, or opportunities table layout.
- Do not change percentage precision in this iteration.
- Do not modify backend price calculations or WebSocket payloads.
- Do not add KuCoin market-data connectors in this iteration.
