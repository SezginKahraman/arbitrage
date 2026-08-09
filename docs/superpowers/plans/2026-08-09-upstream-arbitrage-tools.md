# Upstream Arbitrage Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Install the scanner, Hummingbot, and CCXT as three independent repositories under `arbitrage`, then safely verify Binance Global and Gate.io public and private read-only API access.

**Architecture:** Upstream sources remain isolated in three nested Git repositories and are ignored by the wrapper repository. The scanner and Hummingbot run through Docker, while CCXT uses the existing Node.js runtime and a root-level credential verifier that never prints secret or account data. CCXT client options constrain discovery to spot scope, and offline tests against real production-configured clients intercept the implicit request boundary before live verification.

**Tech Stack:** Git, Docker 28 / Docker Compose 2, Go 1.24 container, Node.js 22, npm 10, CCXT JavaScript 4.5.71 (`4f3ac786d7fde38d888663f7cdd6190379ec9130`)

## Global Constraints

- Clone all three repositories directly under `/Users/sezginkahraman/repos/arbitrage` with their own `.git` directories.
- Treat `BINANCE_API_KEY` and `BINANCE_API_SECRET` as Binance Global spot credentials.
- Make only public market and private read-only balance requests during credential verification.
- Never print, copy, commit, or pass API keys and secrets as command-line arguments.
- Never create or cancel orders, transfer funds, withdraw funds, change leverage, or open positions.
- Do not modify upstream application source code.

---

### Task 1: Protect Workspace Secrets and Runtime Artifacts

**Files:**
- Create: `.gitignore`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: Root `.env` containing the four approved credential variable names.
- Produces: Wrapper-level ignore rules that protect secrets and keep all three nested repositories independent.

- [x] **Step 1: Confirm the current unsafe state without reading secret values**

Run `git check-ignore -q .env`.

Expected: non-zero exit status because `.env` is not ignored yet.

- [x] **Step 2: Create exact ignore rules**

Create `.gitignore` with:

```gitignore
.env
*.log
.DS_Store
crypto-futures-arbitrage-scanner/
hummingbot/
ccxt/
```

- [x] **Step 3: Verify secret and nested-repository paths are ignored**

```bash
git check-ignore -v .env
git check-ignore -v crypto-futures-arbitrage-scanner/
git ls-files --error-unmatch arbitrage/.env
```

Expected: the first two commands identify `.gitignore`; the last command fails because `.env` is untracked.

- [x] **Step 4: Commit wrapper safety configuration**

```bash
git add .gitignore tasks/todo.md
git commit -m "chore: protect arbitrage workspace secrets"
```

### Task 2: Clone and Validate the Three Upstream Repositories

**Files:**
- Create: `crypto-futures-arbitrage-scanner/` via Git clone
- Create: `hummingbot/` via Git clone
- Create: `ccxt/` via Git clone

**Interfaces:**
- Consumes: Official upstream HTTPS URLs from the approved design.
- Produces: Three independent shallow Git checkouts with verified origins.

- [x] **Step 1: Clone the scanner**

```bash
git clone --depth 1 https://github.com/jose-donato/crypto-futures-arbitrage-scanner.git crypto-futures-arbitrage-scanner
```

- [x] **Step 2: Clone Hummingbot**

```bash
git clone --depth 1 https://github.com/hummingbot/hummingbot.git hummingbot
```

- [x] **Step 3: Clone CCXT**

```bash
git clone --depth 1 https://github.com/ccxt/ccxt.git ccxt
```

- [x] **Step 4: Verify independent repositories and exact origins**

Run:

```bash
git -C crypto-futures-arbitrage-scanner rev-parse --is-inside-work-tree
git -C crypto-futures-arbitrage-scanner remote get-url origin
git -C hummingbot rev-parse --is-inside-work-tree
git -C hummingbot remote get-url origin
git -C ccxt rev-parse --is-inside-work-tree
git -C ccxt remote get-url origin
git -C crypto-futures-arbitrage-scanner rev-parse HEAD
git -C hummingbot rev-parse HEAD
git -C ccxt rev-parse HEAD
```

Expected origins:

```text
https://github.com/jose-donato/crypto-futures-arbitrage-scanner.git
https://github.com/hummingbot/hummingbot.git
https://github.com/ccxt/ccxt.git
```

Resolved installation snapshot:

```text
crypto-futures-arbitrage-scanner c93a7e620be5ee2cd041dff32b740893aa7358fe
hummingbot                       2bfaccc48dd49e71a5b6d9b3011808e127dd00cd
ccxt                             4f3ac786d7fde38d888663f7cdd6190379ec9130
```

These immutable HEAD values record the observed shallow checkouts; no upstream
history is rewritten and no version is switched.

### Task 3: Build and Smoke-Test the Arbitrage Scanner

**Files:**
- Use unchanged: `crypto-futures-arbitrage-scanner/go.mod`
- Use unchanged: `crypto-futures-arbitrage-scanner/main.go`

**Interfaces:**
- Consumes: Scanner source checkout and official `golang:1.24` Docker image.
- Produces: Successful Go build and an HTTP 200 response from the scanner UI.

- [x] **Step 1: Resolve modules and run repository tests inside Docker**

```bash
docker run --rm -v "$PWD/crypto-futures-arbitrage-scanner:/src" -w /src golang:1.24 sh -c 'go mod download && go test ./...'
```

Expected: exit status 0.

- [x] **Step 2: Build the scanner inside Docker**

```bash
docker run --rm -v "$PWD/crypto-futures-arbitrage-scanner:/src" -w /src golang:1.24 go build -o /tmp/arbitrage-scanner .
```

Expected: exit status 0.

- [x] **Step 3: Start an isolated smoke-test container**

```bash
docker run --rm -d --name arbitrage-scanner-smoke -e PORT=8082 -p 127.0.0.1:18082:8082 -v "$PWD/crypto-futures-arbitrage-scanner:/src" -w /src golang:1.24 go run .
```

- [x] **Step 4: Verify the scanner UI and stop only the smoke-test container**

```bash
curl --fail --silent --show-error http://127.0.0.1:18082/ >/dev/null
docker stop arbitrage-scanner-smoke
```

Expected: curl and container stop both exit 0.

### Task 4: Install and Validate Hummingbot with Docker

**Files:**
- Use unchanged: `hummingbot/docker-compose.yml`
- Generated by upstream setup: `hummingbot/.compose.env`

**Interfaces:**
- Consumes: Official Hummingbot Compose definition and `hummingbot/hummingbot:latest` image.
- Produces: A running non-trading Hummingbot container without Gateway.

- [x] **Step 1: Validate the official Compose definition**

```bash
docker compose -f hummingbot/docker-compose.yml config --quiet
```

Expected: exit status 0.

- [x] **Step 2: Configure the official setup without Gateway**

```bash
printf 'n\n' | make -C hummingbot setup
```

Expected: `hummingbot/.compose.env` contains `COMPOSE_PROFILES=` and no credential data.

- [x] **Step 3: Pull and deploy Hummingbot**

```bash
docker compose -f hummingbot/docker-compose.yml --env-file hummingbot/.compose.env pull hummingbot
docker compose -f hummingbot/docker-compose.yml --env-file hummingbot/.compose.env up -d hummingbot
```

- [x] **Step 4: Verify the non-trading container state**

```bash
docker inspect --format '{{.State.Running}}' hummingbot
docker compose -f hummingbot/docker-compose.yml ps hummingbot
docker inspect --format '{{.Image}}' hummingbot
```

Expected: inspect prints `true`, and Compose reports the service as running. Do not attach exchange credentials or start a strategy.

Inspect the exact image ID returned by the container and record its
`.RepoDigests` value. The observed artifact is:

```text
hummingbot/hummingbot@sha256:632d2b07aa156b761310f2f7258a78c9660a1c28b6df4b33874e09a0c7d06c85
```

The upstream Compose file still uses `hummingbot/hummingbot:latest`; this
digest is point-in-time reproducibility evidence, not a change to or pin of the
upstream definition.

### Task 5: Install CCXT and Create the Safe Credential Verifier

**Files:**
- Use unchanged: `ccxt/package.json`
- Create: `scripts/verify-api-keys.mjs`
- Create: `scripts/test-verify-api-keys.mjs`
- Create during endpoint hardening: `scripts/test-verify-api-routes.mjs`

**Interfaces:**
- Consumes: CCXT default export from `../ccxt/js/ccxt.js` and four root environment variables.
- Produces: `classifyError(error)`, `validateEnvironment()`,
  `createExchanges(environment)`, `checkExchange(name, exchange)`, exact-route
  offline coverage, and CLI pass/fail output containing no account data.

- [x] **Step 1: Install CCXT's Node.js dependencies**

```bash
npm --prefix ccxt install
```

Expected: exit status 0 and `ccxt/js/ccxt.js` remains importable.

- [x] **Step 2: Write classifier tests**

Create `scripts/test-verify-api-keys.mjs` that imports `classifyError` and asserts
these mappings with `node:assert/strict`. These are the historical baseline
unit tests; Task 7 adds the real-client endpoint-route regression suite.

```javascript
assert.equal(classifyError({ name: "AuthenticationError", message: "bad key" }), "AUTHENTICATION_FAILED");
assert.equal(classifyError({ name: "PermissionDenied", message: "denied" }), "PERMISSION_DENIED");
assert.equal(classifyError({ name: "ExchangeError", message: "IP whitelist" }), "IP_RESTRICTED");
assert.equal(classifyError({ name: "InvalidNonce", message: "timestamp" }), "CLOCK_SKEW");
assert.equal(classifyError({ name: "NetworkError", message: "timeout" }), "NETWORK_ERROR");
assert.equal(classifyError({ name: "Unexpected", message: "opaque" }), "EXCHANGE_ERROR");
```

- [x] **Step 3: Run the classifier test to verify it initially fails**

```bash
node scripts/test-verify-api-keys.mjs
```

Expected: failure because `scripts/verify-api-keys.mjs` does not exist.

- [x] **Step 4: Implement the verifier**

Create `scripts/verify-api-keys.mjs` with:

```javascript
import ccxt from "../ccxt/js/ccxt.js";

export function classifyError(error) {
  const name = String(error?.name ?? "");
  const message = String(error?.message ?? "").toLowerCase();
  if (message.includes("ip") && (message.includes("whitelist") || message.includes("restricted"))) return "IP_RESTRICTED";
  if (name === "InvalidNonce" || message.includes("timestamp") || message.includes("recvwindow")) return "CLOCK_SKEW";
  if (name === "AuthenticationError") return "AUTHENTICATION_FAILED";
  if (name === "PermissionDenied") return "PERMISSION_DENIED";
  if (name.includes("Network") || name === "RequestTimeout") return "NETWORK_ERROR";
  return "EXCHANGE_ERROR";
}

export function validateEnvironment(environment = process.env) {
  const required = ["BINANCE_API_KEY", "BINANCE_API_SECRET", "GATEIO_API_KEY", "GATEIO_API_SECRET"];
  return required.filter((name) => !environment[name]);
}

export function createExchanges(environment = process.env) {
  return [
    ["binance", new ccxt.binance({
      apiKey: environment.BINANCE_API_KEY,
      secret: environment.BINANCE_API_SECRET,
      enableRateLimit: true,
      options: {
        defaultType: "spot",
        fetchCurrencies: false,
        fetchMargins: false,
        fetchMarkets: { types: ["spot"] },
      },
    })],
    ["gateio", new ccxt.gate({
      apiKey: environment.GATEIO_API_KEY,
      secret: environment.GATEIO_API_SECRET,
      enableRateLimit: true,
      options: {
        defaultType: "spot",
        unifiedAccount: false,
        fetchMarkets: { types: ["spot"] },
      },
    })],
  ];
}

export async function checkExchange(name, exchange) {
  try {
    await exchange.loadMarkets();
    console.log(`[${name}] public: PASS`);
  } catch (error) {
    console.log(`[${name}] public: FAIL (${classifyError(error)})`);
    return false;
  }
  try {
    await exchange.fetchBalance({ type: "spot" });
    console.log(`[${name}] private-read: PASS`);
    return true;
  } catch (error) {
    console.log(`[${name}] private-read: FAIL (${classifyError(error)})`);
    return false;
  } finally {
    await exchange.close?.();
  }
}

async function main() {
  const missing = validateEnvironment();
  if (missing.length > 0) {
    console.error(`Missing required variables: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }
  const exchanges = createExchanges();
  const results = [];
  for (const [name, exchange] of exchanges) results.push(await checkExchange(name, exchange));
  if (results.some((result) => !result)) process.exitCode = 1;
}

if (import.meta.url === `file://${process.argv[1]}`) await main();
```

The implemented source additionally contains the reviewed cleanup-error
containment, sanitized top-level fallback, robust `pathToFileURL`
direct-execution guard, and Task 7 exact-route hardening. The final constructor
is `ccxt.gate`; `gateio` remains only the user-facing label.

- [x] **Step 5: Run the classifier and import smoke tests**

```bash
node scripts/test-verify-api-keys.mjs
node --test scripts/test-verify-api-routes.mjs
node -e "import('./ccxt/js/ccxt.js').then(m => { if (!m.default.binance || !m.default.gate) process.exit(1) })"
```

Expected: all commands exit 0.

- [x] **Step 6: Commit the verifier and tests**

```bash
git add scripts/verify-api-keys.mjs scripts/test-verify-api-keys.mjs tasks/todo.md
git commit -m "feat: add read-only exchange credential verifier"
```

### Task 6: Run Credential Checks and Final Verification

**Files:**
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: Root `.env`, installed CCXT checkout, and the credential verifier.
- Produces: Sanitized pass/fail evidence for Binance Global and Gate.io.

- [x] **Step 1: Run the credential verifier with Node's safe env loader**

```bash
node --env-file=.env scripts/verify-api-keys.mjs
```

Expected success output:

```text
[binance] public: PASS
[binance] private-read: PASS
[gateio] public: PASS
[gateio] private-read: PASS
```

If authentication fails, preserve only the script's sanitized category and do not print raw response bodies.

- [x] **Step 2: Verify origins, service state, script tests, and secret protection together**

Run the origin checks from Task 2, scanner HTTP smoke test from Task 3, Hummingbot state checks from Task 4, verifier tests from Task 5, and `git check-ignore -v .env` in one fresh verification pass.

- [x] **Step 3: Record results in the task review**

Update `tasks/todo.md` with exact checked items, command outcomes, API pass/fail categories, and any operational limitation. Do not include balances, key fragments, secrets, raw exchange responses, or wallet data.

- [x] **Step 4: Commit the final review**

```bash
git add tasks/todo.md
git commit -m "docs: record arbitrage tools verification"
```

#### Task 6 Review

Historical Task 6 result (superseded for endpoint-scope proof): the live
credential verifier exited 0 with Binance Global public/private-read PASS and
Gate.io public/private-read PASS. Only the four sanitized category lines were
retained. They remain point-in-time authentication/read evidence, but did not
prove the stated transport boundary.

- Endpoint correction: under CCXT 4.5.71, the original credentialed Binance
  client invoked signed SAPI wallet/currency and margin metadata plus default
  non-spot public market discovery during `loadMarkets()`. The original Gate
  client left `unifiedAccount` undefined, which invoked private account-detail
  discovery and could choose the unified balance route. No mutating or
  trading-capable request was identified, but the run was broader than public
  spot-market metadata plus the intended private spot-account balance route.
  Task 7's hardened rerun supersedes it for endpoint-scope evidence.
- Secret protection: `.env` remained ignored and untracked; its contents were not inspected.
- Repository integrity: scanner, Hummingbot, and CCXT retained their exact approved origins, independent shallow worktrees, and clean tracked state.
- Verifier safety: both syntax checks, deterministic tests, CCXT Binance/Gate import smoke, and the forbidden trading-capable-call source-token scan passed. That scan did not cover CCXT's transitive route selection.
- Scanner: Go module resolution/tests and build passed using the Docker Desktop helper PATH and corrected `sh -c`; the upstream repository has no Go test files. Isolated readiness returned HTTP 200 on attempt 6 within the 180-second bound. Direct-ID cleanup left no exact-name smoke container or port 18082 listener.
- Hummingbot: Compose validation passed; only the `hummingbot` service was running, Gateway was absent, and connector/controller/script/strategy directories held scaffold files only. The service was left running and non-trading.
- Scope limitation: the original run intended public market loading and a private spot balance read, but CCXT added the read-only discovery calls above. It did not call or validate trading-capable endpoints, and no balances or raw responses were recorded.

### Task 7: Harden and Revalidate the Exact Endpoint Scope

**Files:**

- Modify: `scripts/verify-api-keys.mjs`
- Create: `scripts/test-verify-api-routes.mjs`
- Modify: `scripts/test-verify-api-keys.mjs` only if baseline coverage requires it
- Modify: `tasks/todo.md`
- Modify: this plan and the installation design

**Interfaces:**

- Consumes: production-configured CCXT 4.5.71 clients, dummy offline
  credentials for route tests, and the already authorized live verifier
  command.
- Produces: exact implicit-route evidence, corrected sanitized live evidence,
  and transparent supersession of the first run.

- [x] **Step 1: Extract the existing production client construction without changing behavior**

Export `createExchanges(environment)` and make `main()` consume it. Run the
existing syntax, unit, and import checks before and after this refactor.

- [x] **Step 2: Add real-client offline route tests and capture RED**

Construct actual Binance and Gate clients through `createExchanges()` with
dummy credentials. Intercept their implicit `request(path, api, method)`
boundary, fail closed on every unexpected request, and set known private
account-detail, unified, wallet, and margin metadata methods to fail if
selected.

The uncorrected configuration failed 0/2 as expected:

- Binance selected `sapiGetCapitalConfigGetall` and
  `sapiGetMarginAllPairs` instead of the accepted public-spot/private-spot
  sequence.
- Gate selected `privateAccountGetDetail` plus futures, delivery, and options
  market discovery instead of spot-only loading and spot accounts.

- [x] **Step 3: Apply the minimal route configuration**

Use the exact constructor options documented in the design: disable Binance
private currency/margin discovery, set both market type lists to `spot`, and
set Gate `unifiedAccount` to `false`.

- [x] **Step 4: Capture GREEN and run the full offline safety suite**

```bash
node --check scripts/verify-api-keys.mjs
node --check scripts/test-verify-api-keys.mjs
node --check scripts/test-verify-api-routes.mjs
node scripts/test-verify-api-keys.mjs
node --test scripts/test-verify-api-routes.mjs
node -e "import('./ccxt/js/ccxt.js').then(m => { if (m.default.version !== '4.5.71' || !m.default.binance || !m.default.gate) process.exit(1) })"
```

Expected route sequences:

```text
binance GET public exchangeInfo -> GET private account
gateio  GET public.spot currencies -> GET public.margin currency_pairs -> GET public.spot currency_pairs -> GET private.spot accounts
```

- [x] **Step 5: Record immutable upstream and image evidence**

Record the three resolved nested repository HEADs and the running Hummingbot
image RepoDigest listed in Tasks 2 and 4. Preserve upstream histories and
versions unchanged.

- [x] **Step 6: Run the already authorized sanitized live verifier once**

Only after offline GREEN, run:

```bash
node --env-file=.env scripts/verify-api-keys.mjs
```

Corrected result: exit 0 with the four allowlisted Binance/Gate public and
private-read PASS lines. No raw response, balance, credential value, URL, or
header was retained.

- [x] **Step 7: Run final safety/runtime checks and document the correction**

Verify syntax/import/static forbidden-call checks, all upstream tracked states,
`.env` ignored/untracked state, Hummingbot running/non-trading state with
Gateway absent, and scanner smoke container/port absence. The scanner smoke
itself does not need rerunning because this wave changes only the verifier and
wrapper documentation.

#### Task 7 Review

- Offline exact-route tests passed 2/2 and prevented all external transport.
  Binance selected only `publicGetExchangeInfo` then `privateGetAccount`; Gate
  selected only its three public spot-market metadata routes then
  `privateSpotGetAccounts`.
- The corrected live rerun exited 0 with the same four sanitized PASS lines.
  The live lines prove point-in-time connectivity/authentication; endpoint
  identity comes from the real-client test against the resolved CCXT checkout.
- The first live PASS record remains in the chronology but is superseded for
  endpoint-scope proof. The correction narrowed additional read-only discovery
  routes; it did not remediate a funds mutation or trading request.
