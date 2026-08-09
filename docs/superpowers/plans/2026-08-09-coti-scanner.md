# COTI Scanner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `COTIUSDT` selectable in the scanner and stream its live prices from the existing exchanges that list it.

**Architecture:** Add a small source-aware symbol configuration in the Go backend so unsupported connectors keep their existing subscriptions while Binance spot/futures, Bybit futures, Gate futures, and Kraken futures also receive COTI. Extend Kraken's symbol normalization and the served static UI without changing the WebSocket payload format.

**Tech Stack:** Go 1.24, Gorilla WebSocket, `net/http`, vanilla HTML/JavaScript, Docker.

## Global Constraints

- The scanner remains public and read-only; do not use API credentials or private/trading endpoints.
- KuCoin connector implementation is outside this plan.
- Existing BTC, ETH, XRP, and SOL subscriptions must remain unchanged.
- Unsupported sources must not receive a COTI subscription.
- Use strict RED-GREEN TDD for every production behavior change.

---

### Task 1: Source-aware symbol configuration

**Files:**
- Create: `crypto-futures-arbitrage-scanner/symbols.go`
- Create: `crypto-futures-arbitrage-scanner/symbols_test.go`
- Modify: `crypto-futures-arbitrage-scanner/main.go:352-374`

**Interfaces:**
- Produces: `symbolsForSource(source string) []string`
- Produces: source constants used by `main` and the test table.
- Consumes: existing exchange connector functions accepting `[]string`.

- [x] **Step 1: Write the failing source-routing test**

Create a table-driven test that asserts exact literal symbol lists. Binance spot/futures, Bybit futures, Gate futures, and Kraken futures must receive `BTCUSDT`, `ETHUSDT`, `XRPUSDT`, `SOLUSDT`, and `COTIUSDT`. Bybit spot, Hyperliquid futures, OKX futures, Paradex futures, and Pyth must receive only the original four symbols. Mutating a returned slice must not alter later calls.

```go
package main

import (
	"reflect"
	"testing"
)

func TestSymbolsForSourceRoutesCOTIOnlyToSupportedSources(t *testing.T) {
	core := []string{"BTCUSDT", "ETHUSDT", "XRPUSDT", "SOLUSDT"}
	withCOTI := []string{"BTCUSDT", "ETHUSDT", "XRPUSDT", "SOLUSDT", "COTIUSDT"}
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{"Binance futures", sourceBinanceFutures, withCOTI},
		{"Bybit futures", sourceBybitFutures, withCOTI},
		{"Kraken futures", sourceKrakenFutures, withCOTI},
		{"Gate futures", sourceGateFutures, withCOTI},
		{"Binance spot", sourceBinanceSpot, withCOTI},
		{"Bybit spot", sourceBybitSpot, core},
		{"Hyperliquid futures", sourceHyperliquidFutures, core},
		{"OKX futures", sourceOKXFutures, core},
		{"Paradex futures", sourceParadexFutures, core},
		{"Pyth", sourcePyth, core},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := symbolsForSource(test.source); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("symbolsForSource(%q) = %v, want %v", test.source, got, test.want)
			}
		})
	}
}

func TestSymbolsForSourceReturnsIndependentSlices(t *testing.T) {
	first := symbolsForSource(sourceBinanceFutures)
	first[0] = "CHANGED"
	if got := symbolsForSource(sourceBinanceFutures)[0]; got != "BTCUSDT" {
		t.Fatalf("later call starts with %q, want BTCUSDT", got)
	}
}
```

- [x] **Step 2: Run the test and verify RED**

Run: `go test ./...`

Expected: FAIL because `symbolsForSource` and the source constants do not exist.

- [x] **Step 3: Implement the minimal source-aware configuration**

Create `symbols.go` in package `main` with the four core symbols, the COTI symbol, an explicit set of COTI-capable source names, and `symbolsForSource`. The function must allocate a fresh result slice. Update every connector call in `main.go` to request its own source-specific list.

```go
package main

const (
	sourceBinanceFutures     = "binance_futures"
	sourceBybitFutures       = "bybit_futures"
	sourceHyperliquidFutures = "hyperliquid_futures"
	sourceKrakenFutures      = "kraken_futures"
	sourceOKXFutures         = "okx_futures"
	sourceGateFutures        = "gate_futures"
	sourceParadexFutures     = "paradex_futures"
	sourceBinanceSpot        = "binance_spot"
	sourceBybitSpot          = "bybit_spot"
	sourcePyth               = "pyth"
)

var coreSymbols = []string{"BTCUSDT", "ETHUSDT", "XRPUSDT", "SOLUSDT"}

var cotiSources = map[string]struct{}{
	sourceBinanceFutures: {},
	sourceBybitFutures:   {},
	sourceKrakenFutures:  {},
	sourceGateFutures:    {},
	sourceBinanceSpot:    {},
}

func symbolsForSource(source string) []string {
	result := append([]string(nil), coreSymbols...)
	if _, ok := cotiSources[source]; ok {
		result = append(result, "COTIUSDT")
	}
	return result
}
```

Each connector call changes from the shared `symbols` variable to its matching
`symbolsForSource(source...)` result; remove the shared local variable.

- [x] **Step 4: Run the tests and verify GREEN**

Run: `gofmt -w symbols.go symbols_test.go main.go && go test ./...`

Expected: PASS with no source-routing failures.

- [x] **Step 5: Commit the task**

Commit only `symbols.go`, `symbols_test.go`, and `main.go` with message `feat: route COTI to supported scanner sources`.

---

### Task 2: Kraken COTI symbol normalization

**Files:**
- Create: `crypto-futures-arbitrage-scanner/exchanges/kraken_test.go`
- Modify: `crypto-futures-arbitrage-scanner/exchanges/kraken.go:194-224`

**Interfaces:**
- Consumes: `convertToKrakenSymbol(string) string`
- Consumes: `convertFromKrakenSymbol(string) string`
- Produces mapping: `COTIUSDT` ↔ `PF_COTIUSD`.

- [x] **Step 1: Write the failing round-trip tests**

Add literal assertions that `convertToKrakenSymbol("COTIUSDT")` returns `PF_COTIUSD` and `convertFromKrakenSymbol("PF_COTIUSD")` returns `COTIUSDT`. Retain one BTC case to protect the existing convention.

```go
package exchanges

import "testing"

func TestKrakenCOTISymbolConversion(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"COTI outbound", convertToKrakenSymbol("COTIUSDT"), "PF_COTIUSD"},
		{"COTI inbound", convertFromKrakenSymbol("PF_COTIUSD"), "COTIUSDT"},
		{"BTC outbound", convertToKrakenSymbol("BTCUSDT"), "PF_XBTUSD"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q, want %q", test.got, test.want)
			}
		})
	}
}
```

- [x] **Step 2: Run the focused test and verify RED**

Run: `go test ./exchanges -run TestKrakenCOTISymbolConversion -v`

Expected: FAIL because both COTI conversions currently return the input unchanged.

- [x] **Step 3: Add the two COTI switch cases**

Extend the existing conversion switches only; do not refactor unrelated Kraken order-book code.

```go
case "COTIUSDT":
	return "PF_COTIUSD"
```

and in the reverse switch:

```go
case "PF_COTIUSD":
	return "COTIUSDT"
```

- [x] **Step 4: Run focused and full tests**

Run: `gofmt -w exchanges/kraken.go exchanges/kraken_test.go && go test ./...`

Expected: PASS.

- [x] **Step 5: Commit the task**

Commit only the Kraken source and test with message `feat: normalize COTI for Kraken futures`.

---

### Task 3: Served UI symbol option

**Files:**
- Create: `crypto-futures-arbitrage-scanner/ui_test.go`
- Modify: `crypto-futures-arbitrage-scanner/static/index.html:596-600`

**Interfaces:**
- Consumes: the real static file server rooted at `./static`.
- Produces: a selectable `<option value="COTIUSDT">COTIUSDT</option>` in the served page.

- [x] **Step 1: Write the failing UI behavior test**

Use `httptest` with `http.FileServer(http.Dir("./static"))`, request `/`, assert HTTP 200, and parse the response body to verify an option element has both value and text `COTIUSDT`. Do not inspect the source file directly.

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServedUIOffersCOTI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	http.FileServer(http.Dir("./static")).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	const option = `<option value="COTIUSDT">COTIUSDT</option>`
	if !strings.Contains(response.Body.String(), option) {
		t.Fatalf("served UI does not contain %s", option)
	}
}
```

- [x] **Step 2: Run the focused test and verify RED**

Run: `go test . -run TestServedUIOffersCOTI -v`

Expected: FAIL because the served dropdown lacks COTI.

- [x] **Step 3: Add the COTI option**

Append the option to the existing symbol selector without changing its default BTC selection.

```html
<option value="COTIUSDT">COTIUSDT</option>
```

- [x] **Step 4: Run focused and full tests**

Run: `gofmt -w ui_test.go && go test ./...`

Expected: PASS.

- [x] **Step 5: Commit the task**

Commit only `ui_test.go` and `static/index.html` with message `feat: expose COTI in scanner UI`.

---

### Task 4: Build and live scanner verification

**Files:**
- Modify: `tasks/todo.md` in the wrapper repository for final evidence only.

**Interfaces:**
- Consumes: scanner HTTP endpoint `http://127.0.0.1:8082/`.
- Consumes: scanner WebSocket endpoint `ws://127.0.0.1:8082/ws`.

- [x] **Step 1: Run static verification**

Run inside the scanner repository: `go test ./... && go vet ./... && go build ./... && git diff --check`.

Expected: all commands exit 0.

- [x] **Step 2: Restart only the named scanner container**

Inspect the existing `arbitrage-scanner` container, stop that exact container, and recreate it from `golang:1.24` with the scanner repository bind-mounted read/write at `/src`, working directory `/src`, port `127.0.0.1:8082:8082`, and command `go run .`. Do not stop or alter Hummingbot.

- [x] **Step 3: Verify HTTP and live COTI WebSocket data**

Use a bounded readiness loop for HTTP 200. Then connect to `/ws` with a bounded client and require a `prices` message containing `COTIUSDT` with at least two finite, positive source prices. Record only source names and count, not credentials or balances.

- [x] **Step 4: Review logs and repository scope**

Confirm the scanner logs show no repeated invalid-symbol reconnect loop, the scanner nested repository contains only the planned changes, the wrapper contains only documentation/task changes, and Hummingbot remains running.

- [x] **Step 5: Record final evidence**

Update `tasks/todo.md` with test/build/HTTP/WebSocket results and any operational limitation discovered during the live check.
