# COTI Scanner Design

## Goal

Add `COTIUSDT` to the existing public market-data scanner so it is selectable
in the browser and receives live prices from the scanner's existing exchanges
that currently support the market.

## Scope

- Add `COTIUSDT` to the scanner's supported-symbol configuration and UI.
- Subscribe to COTI only on existing sources whose public market catalog
  currently lists it: Binance spot, Binance futures, Bybit futures, Gate.io
  futures, and Kraken futures.
- Keep the existing BTC, ETH, XRP, and SOL behavior unchanged.
- Keep the scanner public and read-only. No API credentials, balances, orders,
  transfers, or trading endpoints are involved.
- KuCoin spot/futures connectors are explicitly outside this change. They will
  be designed and implemented separately after the KuCoin credential issue is
  resolved.

## Architecture

The backend will expose one complete UI symbol list and source-specific
subscription lists. This avoids sending `COTIUSDT` to connectors that do not
list it and prevents invalid-symbol errors from affecting their existing
streams. The existing exchange goroutines and price-processing channels remain
unchanged.

Kraken's adapter will map `COTIUSDT` to its futures product identifier and map
incoming updates back to the scanner's standard `COTIUSDT` name. The static UI
dropdown will add a matching `COTIUSDT` option.

## Data Flow

1. `main` obtains the source-specific symbol list for each connector.
2. Supported connectors subscribe to COTI alongside their existing symbols.
3. Connectors normalize the incoming symbol to `COTIUSDT` and publish best
   bid/ask data through the existing channel.
4. The backend broadcasts the unchanged message format to the browser.
5. Selecting COTI in the existing dropdown filters the chart, source list,
   spread matrix, and alert view to COTI updates.

## Error Handling

- Unsupported sources never receive a COTI subscription.
- Existing connector reconnect behavior remains unchanged.
- A temporarily unavailable supported source may be absent from the COTI view;
  it must not prevent other sources from updating.

## Testing and Acceptance

- Unit tests prove the per-source symbol selection includes COTI only for the
  supported existing connectors.
- Unit tests prove Kraken converts `COTIUSDT` in both directions.
- A UI regression test proves the dropdown contains `COTIUSDT`.
- The full Go test suite and Go build pass.
- A rebuilt Docker scanner returns HTTP 200 and its live WebSocket produces a
  valid COTI update from at least two supported sources within a bounded wait.
- The scanner UI remains available at `http://localhost:8082`.

## Non-Goals

- KuCoin integration.
- Automated trading or credential use.
- Fee-adjusted executable-profit calculation.
- Order-book depth, slippage, transfer, or inventory modeling.
