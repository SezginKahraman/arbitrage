# React Scanner Dashboard Design

## Goal

Replace the current dense, desktop-only scanner page with a clear single-user
dashboard built with React, TypeScript, Vite, and Tailwind CSS. Preserve the
existing public market-data scanner and WebSocket connections, persist browser
preferences across reloads, and add a small SQLite opportunity history without
pretending that unavailable execution-safety data is known.

The page's single job is to answer: **what is the best observable spread now,
how trustworthy is it, and what changed recently?**

## Selected Approach

Use an incremental frontend migration:

- Add a `web/` React application and build it into static assets served by Go.
- Keep the current Go exchange connectors and `/ws` transport.
- Introduce typed WebSocket messages and small React hooks around the existing
  feed instead of adding a large client state library.
- Store user-interface preferences in versioned `localStorage`.
- Store bounded opportunity history in SQLite through a small repository and
  expose it through read-only HTTP endpoints.
- Keep the scanner public and read-only; do not connect credentials or trading
  endpoints to this dashboard work.

This approach gives the UI clear component boundaries while limiting backend
risk. It also lets the existing scanner continue running during the migration.

## Alternatives Considered

1. **Incremental React migration — selected.** Best balance of maintainability,
   delivery speed, and preserving the proven Go market feeds.
2. **Keep vanilla HTML/CSS/JS.** Lowest dependency cost, but the existing
   685-line HTML and 929-line application class would remain difficult to
   reason about and test.
3. **Full-stack framework or complete backend rewrite.** Would provide routing
   and server rendering but adds deployment and integration complexity without
   helping a local real-time dashboard.

## Visual Direction

The supplied dark trading-terminal reference is the direction, but the page
will avoid decorative dashboard clutter. Its signature element is the
**execution route rail**: one horizontal card that reads from buy quote, through
gross/net spread, to sell quote, ending in an explicit route-safety state.

### Design tokens

- `terminal-ink` `#071015` — page canvas
- `terminal-panel` `#0C171D` — cards and navigation
- `terminal-line` `#1B2B33` — structural borders and chart grid
- `terminal-text` `#E7EEF2` — primary text
- `signal-mint` `#27E58C` — healthy/live/profitable states
- `signal-amber` `#F4B72A` — unverified or caution states

Use Space Grotesk for navigation and headings, IBM Plex Mono for prices,
percentages, and timestamps, and system sans-serif as a resilient fallback.
Color is always paired with text or an icon so state is not color-only.

### Desktop layout

```text
┌──────┬────────────────────────────────────────────────────────────┐
│ nav  │ title / pair search                         live / updated │
│      ├────────────────────────────────────────────────────────────┤
│      │ buy quote ── gross + net spread ── sell quote ── safety  │
│      ├────────────────────────────────────────────────────────────┤
│      │ best spread │ active │ sources online │ minimum spread    │
│      ├────────────────────────────┬───────────────────────────────┤
│      │ opportunities              │ price comparison              │
│      │ table                      ├───────────────────────────────┤
│      │                            │ execution checks              │
└──────┴────────────────────────────┴───────────────────────────────┘
```

At narrower widths the navigation collapses to a top bar, metric cards become
a horizontal scroll row, and the table and chart stack vertically. The table
retains horizontal scrolling rather than hiding critical price columns.

## Frontend Architecture

```text
web/src/
  app/
    App.tsx
    types.ts
  components/
    layout/AppShell.tsx
    layout/Sidebar.tsx
    layout/TopBar.tsx
    scanner/OpportunityRoute.tsx
    scanner/MetricStrip.tsx
    scanner/OpportunitiesTable.tsx
    scanner/PriceComparisonChart.tsx
    scanner/ExecutionChecks.tsx
    settings/SettingsDrawer.tsx
    shared/StatusBadge.tsx
    shared/SourceMark.tsx
  hooks/
    useScannerSocket.ts
    usePreferences.ts
    useOpportunityHistory.ts
  lib/
    format.ts
    sources.ts
  styles/
    index.css
```

Component responsibilities remain narrow:

- `useScannerSocket` owns connection lifecycle, message validation, reconnect
  state, and normalized in-memory market state.
- `usePreferences` owns a versioned preference object and migrations.
- `OpportunityRoute` renders only the best current route and its confidence.
- `OpportunitiesTable` filters and sorts current plus persisted opportunities.
- `PriceComparisonChart` adapts Lightweight Charts behind a React component.
- `ExecutionChecks` renders `verified`, `warning`, or `unknown`; it never
  converts missing data into a green check.

The first release remains a single dashboard. Sidebar entries without working
screens will not be presented as active navigation. Settings opens a drawer;
opportunity and exchange details can become later routes when they have real
content.

### Build and serving

During development, Vite runs the React app and proxies `/ws` and `/api` to the
Go scanner. The production build emits versioned assets to `web/dist`; Go
serves that directory with an SPA fallback while keeping `/ws` and `/api/*`
reserved for backend handlers. The scanner container becomes a multi-stage
Node-and-Go build so a clean checkout does not depend on committed build
artifacts.

## Preferences

Use one versioned local key, `arbitrage.ui.preferences.v1`, containing:

- selected symbol
- enabled sources
- minimum spread filter
- table sort field and direction
- selected chart time range
- collapsed navigation state

Preferences are parsed against defaults. Invalid or obsolete fields fall back
individually instead of discarding the entire object. SQLite is deliberately
not used for these single-browser preferences; doing so would add a write API
without providing a benefit in the approved local, single-user scope.
The first load imports the existing `enabledSources` value, if present, and
then removes that legacy key only after the new preference object is saved.

## SQLite Persistence

SQLite stores useful scanner history, not every 200 ms price broadcast.
High-frequency raw samples would grow rapidly and are outside this iteration.

Initial table:

```sql
opportunities (
  id INTEGER PRIMARY KEY,
  symbol TEXT NOT NULL,
  buy_source TEXT NOT NULL,
  sell_source TEXT NOT NULL,
  buy_price REAL NOT NULL,
  sell_price REAL NOT NULL,
  first_spread_pct REAL NOT NULL,
  latest_spread_pct REAL NOT NULL,
  peak_spread_pct REAL NOT NULL,
  started_at_ms INTEGER NOT NULL,
  last_seen_at_ms INTEGER NOT NULL,
  ended_at_ms INTEGER
)
```

The backend writes through one repository interface and uses batched or
rate-limited persistence so WebSocket processing is never blocked by disk I/O.
A route becomes one opportunity session: it is inserted when first detected,
updated while the same symbol/buy/sell route remains above the configured
scanner threshold, and closed after no qualifying update has been seen for 15
seconds. A process restart closes any previously open sessions before scanning
resumes.
A retention job removes records older than a configurable period, defaulting
to seven days.

Read API:

- `GET /api/opportunities?symbol=&minSpread=&limit=`
- `GET /api/health` for scanner, database, and source freshness status

No endpoint accepts credentials or submits orders.

## Market Data Truthfulness

The current scanner calculates opportunities from midpoint prices. The new UI
must distinguish data it can prove from data it cannot:

- Buy quote uses the lowest executable best ask.
- Sell quote uses the highest executable best bid.
- Gross spread is computed from those two sides.
- Net spread remains explicitly estimated until per-source fee configuration
  exists; the UI displays the assumptions beside it.
- Liquidity is `unknown` until bid/ask quantities or order-book depth are
  normalized across connectors.
- Deposit, withdrawal, and common-network state is `unknown` until a dedicated
  route-verification service exists.

The first UI release may show these checks, but unavailable checks use neutral
or amber states and explanatory copy, never fabricated green status.

## Data Flow

1. Exchange connectors publish normalized quotes to the Go scanner.
2. The scanner updates in-memory current state and detects a route.
3. A non-blocking persistence worker aggregates qualifying opportunity events
   into SQLite.
4. `/ws` pushes current prices, spread updates, opportunities, and health. A
   versioned `quote_update` message adds best bid/ask without removing the old
   messages until the React screen reaches parity.
5. React normalizes messages in `useScannerSocket` and renders focused
   components.
6. On load, the opportunities hook fetches recent history while live messages
   continue independently.

## Failure and Empty States

- WebSocket reconnects with bounded exponential backoff and a visible stale
  indicator.
- Each source displays freshness; old data cannot count as online.
- SQLite failure does not stop live scanning. Health reports degraded history
  and the UI keeps the live dashboard usable.
- Empty opportunity lists explain which threshold and source filters are
  active.
- Unsupported or missing checks say what data is needed to verify them.

## Testing and Acceptance

- Unit tests cover preference migration, price/spread formatting, best-route
  selection, source freshness, and unknown safety states.
- React component tests cover loading, live, stale, empty, and degraded states.
- Go tests cover SQLite migrations, repository retention, API filtering, and
  non-blocking opportunity persistence.
- An integration test connects to the real local `/ws` contract and renders a
  low-priced COTI route with eight decimals.
- Production build, Go tests/vet/build, responsive checks, keyboard navigation,
  visible focus, and reduced-motion behavior pass.
- Existing public connectors continue producing live COTI data after the new
  frontend is served.

## Delivery Slices

1. React/Tailwind shell and component boundaries with current WebSocket parity.
2. Dashboard hierarchy, responsive behavior, saved preferences, and chart.
3. SQLite opportunity history, API, health, and retention.
4. Executable bid/ask route calculation and honest estimated/unknown checks.

Each slice remains runnable and testable; the old UI is removed only after the
React dashboard reaches feature parity.

## Non-Goals

- Automated order submission, transfers, leverage, or credential use.
- Multi-user authentication or cross-device preference sync.
- Full-depth slippage modeling in the initial dashboard migration.
- Claiming a transfer route or net profit is safe without verified source data.
- Rewriting existing exchange connectors solely for frontend aesthetics.
