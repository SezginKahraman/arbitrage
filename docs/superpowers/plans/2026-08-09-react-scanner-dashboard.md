# React Scanner Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a responsive React/Tailwind scanner dashboard backed by the existing Go WebSocket feed, persistent browser preferences, executable best-bid/best-ask opportunities, and bounded SQLite opportunity history.

**Architecture:** A Vite React application in `web/` consumes versioned market messages from the existing Go service and builds focused dashboard components. Go remains the market-data authority, gains normalized quote state, a non-blocking SQLite opportunity repository, and small read-only health/history APIs. Development uses a Vite proxy; production serves `web/dist` from Go.

**Tech Stack:** Go 1.23.5+, React with TypeScript, Vite, Tailwind CSS v4 with `@tailwindcss/vite`, Lightweight Charts, Lucide React, Vitest, Testing Library, SQLite through `modernc.org/sqlite`.

## Global Constraints

- Preserve all existing public market-data connectors and keep the scanner read-only.
- Never read or send exchange credentials from the dashboard.
- UI preferences use `arbitrage.ui.preferences.v1`; SQLite stores opportunities, not 200 ms raw price broadcasts.
- Buy prices use best ask, sell prices use best bid; unavailable fee, liquidity, and transfer checks remain explicitly `unknown`.
- The application remains a local, single-user dashboard with no authentication or cross-device preference sync.
- COTI prices retain eight-decimal formatting.
- Use TypeScript strict mode, visible keyboard focus, non-color state labels, responsive stacking, and reduced-motion support.

---

### Task 1: React, TypeScript, Vite, and Tailwind Foundation

**Files:**
- Create: `crypto-futures-arbitrage-scanner/web/package.json`
- Create: `crypto-futures-arbitrage-scanner/web/package-lock.json`
- Create: `crypto-futures-arbitrage-scanner/web/index.html`
- Create: `crypto-futures-arbitrage-scanner/web/tsconfig.json`
- Create: `crypto-futures-arbitrage-scanner/web/tsconfig.app.json`
- Create: `crypto-futures-arbitrage-scanner/web/vite.config.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/main.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/styles/index.css`
- Create: `crypto-futures-arbitrage-scanner/web/src/test/setup.ts`
- Modify: `crypto-futures-arbitrage-scanner/.gitignore`

**Interfaces:**
- Produces: `npm run dev`, `npm run test`, `npm run typecheck`, and `npm run build` commands.
- Produces: Tailwind design tokens `terminal-ink`, `terminal-panel`, `terminal-line`, `terminal-text`, `signal-mint`, and `signal-amber` as CSS theme variables.

- [ ] **Step 1: Scaffold the React TypeScript package and install pinned dependencies**

Use the React TypeScript Vite template semantics, Tailwind's first-party Vite plugin, and a lockfile. Configure `/ws` and `/api` proxies to `http://127.0.0.1:8082`.

- [ ] **Step 2: Add the failing foundation smoke test**

Create `src/app/App.test.tsx` expecting an `Arbitrage Scanner` heading and a `Live market connection` status region.

- [ ] **Step 3: Run the test and confirm RED**

Run: `npm --prefix crypto-futures-arbitrage-scanner/web test -- --run`

Expected: FAIL because `App` does not exist.

- [ ] **Step 4: Add the minimal typed application entry and Tailwind theme**

Create `src/app/App.tsx`, import the CSS and font packages from `main.tsx`, and render the tested semantic shell. Add base focus, scrollbar, selection, and reduced-motion rules.

- [ ] **Step 5: Verify GREEN and commit**

Run typecheck, tests, and production build. Commit the complete foundation as `feat: scaffold React scanner dashboard`.

### Task 2: Typed Market State, Preferences, and Formatting

**Files:**
- Create: `crypto-futures-arbitrage-scanner/web/src/app/types.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/lib/format.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/lib/format.test.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/lib/preferences.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/lib/preferences.test.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/lib/sources.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/hooks/usePreferences.ts`

**Interfaces:**
- Produces: `formatPrice(price: number): string` with eight decimals below 1.
- Produces: `UiPreferences` with `symbol`, `enabledSources`, `minSpread`, `sort`, `chartRange`, and `navigationCollapsed`.
- Produces: `loadPreferences(storage: Storage): UiPreferences` and `savePreferences(storage: Storage, preferences: UiPreferences): void`.
- Produces: source metadata keyed by normalized source name.

- [ ] **Step 1: Write failing formatter and preference migration tests**

Cover `0.01140723`, high-price adaptive precision, defaults, corrupt JSON, partial fields, and migration from the legacy `enabledSources` key.

- [ ] **Step 2: Run focused tests and confirm RED**

Run: `npm --prefix crypto-futures-arbitrage-scanner/web test -- --run src/lib/format.test.ts src/lib/preferences.test.ts`

- [ ] **Step 3: Implement the pure format and preference modules**

Validate every loaded field. Save the v1 object before removing the legacy source key. Keep default symbol `COTIUSDT` only when no valid stored symbol exists.

- [ ] **Step 4: Add the React preference hook**

Expose `[preferences, updatePreferences]`; merge functional updates and save after each change.

- [ ] **Step 5: Verify typecheck/tests and commit**

Commit as `feat: add typed scanner preferences`.

### Task 3: WebSocket Normalization and Live Dashboard State

**Files:**
- Create: `crypto-futures-arbitrage-scanner/web/src/lib/market-state.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/lib/market-state.test.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/hooks/useScannerSocket.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/hooks/useScannerSocket.test.tsx`

**Interfaces:**
- Consumes: current `prices`, `spreads`, and `arbitrage` WebSocket messages.
- Produces: `ScannerState` with connection status, symbol/source prices, spread maps, opportunities, source freshness, and `lastUpdatedAt`.
- Produces: `selectBestOpportunity(state, symbol, enabledSources)` returning an opportunity or `null`.

- [ ] **Step 1: Write failing reducer tests**

Test current payload parsing, malformed payload rejection, all-symbol opportunity retention, 15-second source staleness, and best-opportunity selection.

- [ ] **Step 2: Confirm reducer tests fail**

Run the focused Vitest file and require missing exports to fail.

- [ ] **Step 3: Implement pure message parsing and reduction**

Use discriminated TypeScript unions after runtime shape checks. Cap in-memory opportunities at 250 and never let malformed messages replace valid state.

- [ ] **Step 4: Write and implement the WebSocket hook test**

Use a controllable fake WebSocket to prove initial connect, message reduction, close state, bounded exponential reconnect, and timer cleanup on unmount.

- [ ] **Step 5: Verify and commit**

Commit as `feat: add typed scanner websocket state`.

### Task 4: Responsive Dashboard Components

**Files:**
- Create: `crypto-futures-arbitrage-scanner/web/src/components/layout/AppShell.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/layout/Sidebar.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/layout/TopBar.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/scanner/OpportunityRoute.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/scanner/MetricStrip.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/scanner/OpportunitiesTable.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/scanner/ExecutionChecks.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/settings/SettingsDrawer.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/shared/SourceMark.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/shared/StatusBadge.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/scanner/Dashboard.test.tsx`
- Modify: `crypto-futures-arbitrage-scanner/web/src/app/App.tsx`

**Interfaces:**
- Consumes: `ScannerState`, `UiPreferences`, and typed update callbacks.
- Produces: semantic desktop/mobile dashboard without dead navigation actions.

- [ ] **Step 1: Write failing dashboard behavior tests**

Assert the selected pair, buy/sell route, gross spread, unknown execution checks, source-online metric, opportunities filtering, settings controls, and empty/stale states.

- [ ] **Step 2: Confirm tests fail**

Run the focused component test and inspect failures for absent components.

- [ ] **Step 3: Build layout and shared components**

Implement the reference hierarchy with the route rail as the signature element. Use real buttons/labels, `aria-live` for connection state, keyboard focus, and tooltips/copy for unknown checks.

- [ ] **Step 4: Build settings and preference interactions**

Allow pair selection, source toggling, and minimum spread editing. Do not render non-functional Alerts, Exchanges, or Help navigation as active controls.

- [ ] **Step 5: Verify responsive/component tests and commit**

Commit as `feat: build scanner dashboard components`.

### Task 5: React Price Comparison Chart

**Files:**
- Create: `crypto-futures-arbitrage-scanner/web/src/components/scanner/PriceComparisonChart.tsx`
- Create: `crypto-futures-arbitrage-scanner/web/src/components/scanner/PriceComparisonChart.test.tsx`
- Modify: `crypto-futures-arbitrage-scanner/web/src/app/App.tsx`

**Interfaces:**
- Consumes: selected symbol, enabled sources, source metadata, and rolling price observations.
- Produces: a responsive Lightweight Charts panel with per-source precision and 15m/1h/4h display controls.

- [ ] **Step 1: Write failing chart adapter tests**

Mock Lightweight Charts and assert series lifecycle, eight-decimal COTI format, source visibility, range changes, resize handling, and cleanup.

- [ ] **Step 2: Confirm RED and implement the chart component**

Keep the imperative chart instance inside the component, update series without recreating the chart, and respect reduced motion.

- [ ] **Step 3: Verify tests/build and commit**

Commit as `feat: add React price comparison chart`.

### Task 6: Executable Quote and Opportunity Model in Go

**Files:**
- Modify: `crypto-futures-arbitrage-scanner/exchanges/types.go`
- Modify: `crypto-futures-arbitrage-scanner/main.go`
- Create: `crypto-futures-arbitrage-scanner/quotes.go`
- Create: `crypto-futures-arbitrage-scanner/quotes_test.go`

**Interfaces:**
- Produces: `Quote` containing source, symbol, best bid, best ask, optional bid/ask quantity, and timestamp.
- Produces: `FindBestOpportunity(symbol string, quotes map[string]Quote) (ArbitrageOpportunity, bool)`.
- Produces: versioned `quote_update` WebSocket messages while preserving legacy payloads during migration.

- [ ] **Step 1: Write failing executable-route tests**

Prove that the cheapest ask is the buy side, highest bid is the sell side, crossed same-source routes are rejected, stale/invalid quotes are excluded, and midpoint-only false positives disappear.

- [ ] **Step 2: Confirm Go tests fail**

Run: `go test ./... -run 'TestFindBestOpportunity|TestQuote' -v`.

- [ ] **Step 3: Implement quote state and calculation**

Keep channel processing non-blocking and copy state before route calculation. Preserve existing message consumers until the React dashboard is verified.

- [ ] **Step 4: Update React message reduction for `quote_update`**

Add failing then passing reducer tests showing bid/ask storage and executable route rendering.

- [ ] **Step 5: Verify Go/React suites and commit**

Commit as `feat: calculate executable arbitrage routes`.

### Task 7: SQLite Opportunity Sessions and Read APIs

**Files:**
- Create: `crypto-futures-arbitrage-scanner/storage/store.go`
- Create: `crypto-futures-arbitrage-scanner/storage/sqlite.go`
- Create: `crypto-futures-arbitrage-scanner/storage/sqlite_test.go`
- Create: `crypto-futures-arbitrage-scanner/api.go`
- Create: `crypto-futures-arbitrage-scanner/api_test.go`
- Modify: `crypto-futures-arbitrage-scanner/main.go`
- Modify: `crypto-futures-arbitrage-scanner/go.mod`
- Modify: `crypto-futures-arbitrage-scanner/go.sum`

**Interfaces:**
- Produces: `OpportunityStore` with `Observe`, `CloseStale`, `List`, `Prune`, and `Close` operations.
- Produces: `GET /api/opportunities` and `GET /api/health` JSON handlers.
- Consumes: qualifying executable opportunities from Task 6 through a bounded worker channel.

- [ ] **Step 1: Write failing migration and session tests**

Use a temporary SQLite file to prove schema creation, route-session updates, 15-second closure, startup closure of old sessions, seven-day pruning, filtering, ordering, and limits.

- [ ] **Step 2: Confirm RED and implement the repository**

Use `database/sql` with `modernc.org/sqlite`, prepared statements, a single writer, busy timeout, WAL mode, and context-aware shutdown.

- [ ] **Step 3: Write failing API tests and implement handlers**

Validate query parameters, cap `limit` at 500, return stable JSON envelopes, and report database degradation without exposing file paths or internals.

- [ ] **Step 4: Connect the bounded persistence worker**

Dropping a history observation under backpressure must log a rate-limited warning and must never block live quote processing.

- [ ] **Step 5: Verify Go tests/race checks and commit**

Commit as `feat: persist opportunity history in sqlite`.

### Task 8: Opportunity History in React

**Files:**
- Create: `crypto-futures-arbitrage-scanner/web/src/hooks/useOpportunityHistory.ts`
- Create: `crypto-futures-arbitrage-scanner/web/src/hooks/useOpportunityHistory.test.tsx`
- Modify: `crypto-futures-arbitrage-scanner/web/src/components/scanner/OpportunitiesTable.tsx`
- Modify: `crypto-futures-arbitrage-scanner/web/src/app/App.tsx`

**Interfaces:**
- Consumes: `GET /api/opportunities` and current preferences.
- Produces: merged recent/live opportunity rows with loading, stale, empty, and database-degraded states.

- [ ] **Step 1: Write failing history hook tests**

Cover successful fetch, query encoding, abort on preference change, malformed JSON, HTTP failure, retry, and live-row de-duplication.

- [ ] **Step 2: Implement the hook and table integration**

Keep live data visible if history fails and label historical sessions distinctly from active routes.

- [ ] **Step 3: Verify tests/typecheck/build and commit**

Commit as `feat: show persisted opportunity history`.

### Task 9: Production Serving, Migration, and End-to-End Verification

**Files:**
- Modify: `crypto-futures-arbitrage-scanner/main.go`
- Modify: `crypto-futures-arbitrage-scanner/ui_test.go`
- Create: `crypto-futures-arbitrage-scanner/Dockerfile`
- Create: `crypto-futures-arbitrage-scanner/.dockerignore`
- Modify: `crypto-futures-arbitrage-scanner/README.md`
- Remove after parity: `crypto-futures-arbitrage-scanner/static/index.html`
- Remove after parity: `crypto-futures-arbitrage-scanner/static/app.js`
- Remove after parity: `crypto-futures-arbitrage-scanner/static/price-format.js`
- Remove after parity: old static Node tests superseded by Vitest coverage

**Interfaces:**
- Produces: a multi-stage production image serving the React SPA, `/ws`, and `/api/*` on port 8082.

- [ ] **Step 1: Write failing Go serving tests**

Require the production index, hashed assets, SPA fallback, no-store for HTML/API, immutable caching for hashed assets, and unchanged `/ws` routing.

- [ ] **Step 2: Implement production asset serving and Docker build**

Build React in a Node stage, Go in a Go stage, and copy only the binary, dist assets, and writable data directory into the runtime image.

- [ ] **Step 3: Run complete verification**

Run React tests/typecheck/build, Go tests/vet/build/race, `git diff --check`, Docker build, HTTP/API smoke, and bounded WebSocket checks for COTI's five existing live sources.

- [ ] **Step 4: Perform visual and accessibility review**

Check desktop plus mobile screenshots, keyboard-only operation, focus order, reduced motion, overflow, low-price labels, stale states, and empty/error copy. Fix findings before removing the legacy UI.

- [ ] **Step 5: Replace the exact scanner container and verify runtime**

Stop/remove only the resolved `arbitrage-scanner` container, start the production image with a named SQLite volume, wait for HTTP 200, and verify Hummingbot remains untouched.

- [ ] **Step 6: Update documentation and commit**

Document development, production, database location/retention, public read-only scope, and UI behavior. Commit as `feat: ship React arbitrage dashboard`.

