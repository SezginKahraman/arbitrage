# Arbitrage Tools Setup

## Plan

- [x] Protect `.env` and ignore the three independent upstream repositories.
- [x] Clone the scanner, Hummingbot, and CCXT under the workspace root.
- [x] Build, test, and smoke-test the scanner in Docker.
- [x] Install and start Hummingbot in Docker without Gateway or trading configuration.
- [x] Install CCXT dependencies and add a tested, read-only credential verifier.
  - [x] Install CCXT dependencies and verify the supported exchange exports.
  - [x] Add verifier behavior tests and capture the initial RED result.
  - [x] Implement the read-only verifier and capture the GREEN result.
  - [x] Run import/safety checks, self-review, and commit the wrapper changes.
  - [x] Fix round 1: contain rejecting exchange cleanup and verify loop continuity.
- [x] Verify Binance Global and Gate.io public/private read-only API access.
- [x] Run the complete verification pass and document results.
- [x] Final review fix wave: harden and re-verify the exact read-only endpoint boundary.
  - [x] Extract the existing production client construction without changing behavior and keep the baseline suite green.
  - [x] Add offline route tests against real CCXT 4.5.71 clients and capture the expected unauthorized-route/config RED.
  - [x] Disable Binance private currency/margin metadata, disable Gate unified-account discovery, and constrain both clients to spot markets.
  - [x] Capture route-test GREEN plus the full verifier and safety verification suite.
  - [x] Correct the historical evidence, record immutable upstream/image identifiers, and rerun the authorized sanitized live verifier once.
  - [x] Commit the focused wrapper fixes and complete a final self-review.

## Review

Verified `.env` and each planned nested repository path are ignored by the wrapper-level `.gitignore`; `.env` remains untracked.

Cloned the three upstream projects as clean, independent shallow repositories and verified their exact approved `origin` URLs.

Scanner verification: Go module resolution/tests and build passed in `golang:1.24`; the UI returned HTTP 200; the isolated smoke container and port were clean afterward. The plan's login-shell flag was corrected because it removed the official image's Go path.

Hummingbot verification: official Compose configuration validated; the service is running with Gateway absent and only tracked configuration scaffolds. The mutable `hummingbot/hummingbot:latest` tag resolved in the running container to `hummingbot/hummingbot@sha256:632d2b07aa156b761310f2f7258a78c9660a1c28b6df4b33874e09a0c7d06c85`. Tracked upstream source remains clean; generated runtime files stay ignored.

CCXT verification: dependencies installed successfully and `ccxt/js/ccxt.js` imports cleanly. The baseline verifier tests cover category mapping, missing-variable detection, public/private call ordering, short-circuiting, cleanup, sanitized output, and direct execution without credentials. During Task 5, no live exchange request was made. Current CCXT 4.5.71 uses the `ccxt.gate` constructor while the verifier retains the user-facing `gateio` label and `GATEIO_*` variable names.

### Historical Task 6 evidence (superseded for endpoint-scope proof)

The first live credential verification ran only `node --env-file=.env scripts/verify-api-keys.mjs`. It exited 0 with these sanitized results:

- `[binance] public: PASS`
- `[binance] private-read: PASS`
- `[gateio] public: PASS`
- `[gateio] private-read: PASS`

Those lines remain point-in-time authentication/read evidence, but later CCXT
source and offline-route review showed that they did not prove the intended
transport boundary. The original credentialed Binance client also invoked
signed SAPI wallet/currency and margin metadata plus non-spot public market
discovery during `loadMarkets()`. The original Gate client invoked private
account-detail discovery and could select unified balances because
`unifiedAccount` was undefined. These additional calls were read-only; no
mutating or trading-capable request was identified. The run is therefore
retained in the chronology but superseded by the hardened evidence below.

The original final verification pass completed with these outcomes:

- Scanner, Hummingbot, and CCXT were independent shallow worktrees at the exact approved origins; all three tracked worktrees were clean.
- `.env` is ignored by `arbitrage/.gitignore` and is untracked. Its contents were not inspected.
- Verifier and test syntax checks, deterministic verifier tests, CCXT Binance/Gate import smoke, and the trading-capable-call source-token scan passed. That scan covered only explicit wrapper tokens and did not prove CCXT's transitive implicit routes.
- Scanner Go module resolution/tests and build passed with the Docker Desktop credential-helper PATH and corrected non-login `sh -c`; the repository currently reports no Go test files. The bounded readiness check returned HTTP 200 on attempt 6, within 180 seconds. The exact smoke container was stopped by its captured ID, and no residual exact-name container or listener on port 18082 remained.
- Hummingbot Compose validation passed. The `hummingbot` service remained running before and after the checks; Gateway was absent, and connector/controller/script/strategy configuration directories contained tracked scaffold files only.
- Final tracked-source checks passed for all three upstream repositories. No upstream source was modified.

### Final review fix wave

The production factory now constrains Binance with
`fetchCurrencies: false`, `fetchMargins: false`, and spot-only
`fetchMarkets`; it constrains Gate with `unifiedAccount: false` and spot-only
`fetchMarkets`. Gate remains constructed as `ccxt.gate` and labeled `gateio`.

Strict TDD evidence against real production-configured CCXT 4.5.71 clients:

- RED: 0/2 route tests passed. Binance selected
  `sapiGetCapitalConfigGetall` and `sapiGetMarginAllPairs`; Gate selected
  `privateAccountGetDetail` plus futures, delivery, and options market routes.
- GREEN: 2/2 route tests passed with the network request boundary fully
  intercepted. Binance selected only `GET public exchangeInfo` then
  `GET private account`. Gate selected only `GET public.spot currencies`,
  `GET public.margin currency_pairs`, `GET public.spot currency_pairs`, then
  `GET private.spot accounts`. The public Gate margin-pair request only
  annotates spot markets.

After offline GREEN, the single authorized corrected live command
`node --env-file=.env scripts/verify-api-keys.mjs` exited 0 with exactly the
same four sanitized PASS lines shown above. No raw error, response, balance,
credential value, signed URL, or header was retained. The live lines establish
point-in-time connectivity/authentication; the exact endpoint identities come
from the fail-closed offline test against the resolved CCXT checkout.

Resolved installation evidence:

- Scanner HEAD: `c93a7e620be5ee2cd041dff32b740893aa7358fe`
- Hummingbot HEAD: `2bfaccc48dd49e71a5b6d9b3011808e127dd00cd`
- CCXT HEAD: `4f3ac786d7fde38d888663f7cdd6190379ec9130`
- Running Hummingbot RepoDigest: `hummingbot/hummingbot@sha256:632d2b07aa156b761310f2f7258a78c9660a1c28b6df4b33874e09a0c7d06c85`

Operational limitations: the accepted evidence combines a pinned offline
route proof with a point-in-time live authentication check; it does not test or
authorize trading endpoints. Balances and raw exchange responses were neither
printed nor recorded. The scanner remains an isolated transient smoke result,
and Hummingbot remains deliberately running without trading configuration.

## COTI Scanner Integration

- [x] Add source-aware COTI symbol routing with RED-GREEN tests.
- [x] Add Kraken COTI normalization with RED-GREEN tests.
- [x] Add COTI to the served UI with a RED-GREEN test.
- [x] Run full tests, vet, build, Docker HTTP smoke, and live COTI WebSocket verification.
- [x] Record final verification evidence and review the scoped changes.

### COTI Review

- An earlier KuCoin check passed public access while the exact private
  read-only spot account endpoint returned `400003`. After the user updated the
  credentials, the Global public and private read-only calls both passed; the
  same key returned `400003` only on the EU domain. No balance, credential,
  signed URL, header, or raw exchange response was printed.
- Strict RED-GREEN cycles covered source-aware subscriptions, Kraken's
  `COTIUSDT` ↔ `PF_COTIUSD` normalization, and the served UI option.
- In the `golang:1.24` scanner container, `go test ./...`, `go vet ./...`, and
  `go build ./...` exited 0; `git diff --check` also passed.
- The exact prior `arbitrage-scanner` container was replaced with the same
  source bind mount and `127.0.0.1:8082` port. HTTP readiness returned 200.
- A bounded live WebSocket check received finite, positive COTI prices from
  five sources: Binance futures, Binance spot, Bybit futures, Gate.io futures,
  and Kraken futures.
- Scanner startup logs showed all existing connectors establishing their
  connections and no invalid-symbol reconnect loop. Hummingbot remained
  running and was not modified.

## Low-Price Precision

- [x] Add an eight-decimal low-price formatter with RED-GREEN Node tests.
- [x] Apply the formatter to chart series and existing price text surfaces.
- [x] Run Node/Go verification and confirm live COTI UI data remains healthy.

### Precision Review

- RED proved the old app rendered `0.01140723` as `0.011407` and did not
  configure the chart series. GREEN covered five Node tests for formatting,
  chart precision, boundary behavior, and per-band option caching.
- Prices below 1 now use eight decimals and chart `minMove: 0.00000001`; prices
  at or above 1 retain the previous adaptive precision bands.
- The full Go test suite, vet, build, and diff checks passed. The formatter was
  served successfully over HTTP and the live COTI WebSocket check still
  received five positive sources.
- The browser was reopened with a cache-busting page query at
  `http://localhost:8082/?precision=8`.

## Price Chart Cache Fix

- [x] Reproduce the stale unversioned UI asset path.
- [x] Version the price formatter and application script URLs.
- [x] Disable browser caching for scanner static responses.
- [x] Restart the scanner and verify HTTP headers plus live COTI sources.

### Cache Fix Review

- The live server now returns `Cache-Control: no-store`, and the served page
  loads `price-format.js?v=8-decimal` before `app.js?v=8-decimal`.
- Go tests, vet, build, Node formatter tests, and diff checks pass.
- After the container restart, the COTI stream again supplied Binance spot,
  Binance futures, Bybit futures, Gate.io futures, and Kraken futures prices.

## GitHub Publication

- [x] Initialize `arbitrage` as an independent Git repository on `main`.
- [x] Import the tracked scanner, Hummingbot, and CCXT source trees as real
  monorepo files while excluding nested Git metadata and generated files.
- [x] Verify root `.env` is ignored and no configured credential value appears
  in the staged tree.
- [x] Create the requested `first commit` and push `main` to
  `https://github.com/SezginKahraman/arbitrage.git`.
- [x] Verify the remote SHA, representative files from all three projects, and
  absence of the root `.env` through the GitHub API.

### Publication Review

- The initial commit contains 11,975 files and no tracked file is 50 MB or
  larger. Local Git object storage was approximately 175 MB before push.
- GitHub reports the repository as public with `main` as its default branch.
- Remote file checks passed for the root README, customized scanner app,
  Hummingbot README, and CCXT README.
- The configured Binance, Gate.io, and KuCoin key, secret, and passphrase
  values were not found in the committed content; root `.env` is absent from
  GitHub.

## React Scanner Dashboard

- [x] Replace the legacy static UI with a typed React 19, Vite, and Tailwind 4 application.
- [x] Persist pair, source, threshold, chart-range, and navigation preferences in browser storage.
- [x] Normalize live WebSocket state and render eight-decimal low-price charts.
- [x] Calculate executable cross-source routes from best ask to best bid.
- [x] Store bounded opportunity sessions in SQLite and expose read-only history/health APIs.
- [x] Merge live and historical opportunities without hiding live data when history is unavailable.
- [x] Ship a multi-stage production image and replace only the scanner container on port 8082.
- [x] Run component, type, build, race, vet, Docker, HTTP, API, and live COTI WebSocket checks.
- [ ] Complete a browser screenshot/pixel review when an automation browser is available.

### React Dashboard Review

- React verification passed with 9 test files and 40 tests, strict TypeScript
  typecheck, and a Vite production build.
- Go verification passed with `go test -race ./...`, `go vet ./...`, and
  `go build ./...`. SPA tests cover production index serving, client-route
  fallback, immutable hashed assets, and missing-file behavior.
- The production Docker image built successfully and the exact prior scanner
  container was replaced by `arbitrage-scanner:local` using the named
  `arbitrage-scanner-data` volume. `GET /`, `/api/health`, and filtered
  `/api/opportunities` returned successful responses; SQLite reported healthy
  and persisted a real COTI opportunity session.
- A bounded live WebSocket check received versioned best-bid/best-ask COTI
  quotes from Binance futures, Binance spot, Bybit futures, Gate.io futures,
  and Kraken futures. Hummingbot remained running and unchanged.
- Final review hardening moved every WebSocket client to a bounded non-blocking
  queue, made scanner health require two fresh executable books for one symbol,
  preserved first/peak/latest observations during SQLite coalescing, and passed
  one ten-second shutdown context through the full history drain. Five-second
  chart buckets cap four-hour history at about 2,881 points per source while
  preserving the history reference between buckets. The focused race tests
  passed 100 consecutive runs, and the independent final review returned PASS
  with no Critical or Important findings.
- The in-app browser runtime reported no available browser, so a screenshot and
  pixel-level desktop/mobile review could not be automated in this session.
  Semantic component tests and responsive/focus source review passed; the live
  dashboard remains available at `http://127.0.0.1:8082` for manual inspection.

## Market Comparison Controls

- [x] Add mutually exclusive Spot, Futures, and Spot ↔ Futures route modes.
- [x] Add Gate.io Spot BBO data so COTI has a real spot-to-spot comparison.
- [x] Make market-source toggles filter the route, table, metrics, and chart together.
- [x] Add persistent opportunities collapse and split/stacked layout controls.
- [x] Make table sort direction visually explicit while preserving accessible sorting.
- [x] Run frontend/Go tests, rebuild the production scanner, and verify live COTI sources.

### Market Controls Review

- Spot, Futures, and Spot ↔ Futures are mutually exclusive persisted modes;
  source chips drive the hero route, metrics, opportunities table, and chart.
- Live Opportunities is persisted as collapsed/expanded, every column exposes
  accessible ascending/descending sorting, and split/stacked layout is persisted.
- Gate.io Spot now supplies public executable best-bid/best-ask data. A live
  WebSocket verification received all six COTI books: Binance futures/spot,
  Bybit futures, Gate.io futures/spot, and Kraken futures, plus the authoritative
  opportunity snapshot protocol.
- Route snapshots remove closed routes and seed reconnecting clients. A one-second
  serial revalidation loop handles silent feeds while a min-leg timestamp and
  expiry-aware refresh prevent stale or temporarily hidden executable routes.
- Historical sessions are selected latest-per-route before the SQL limit. Writes
  are coalesced for 250 ms and committed in one transaction; drain failures are
  surfaced from shutdown instead of being reported as success.
- React finished with 9 test files and 52 passing tests plus strict typecheck and
  production build. Go race tests, vet, build, and diff checks passed. Independent
  final review returned Ready with no Critical or Important findings.
- The production scanner image was rebuilt and only `arbitrage-scanner` was
  replaced on `127.0.0.1:8082`; Hummingbot retained the same container ID. HTTP,
  API, immutable assets, SQLite health, and live WebSocket checks passed.
- A prior 3.3 GB WAL caused a one-time slow replay. SQLite now sets a 64 MiB
  journal limit and explicitly validates the three-column TRUNCATE checkpoint
  result. The final restart was immediately healthy and the WAL remained below
  the configured limit.

## Binance Spot Freshness Fix

- [x] Add a regression test for Binance Spot `bookTicker` payloads without event time.
- [x] Fall back to the local receive timestamp for executable Binance Spot quotes.
- [x] Seed reconnecting clients with current valid quote snapshots.
- [x] Run full verification and confirm the live COTI Spot source count and route.

Review: Binance Spot `bookTicker` messages without an event-time field now use their local receive time, so executable quotes remain fresh. A reconnecting browser receives all current valid quote snapshots before opportunity snapshots; two consecutive live WebSocket connections received Binance Spot and Gate Spot COTI quotes within 46 ms and 9 ms, respectively, together with the Gate Spot → Binance Spot route. Go race/vet/build, 52 frontend tests, TypeScript checks, production build, HTTP 200, container identity, database size, and Hummingbot isolation were verified.

## KuCoin Market Integration

- [x] Add failing backend tests for KuCoin symbol routing and Spot/Futures BBO parsing.
- [x] Implement authenticated-free KuCoin public Spot/Futures WebSocket feeds with token refresh, heartbeat, and reconnect.
- [x] Add KuCoin Spot/Futures to every UI market selector, filter, chart, and persisted preference.
- [x] Run the full backend/frontend verification suite.
- [x] Deploy only the scanner and verify live KuCoin Spot/Futures COTI quotes and UI source counts.

Review: KuCoin is now a first-class Spot and Futures source for BTC, ETH, XRP, SOL, and COTI. Both feeds use KuCoin's classic public bullet-token WebSockets, wait for the welcome frame before subscribing, maintain application heartbeats, and reconnect with a fresh public token. Live verification after multiple heartbeat intervals returned current COTI quotes from both sources in 163 ms; the dashboard had 3 fresh Spot and 5 fresh Futures COTI sources. The initial live test exposed that KuCoin Spot's `time` field is the last-trade time rather than BBO receipt time, so Spot freshness now uses the local WebSocket receipt time under a red/green regression test. Go race/vet/build, 54 frontend tests, TypeScript, production build, HTTP 200, scanner restart count, SQLite WAL size, and Hummingbot isolation were verified.

## Opportunities And Alerts Pages

- [x] Add application navigation for Scanner, Opportunities, and Alerts without losing the live socket state.
- [x] Build the all-pair Opportunities page with live-only data, market/exchange/search/spread filters, sorting, and pagination.
- [x] Add SQLite alert-rule and trigger storage with validation, cooldown, and live opportunity evaluation.
- [x] Expose alert-rule and recent-trigger APIs and broadcast new triggers to connected clients.
- [x] Build the Alerts page with rule creation, enable/mute controls, browser delivery, and recent triggers.
- [x] Verify responsive/accessibility behavior, full Go/React suites, and production build.
- [x] Deploy only the scanner and verify all three pages plus a real alert trigger end to end.

### Opportunities And Alerts Review

- The shared App-level WebSocket remains mounted while the pathname switches
  between Scanner, Opportunities, and Alerts. Desktop sidebar navigation
  becomes a persistent mobile bottom bar on smaller screens.
- Opportunities renders every fresh route across all five tracked pairs and
  supports search, market type, exchange, minimum gross spread, accessible
  sorting, and pagination. Rows expire locally after the 15-second quote
  freshness window; the UI does not invent fee, liquidity, or transfer-ready
  claims.
- SQLite stores validated alert rules and immutable trigger snapshots. Rules
  match symbol, market type, buy/sell source, gross-spread threshold, enabled
  status, and transactional cooldown state. The API supports list/create/full
  update and recent-trigger reads; new triggers are also versioned WebSocket
  messages.
- Alerts can create and mute persisted rules, request browser permission from a
  user gesture, merge live and stored trigger history without duplicates, and
  labels delivery honestly as in-app/browser only. Email and webhook delivery
  are not presented as configured.
- React completed with 12 test files and 63 passing tests plus strict TypeScript
  and Vite production build. Go tests, race detector, vet, build, and diff
  checks passed.
- The production image was first exercised on an isolated port and temporary
  SQLite volume. A real COTI route created stored alert triggers end to end;
  the exact temporary container and volume were then removed. Only the scanner
  was replaced on `127.0.0.1:8082`, all three SPA routes returned HTTP 200, the
  database/scanner health was healthy, and live WebSocket data included KuCoin
  Spot/Futures plus opportunities for multiple symbols. Hummingbot retained
  its original running container ID.
- The in-app browser runtime had no available browser, so a new pixel-level
  screenshot review could not be captured. Component semantics, responsive
  source behavior, live HTTP, and WebSocket behavior were verified instead.

## Feed Health And Live Terminal

- [x] Add explicit exchange feed connection events and seed reconnecting dashboard clients with current states.
- [x] Separate connected-feed counts from fresh executable-book counts for the selected pair and market mode.
- [x] Capture bounded quote, connection, opportunity, and alert events in the client market state.
- [x] Build a persisted collapsible live-feed terminal with selected-pair filtering and clear controls.
- [x] Run frontend and Go RED-GREEN tests, race/vet/build checks, and production build.
- [x] Replace only the scanner container and verify connection events, terminal data, and Hummingbot isolation live.

Review: Every public exchange connector now reports explicit connected/disconnected state with its subscribed symbols. New browser sessions receive a deterministic 13-source status snapshot before live updates, while duplicate states are suppressed. The dashboard separates true feed connectivity from 15-second executable-book freshness, so quiet markets no longer make the connection count flicker. A bounded selected-pair terminal records connection, quote, reference-price, opportunity, and alert activity; it can be cleared or collapsed and its collapsed state persists. RED tests failed on the missing protocol/state/UI and all targeted tests passed after implementation. The final suite passed with 13 frontend files / 67 tests, TypeScript + Vite production build, Go tests, race detector, vet, build, and diff checks. A temporary `18082` smoke container returned HTTP 200 and streamed 13 source states plus COTI quotes before only `arbitrage-scanner` was replaced. The live scanner returned HTTP 200, reported 13/13 connected feeds, streamed COTI data, and remained at zero restarts; Hummingbot retained its exact running container ID. The temporary smoke and stopped rollback containers were removed; the persistent scanner data volume was preserved.

## Stable Terminal Retention

- [x] Reproduce the global 120-event eviction with a failing retention regression test.
- [x] Retain terminal activity for five minutes and sample high-frequency quote rows.
- [x] Show the retention window in the terminal UI and verify clear/collapse behavior.
- [x] Run the complete frontend suite/build and deploy only the scanner.
- [x] Verify live terminal activity, scanner stability, and Hummingbot isolation.

Review: The unstable disappear/reappear behavior came from one global 120-row cap shared by every symbol. Terminal events now expire by browser receipt time after five minutes, retain more than 120 in-window events, and use a 5,000-row defensive ceiling. High-frequency quote/reference streams add one row per source and symbol in each five-second bucket; intermediate ticks continue updating scanner prices without rebuilding the terminal list. The terminal is memoized and explicitly labels its five-minute buffer. Regression tests failed against the row cap, missing TTL, unsampled quotes, and missing label before implementation. The final frontend suite passed 70 tests across 13 files and the strict TypeScript/Vite production build passed. An isolated `18082` image check confirmed HTTP 200, the new asset label, and 13/13 source states before only the scanner was replaced. Live `8082` serves the new hashed asset, reports 13/13 connected sources with COTI quotes, and has zero restarts; Hummingbot retained its exact running container ID. Temporary smoke and rollback containers were removed while the scanner data volume was preserved.

## Quiet Spot Book Validation

- [x] Add RED tests for public Spot REST BBO normalization and quiet-book refresh.
- [x] Refresh Binance, Gate.io, KuCoin, and Bybit Spot BBOs every five seconds.
- [x] Distinguish insufficient fresh books from a below-threshold market in the route banner.
- [x] Explain the transfer verification warning with the exact missing checks.
- [x] Rate-limit browser quote delivery without slowing the server-side arbitrage calculation.
- [x] Run full Go/frontend verification and production build.
- [x] Deploy only the scanner and verify persistent COTI Spot coverage and Hummingbot isolation.

Review: Quiet event-driven Spot WebSockets were aging otherwise valid books out
after 15 seconds. Public, unauthenticated REST top-of-book validation now refreshes
Binance, Bybit, Gate.io, and KuCoin Spot quotes every five seconds while the
WebSockets remain the primary feed. Live smoke testing also exposed roughly 2,900
quote frames in under two seconds, which filled the per-client queue and caused
browser reconnects; UI quote delivery is now capped per source/pair at one frame
per second while every raw quote still reaches server-side opportunity evaluation.
The route banner separately reports missing fresh books versus a market below the
configured spread threshold. Transfer routes remain honestly unverified and now
name the missing deposit/withdraw, common-network, and transfer-fee checks.

The final frontend suite passed 72 tests and the production build; Go tests, race,
vet, and build passed. An isolated 35-second WebSocket check kept all Binance,
Gate.io, and KuCoin COTI Spot books fresh and retained a Spot route without a
disconnect. Production repeated the check for 25 seconds with quote ages of 5.4,
3.6, and 0.8 seconds, HTTP 200, zero scanner restarts, and the persistent SQLite
volume preserved. Only `arbitrage-scanner` was replaced; Hummingbot retained its
exact running container ID.

## Transfer Network Discovery

- [x] Test the Binance Global read-only account and capital-network endpoints without exposing credentials.
- [x] Add RED tests for Binance, Gate.io, and KuCoin network normalization and route evaluation.
- [x] Collect and cache tracked-asset network state without blocking market-data feeds.
- [x] Expose sanitized network snapshots through the scanner API.
- [x] Replace generic transfer warnings with directional READY / BLOCKED / CHECK / UNKNOWN details.
- [x] Run full verification and deploy only the scanner with Hummingbot isolation.

### Transfer Network Discovery Review

- Binance Global server time, private account read, and capital-network metadata
  returned HTTP 200 after the VPN was disabled; no credential, balance, signed
  URL, or raw private payload was printed.
- Binance, Gate.io, and KuCoin network metadata is refreshed in the background
  for BTC, ETH, XRP, SOL, and COTI. A failure at one venue degrades only that
  venue and does not block public market-data feeds.
- COTI live metadata showed Binance BSC and ERC20 open; Gate native COTI open
  with ERC20 withdrawal closed; and KuCoin native COTI open with ERC20
  withdrawal closed. Directional evaluation reported Gate/KuCoin native routes
  as CHECK, Gate/KuCoin to Binance as BLOCKED, and Binance to either venue over
  ERC20 as READY.
- Production live data showed COTI gross spreads around 15–16% from Gate/KuCoin
  to Binance, but both routes were correctly rejected as BLOCKED. No currently
  observed Spot route combined a positive scanner spread with READY network
  status; no trade was attempted.
- React passed 77 tests and the production build. Go 1.24 passed normal tests,
  race tests, vet, build, and diff checks. The isolated image smoke returned
  HTTP 200 with healthy database/scanner state and three ready network sources.
- Only `arbitrage-scanner` was replaced. Its SQLite volume was preserved, the
  root `.env` is mounted read-only, production health is healthy, and restart
  count is zero. Docker Desktop restart had stopped Hummingbot; the exact prior
  container ID was restored to running without configuration changes. Temporary
  smoke/rollback containers and the temporary smoke volume were removed.

## Dynamic Market Watchlist And Route-Aware Opportunities

- [x] Discover active USDT Spot/Futures markets and retain last-good source catalogs.
- [x] Persist a validated server-side watchlist with five defaults and a 20-coin limit.
- [x] Reconcile live source subscriptions when the watchlist changes without restarting the service.
- [x] Refresh network metadata for the active assets and expose batch transfer-route results.
- [x] Add Opportunities coin add/remove controls, coverage details, and route-status filters.
- [x] Feed the dynamic watchlist into Scanner and Alerts pair selectors.
- [x] Run backend/frontend tests, build the production image, and verify live add/remove behavior.

### Review

- Production market discovery returned 1,137 common USDT candidates from 12/12 ready source catalogs.
- Live A47 add/remove smoke proved that only relevant Gate/KuCoin subscriptions restart, removed quotes are purged, and the scanner container is not restarted.
- Post-deploy WebSocket verification reported 13/13 connected feeds and eight live COTI Spot/Futures quote sources.
- Batch transfer-route metadata returned 30 directional routes with non-null arrays: 26 ready, two check, and two blocked.
- Backend race tests, vet, build, frontend tests/typecheck/build, Docker build, and the production health endpoint passed.
- Hummingbot was not changed or restarted; its container identity remained unchanged.
