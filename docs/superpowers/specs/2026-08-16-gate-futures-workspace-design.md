# Gate Futures Workspace Design

## Scope

The existing arbitrage Scanner, Opportunities, and Alerts workspaces remain unchanged. A new `/futures` workspace adds on-demand analysis for Gate.io USDT perpetual contracts and paper trading. The first release does not expose or call any exchange mutation endpoint.

## User flow

1. Open **Futures** from the sidebar.
2. Choose a Gate.io USDT perpetual pair, timeframe (`15m`, `1h`, or `4h`), strategy, paper account size, and risk percentage.
3. Press **Analyze market**.
4. Review the candlestick chart, market regime, funding, order-book liquidity walls, and separate long/short plans.
5. A plan is either `ready`, `watch`, or `no_trade`. Every plan shows entry, stop, targets, risk/reward, estimated fee impact, and the reasons behind the state.
6. A `ready` or `watch` plan can be opened as a paper position. Paper positions are persisted in SQLite and never reach Gate.io.

## Analysis model

The model is deterministic and explainable. It uses EMA20/EMA50, ATR14, RSI14, recent swing levels, Gate contract metadata, and top-of-book liquidity walls. The available strategies are:

- `trend_pullback`: align with EMA20/EMA50 and seek an entry near the fast average.
- `breakout`: evaluate the most recent 20-candle range and place the trigger beyond the range with an ATR buffer.
- `mean_reversion`: use RSI and a 20-period volatility band to target a return toward the mean.

The engine produces both long and short plans and may return `no_trade`. It does not predict certainty and it does not label a setup as profitable.

## Architecture

- `futures/gate_client.go`: public Gate REST client for contracts, candles, and order-book depth.
- `futures/analysis.go`: pure indicator and scenario calculations, independent from HTTP and exchange I/O.
- `storage/paper_positions.go`: SQLite-backed paper-position lifecycle.
- `api.go`: typed `/api/futures/*` endpoints backed by interfaces, so API tests never call Gate.
- `web/src/components/futures/*`: Futures workspace, candlestick chart, strategy controls, plans, and paper ledger.

Credentials stay on the backend. This release does not read Gate credentials because all required analysis data is public. A later live-trading release must add a separate execution service, an explicit server feature flag, isolated-margin limits, idempotent client order IDs, mandatory stop protection, daily-loss limits, and a kill switch.

## Visual direction

The page keeps the existing terminal palette and typography. Its signature element is a split **decision rail** beside the candlestick chart: long and short scenarios mirror each other around the current mark price, with entry, invalidation, and targets encoded as chart lines and compact risk cards. Mint is reserved for viable long actions, coral for short/invalidation risk, amber for watch states, and slate for unavailable data.

## Error and safety behavior

- Invalid symbols, intervals, strategies, account sizes, and risk percentages return `400`.
- Gate timeouts and malformed responses return a sanitized `502`; no upstream body is forwarded.
- Insufficient candle or order-book data returns a successful analysis with `no_trade`, not a fabricated setup.
- Paper positions reject zero/negative size, unknown directions, and analysis data without a valid stop.
- The UI labels the module `Paper` and contains no live-order control in this release.
