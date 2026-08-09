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
