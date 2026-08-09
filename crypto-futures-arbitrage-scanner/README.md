# Crypto Arbitrage Scanner

Real-time spot and futures market scanner with a Go backend and a React dashboard. It compares executable best ask → best bid routes, streams quotes over WebSocket, and keeps short opportunity sessions in SQLite.

The scanner is observation-only. It does not place orders, transfer assets, or use exchange credentials.

## Markets

- Pairs: `BTCUSDT`, `ETHUSDT`, `XRPUSDT`, `SOLUSDT`, `COTIUSDT`
- Futures: Binance, Bybit, Hyperliquid, Kraken, OKX, Gate.io, Paradex
- Spot: Binance, Bybit
- Reference feed: Pyth

Not every source supports every pair. Unsupported combinations are omitted automatically.

## Dashboard

The React + Tailwind dashboard provides:

- executable buy/sell routes based on best ask and best bid
- eight-decimal price precision for low-priced assets such as COTI
- live source status and reconnect handling
- TradingView Lightweight Charts price comparison
- persistent pair, threshold, chart range, and source selections
- SQLite-backed opportunity history with live/history labels and peak spread
- explicit unknown states for network, fee, and transfer checks that are not yet verified

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
- `GET /ws` — live versioned quote, reference-price, and opportunity messages

Opportunity history is retained for seven days. If SQLite is unavailable, live scanning continues and the UI reports history as degraded.
