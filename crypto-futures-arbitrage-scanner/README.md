# Crypto Arbitrage Scanner

Real-time spot and futures market scanner with a Go backend and a React dashboard. It compares executable best ask → best bid routes, streams quotes over WebSocket, and keeps short opportunity sessions in SQLite.

The Scanner and Opportunities screens remain observation-only and never place orders or transfer assets. The separate Futures workspace can place a manually confirmed, delta-neutral Binance Futures ↔ Gate Futures entry when live trading is explicitly enabled on the server.

## Markets

- Pairs: `BTCUSDT`, `ETHUSDT`, `XRPUSDT`, `SOLUSDT`, `COTIUSDT`
- Futures: Binance, Bybit, Hyperliquid, Kraken, OKX, Gate.io, Paradex
- Spot: Binance, Bybit, Gate.io
- Reference feed: Pyth

Not every source supports every pair. Unsupported combinations are omitted automatically.

## Dashboard

The React + Tailwind dashboard provides:

- executable buy/sell routes based on best ask and best bid
- separate Spot, Futures, and cross-market Spot ↔ Futures comparison modes
- market-level visibility controls shared by the route, metrics, table, and chart
- collapsible opportunities plus persistent split/stacked panel layouts
- ascending/descending sorting on every opportunity-table column
- eight-decimal price precision for low-priced assets such as COTI
- live source status and reconnect handling
- TradingView Lightweight Charts price comparison
- persistent pair, threshold, chart range, and source selections
- SQLite-backed opportunity history with live/history labels and peak spread
- explicit unknown states for network, fee, and transfer checks that are not yet verified

## Futures decision lab

Open `/futures` or choose **Futures** in the sidebar to analyze a selected Gate.io USDT perpetual contract on demand. The workspace provides:

- `15m`, `1h`, and `4h` candlestick analysis
- trend pullback, range breakout, and mean-reversion strategies
- separate `ready`, `watch`, or `no trade` long and short plans
- entry, invalidation stop, liquidity-aware target, gross risk/reward, contract sizing, and estimated round-trip fees
- EMA20/EMA50, ATR14, RSI14, volatility bands, funding, mark/index price, and visible bid/ask liquidity walls
- a live-updating Gate candle after analysis
- Binance Futures ↔ Gate Futures order-book comparison in both long/short directions
- contract-multiplier-aware matched quantities, four taker fees, depth, next-funding carry, and index-divergence checks
- an explicit two-leg execution button that revalidates both books after private account preflight
- immediate reduce-only compensation if one order fails or either order partially fills

`Analyze market` only uses public data and never places an order. `Execute both futures legs` is the only UI action that can submit real orders. It opens the cheaper venue long and the richer venue short only when the revalidated projected convergence return still clears the selected threshold. A separate localhost API probe can submit one post-only order capped at 10 USDT, cancel it immediately, and verify the venue is flat; it also requires explicit confirmation and the server kill-switch. Funding is displayed separately and is not included in projected net return.

Live entry requires all of the following environment variables:

```sh
LIVE_FUTURES_TRADING_ENABLED=true
BINANCE_API_KEY=...
BINANCE_API_SECRET=...
GATEIO_API_KEY=...
GATEIO_API_SECRET=...
BINANCE_FUTURES_TAKER_FEE=0.0005
```

Use futures-trading permission only; withdrawal permission is not required and should remain disabled. The current release opens and compensates the entry legs but does not automatically take profit or close a converged pair. After an opened result, manage or close both positions directly on Binance and Gate until an explicit paired-exit workflow is added. Existing SQLite paper-position records and legacy API routes are retained for compatibility but are no longer shown in the Futures UI.

## Run with Docker

Build and start the complete production application:

```sh
docker build -t arbitrage-scanner:local .
docker run --name arbitrage-scanner \
  -p 127.0.0.1:8082:8082 \
  -v arbitrage-scanner-data:/app/data \
  arbitrage-scanner:local
```

Open `http://127.0.0.1:8082`.

The SQLite database is stored at `/app/data/scanner.db` in the container. Override it with `SCANNER_DB_PATH` when running without Docker.

## Develop locally

Backend requirements: Go 1.23.5 or newer.

```sh
go run .
```

Frontend requirements: Node.js 22.12 or newer.

```sh
cd web
npm ci
npm run dev
```

Vite serves the development UI at `http://127.0.0.1:5173` and proxies `/ws` and `/api` to the Go server on port 8082.

## Verify

```sh
go test ./...
go vet ./...

cd web
npm test -- --run
npm run typecheck
npm run build
```

## API

- `GET /api/health` — scanner/database health
- `GET /api/opportunities?symbol=COTIUSDT&minSpread=0.5&limit=100` — recent opportunity sessions
- `GET /api/futures/contracts` — active Gate.io USDT perpetual contracts
- `POST /api/futures/analyze` — on-demand public-data technical and cross-venue analysis
- `GET /api/futures/live` — refreshed candle, Binance/Gate books, and neutral route for the selected market
- `GET /api/futures/access` — read-only Binance/Gate Futures credential, margin, and flat-position preflight
- `POST /api/futures/order-probe` — explicitly confirmed, at-most-10-USDT post-only order/cancel access probe
- `POST /api/futures/execute` — manually confirmed, server-revalidated live two-leg entry
- `GET|POST /api/futures/paper-positions` — retained legacy local paper-position API
- `PUT /api/futures/paper-positions/{id}/close` — retained legacy paper close API
- `GET /ws` — live versioned quote, reference-price, and opportunity messages

Opportunity history is retained for seven days. If SQLite is unavailable, live scanning continues and the UI reports history as degraded.
