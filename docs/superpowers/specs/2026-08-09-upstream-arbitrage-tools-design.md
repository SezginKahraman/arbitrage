# Upstream Arbitrage Tools Installation Design

## Objective

Install three independent upstream Git repositories directly under
`/Users/sezginkahraman/repos/arbitrage`, verify that each project can run in
the local environment, and validate the existing Binance Global and Gate.io
API credentials with read-only requests.

## Repository Layout

The resulting workspace will have this structure:

```text
arbitrage/
├── .env
├── .gitignore
├── crypto-futures-arbitrage-scanner/
├── hummingbot/
├── ccxt/
├── scripts/
│   └── verify-api-keys.mjs
└── docs/
    └── superpowers/
        ├── plans/
        └── specs/
```

Each of the three upstream project directories will retain its own `.git`
directory and remote origin:

- `crypto-futures-arbitrage-scanner` from
  `https://github.com/jose-donato/crypto-futures-arbitrage-scanner.git`
- `hummingbot` from `https://github.com/hummingbot/hummingbot.git`
- `ccxt` from `https://github.com/ccxt/ccxt.git`

The wrapper workspace will not modify the upstream projects' application
code. Local integration scripts and documentation will remain at the
`arbitrage` root.

## Installation Strategy

### Crypto Futures Arbitrage Scanner

The host does not currently have Go installed. The scanner will therefore be
built and run with an official Go Docker image, keeping its dependencies out
of the host environment. Verification will confirm that the Go application
starts and its local HTTP endpoint responds.

### Hummingbot

Hummingbot will use its official Docker-based installation path. Setup will
remain non-trading: no exchange connector credentials will be added to
Hummingbot and no strategy will be started. Verification will establish that
the selected image can be built or pulled and that the service reaches its
expected startup state without placing orders. The upstream Compose file uses
the mutable `hummingbot/hummingbot:latest` tag, so the running image's resolved
repository digest is recorded as point-in-time evidence rather than presented
as a deployment pin.

### CCXT

The full CCXT repository will be cloned as requested. Its Node.js dependencies
will be installed inside `ccxt/` using the existing Node.js 22 and npm 10 host
runtime. CCXT will provide the normalized client used by the credential smoke
test. The verified checkout is CCXT 4.5.71 at commit
`4f3ac786d7fde38d888663f7cdd6190379ec9130`.

## Credential Verification

The existing root `.env` contains these variable names:

```text
BINANCE_API_KEY
BINANCE_API_SECRET
GATEIO_API_KEY
GATEIO_API_SECRET
```

The verification script will load them through Node.js's `--env-file` support
without sourcing the file as shell code. It instantiates CCXT 4.5.71's
`ccxt.binance` constructor and its `ccxt.gate` constructor. The latter retains
the stable user-facing `gateio` label and the existing `GATEIO_*` environment
variable names.

The production constructors enforce these route-scope options:

- Binance: `defaultType: "spot"`, `fetchCurrencies: false`,
  `fetchMargins: false`, and `fetchMarkets: { types: ["spot"] }`.
- Gate: `defaultType: "spot"`, `unifiedAccount: false`, and
  `fetchMarkets: { types: ["spot"] }`.

Offline tests use real clients created by this production configuration and
intercept CCXT's implicit request boundary before any network transport. They
fail closed on every route outside this exact sequence:

- Binance: public `publicGetExchangeInfo`, then private read-only spot account
  `privateGetAccount`.
- Gate: public `publicSpotGetCurrencies`,
  `publicMarginGetCurrencyPairs`, and `publicSpotGetCurrencyPairs`, then private
  read-only spot accounts `privateSpotGetAccounts`. Gate's public margin-pair
  request only annotates spot markets; it is not a private account or margin
  balance request.

The tests explicitly reject Binance SAPI wallet/currency and margin metadata,
derivative market families, Gate account-detail and unified-account reads,
private margin/funding balances, and public futures/delivery/options discovery.
After that offline boundary passes, the live verifier performs, for each
exchange:

1. A public market metadata request to establish connectivity.
2. A private, read-only balance request to validate authentication and
   request signing.

### Endpoint-scope correction (2026-08-09)

The original design treated the high-level calls `loadMarkets()` and
`fetchBalance({ type: "spot" })` as sufficient endpoint-boundary evidence. In
CCXT 4.5.71, the original credentialed Binance configuration also enabled
signed SAPI wallet/currency and margin discovery plus non-spot public market
metadata. The original Gate configuration left `unifiedAccount` undefined,
which triggered private account-detail discovery and could select the unified
balance route. Those additional calls were read-only and did not mutate funds,
but they were broader than the stated public-market plus spot-account boundary.
That first live run is retained as historical authentication evidence and is
superseded for endpoint-scope proof by the hardened constructors, exact-route
offline tests, and subsequent sanitized live rerun.

The script will output only the exchange name and pass/fail status. It will
not output API keys, secrets, balances, wallet addresses, order history, or
other account data. It will not call create-order, cancel-order, transfer,
withdrawal, margin, futures-position, or leverage endpoints.

Authentication failures will be reported with a sanitized error category,
such as invalid credentials, missing permissions, IP whitelist rejection,
clock skew, or regional/network restriction. Raw HTTP response bodies will
not be printed when they could contain sensitive account information.

## Secret Protection

Before dependency installation or execution, the root `.gitignore` will be
created with `.env` and runtime-data exclusions. The credential verifier will
read secrets only from process memory. Commands and logs will not interpolate
secret values into command-line arguments.

No API key will be copied into any upstream repository, Docker image, source
file, generated configuration, or committed document.

## Reproducibility Evidence

The installed snapshot resolves to these immutable identities:

- Crypto Futures Arbitrage Scanner:
  `c93a7e620be5ee2cd041dff32b740893aa7358fe`
- Hummingbot: `2bfaccc48dd49e71a5b6d9b3011808e127dd00cd`
- CCXT: `4f3ac786d7fde38d888663f7cdd6190379ec9130`
- Running Hummingbot image:
  `hummingbot/hummingbot@sha256:632d2b07aa156b761310f2f7258a78c9660a1c28b6df4b33874e09a0c7d06c85`

The three repositories remain shallow checkouts, so these HEAD values record
the resolved installation state without rewriting or switching upstream
history. Likewise, the Hummingbot repository digest identifies the artifact
observed in the running container; the Compose tag itself remains mutable.

## Verification

Completion requires fresh evidence for all of the following:

- All three directories are independent Git repositories with the expected
  `origin` URL.
- Scanner dependencies resolve, the application starts, and its health-facing
  HTTP page responds locally.
- Hummingbot's Docker installation completes its non-trading startup check.
- CCXT installs and imports successfully from Node.js.
- The real-client offline route suite fails closed on unexpected requests and
  proves the exact Binance and Gate sequences above before any live rerun.
- Binance Global public and private read-only requests each return success, or
  a sanitized actionable failure is recorded.
- Gate.io public and private read-only requests each return success, or a
  sanitized actionable failure is recorded.
- `.env` remains ignored and is not tracked by any repository.

The live output proves point-in-time connectivity and authentication only. The
pinned offline route test supplies endpoint-identity evidence; a source-token
scan or high-level method assertion alone does not.

## Operational Boundaries

This phase does not implement an arbitrage strategy, compare live spreads,
persist market data, execute paper trades, or place live orders. Its only
deliverables are reproducible local installations and safe API connectivity
verification. High-level CCXT method names and `type: "spot"` options are not,
by themselves, accepted as proof of the underlying transport boundary.
