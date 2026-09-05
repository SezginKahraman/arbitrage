# Gate Futures Analysis and Paper Trading Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an isolated Gate.io Futures workspace that analyzes selected USDT perpetual pairs and persists paper positions without submitting live orders.

**Architecture:** A public Gate REST adapter supplies normalized candles, contract metadata, and depth to a pure deterministic analysis engine. The Go API exposes on-demand analysis and a SQLite paper ledger; React renders the candlestick decision workspace through focused components.

**Tech Stack:** Go 1.24, `net/http`, modernc SQLite, React 19, TypeScript, Tailwind CSS 4, lightweight-charts 5, Vitest.

## Global Constraints

- Preserve all existing arbitrage pages and behavior.
- Gate.io USDT perpetuals are the only venue in this release.
- Never call or expose a live trading endpoint.
- Never send API credentials to the browser or include upstream response bodies in errors.
- Follow RED-GREEN-REFACTOR for every production behavior.

---

### Task 1: Pure futures analysis engine and Gate public data adapter

**Files:**
- Create: `crypto-futures-arbitrage-scanner/futures/types.go`
- Create: `crypto-futures-arbitrage-scanner/futures/analysis.go`
- Create: `crypto-futures-arbitrage-scanner/futures/analysis_test.go`
- Create: `crypto-futures-arbitrage-scanner/futures/gate_client.go`
- Create: `crypto-futures-arbitrage-scanner/futures/gate_client_test.go`

**Interfaces:**
- Produces: `type Service interface { Contracts(context.Context) ([]Contract, error); Analyze(context.Context, AnalyzeRequest) (Analysis, error) }`
- Produces: `func AnalyzeMarket(AnalyzeRequest, Contract, []Candle, OrderBook, time.Time) (Analysis, error)`

- [x] **Step 1: Write failing pure-engine tests** for indicator warm-up, invalid inputs, long/short symmetry, risk/reward, and `no_trade` behavior using literal candle/depth fixtures.
- [x] **Step 2: Run `go test ./futures -run 'TestAnalyze' -count=1`** and confirm failures are caused by missing analysis behavior.
- [x] **Step 3: Implement normalized futures types and the minimum EMA/ATR/RSI/swing/liquidity calculations** required by the tests.
- [x] **Step 4: Run the focused tests** and confirm they pass.
- [x] **Step 5: Write failing Gate adapter tests** against an `httptest.Server`, asserting exact public paths and sanitized upstream failures.
- [x] **Step 6: Implement the bounded public Gate client** for `/contracts`, `/candlesticks`, and `/order_book` with a 10-second client timeout and strict response validation.
- [x] **Step 7: Run `go test ./futures -count=1`** and confirm all adapter and engine tests pass.

### Task 2: Futures API and SQLite paper ledger

**Files:**
- Create: `crypto-futures-arbitrage-scanner/storage/paper_positions.go`
- Create: `crypto-futures-arbitrage-scanner/storage/paper_positions_test.go`
- Create: `crypto-futures-arbitrage-scanner/api_futures_test.go`
- Modify: `crypto-futures-arbitrage-scanner/storage/sqlite.go`
- Modify: `crypto-futures-arbitrage-scanner/api.go`
- Modify: `crypto-futures-arbitrage-scanner/main.go`

**Interfaces:**
- Produces: `GET /api/futures/contracts`
- Produces: `POST /api/futures/analyze`
- Produces: `GET|POST /api/futures/paper-positions`
- Produces: `PUT /api/futures/paper-positions/{id}/close`

- [x] **Step 1: Write failing storage tests** for creating, listing, validating, and closing a paper position with realized PnL.
- [x] **Step 2: Run `go test ./storage -run Paper -count=1`** and confirm the new behavior is absent.
- [x] **Step 3: Add the paper table and repository methods** with parameterized SQL and server timestamps.
- [x] **Step 4: Run the focused storage tests** and confirm they pass.
- [x] **Step 5: Write failing HTTP tests** using a fake futures service and real in-memory SQLite store; assert method constraints, validation, stable envelopes, and sanitized `502` errors.
- [x] **Step 6: Register the futures dependency and handlers** and construct the public Gate service in `run()`.
- [x] **Step 7: Run `go test ./... -count=1`** and confirm the complete Go suite passes.

### Task 3: Typed client hooks and navigation

**Files:**
- Create: `crypto-futures-arbitrage-scanner/web/src/hooks/useFuturesWorkspace.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/hooks/useFuturesWorkspace.test.tsx`
- Modify: `crypto-futures-arbitrage-scanner/web/src/app/types.ts`
- Modify: `crypto-futures-arbitrage-scanner/web/src/app/navigation.ts`
- Modify: `crypto-futures-arbitrage-scanner/web/src/components/layout/Sidebar.tsx`
- Modify: `crypto-futures-arbitrage-scanner/web/src/app/App.tsx`

**Interfaces:**
- Produces: `useFuturesWorkspace()` with contract loading, `analyze(input)`, paper-position creation/list/close, explicit `idle|loading|ready|error` states, and retryable messages.

- [x] **Step 1: Write failing hook and navigation tests** for `/futures`, contract loading, analyze submission, sanitized errors, and paper ledger refresh.
- [x] **Step 2: Run the focused Vitest files** and confirm failures name the missing Futures behavior.
- [x] **Step 3: Add types, route, sidebar item, and the fetch hook** without changing Scanner state or preferences.
- [x] **Step 4: Run the focused tests** and confirm they pass.

### Task 4: Futures decision workspace UI

**Files:**
- Create: `crypto-futures-arbitrage-scanner/web/src/components/futures/FuturesPage.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/futures/FuturesPage.test.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/futures/FuturesChart.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/futures/FuturesChart.test.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/futures/ScenarioCard.tsx`

**Interfaces:**
- Consumes: `useFuturesWorkspace()` and typed `FuturesAnalysis` data.
- Produces: an accessible Analyze form, candlestick chart with entry/stop/target price lines, mirrored scenario cards, liquidity wall table, and clearly labelled paper ledger.

- [x] **Step 1: Write failing component tests** for explicit analyze action, empty/loading/error states, plan metrics, `no_trade`, and paper action labeling.
- [x] **Step 2: Run the focused component tests** and confirm failures are caused by missing UI.
- [x] **Step 3: Implement the page and chart** using the existing terminal tokens, responsive stacked layout, keyboard-visible controls, and reduced-motion-safe transitions.
- [x] **Step 4: Run focused component tests, then `npm test -- --run` and `npm run typecheck`**.
- [x] **Step 5: Self-critique the rendered structure** and remove decorative elements that do not help entry/risk/target decisions.

### Task 5: Integration verification and operational documentation

**Files:**
- Modify: `crypto-futures-arbitrage-scanner/README.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Verifies: no Gate credentials are required for analysis and no mutating Gate path exists in the new production code.

- [x] **Step 1: Document the Futures workspace, paper-only boundary, and local verification commands.**
- [x] **Step 2: Run Go tests/race/vet/build and frontend tests/typecheck/build.**
- [x] **Step 3: Build the Docker image and smoke `GET /futures`, contract discovery, and one COTI analysis.**
- [x] **Step 4: Inspect the full diff and search new production files for `/orders`, credential access, and accidental live-trade controls.**
- [x] **Step 5: Record exact verification evidence in `tasks/todo.md`; do not push without explicit user authorization.**
