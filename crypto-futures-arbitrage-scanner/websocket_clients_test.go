package main

import (
	"reflect"
	"testing"
	"time"

	"futures-arbitrage-scanner/exchanges"
	"futures-arbitrage-scanner/storage"
)

func TestPublishedOpportunityEvaluatesAndBroadcastsAlertTriggers(t *testing.T) {
	now := time.UnixMilli(100_000)
	alerts := &fakeAlertStore{triggered: []storage.AlertTrigger{{
		ID: 7, RuleID: 3, RuleName: "COTI gap", Symbol: "COTIUSDT", TriggeredAtMS: now.UnixMilli(),
	}}}
	scanner := NewFuturesScanner()
	scanner.alerts = alerts
	client := &wsClient{send: make(chan any, 32)}
	scanner.wsClients[client] = struct{}{}
	scanner.quotes["COTIUSDT"] = map[string]Quote{
		"gate_spot":    {Symbol: "COTIUSDT", Source: "gate_spot", BestBid: 99, BestAsk: 100, Timestamp: now.UnixMilli()},
		"binance_spot": {Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 101, BestAsk: 102, Timestamp: now.UnixMilli()},
	}

	scanner.checkArbitrageAt("COTIUSDT", now)

	if len(alerts.evaluated) != 1 || alerts.evaluated[0].GrossSpreadPct != 1 {
		t.Fatalf("evaluated observations = %+v", alerts.evaluated)
	}
	found := false
	for len(client.send) > 0 {
		message, ok := (<-client.send).(map[string]any)
		if ok && message["type"] == "alert_trigger" && message["version"] == 1 {
			trigger := message["trigger"].(storage.AlertTrigger)
			found = trigger.ID == 7
		}
	}
	if !found {
		t.Fatal("alert trigger was not broadcast")
	}
}

func TestBroadcastQueuesMessagesAndDisconnectsSlowClientsWithoutBlocking(t *testing.T) {
	scanner := NewFuturesScanner()
	fast := &wsClient{send: make(chan any, 1)}
	slow := &wsClient{send: make(chan any, 1)}
	slow.send <- "already full"
	scanner.wsClients[fast] = struct{}{}
	scanner.wsClients[slow] = struct{}{}

	done := make(chan struct{})
	go func() {
		scanner.broadcastMessage(map[string]any{"type": "test"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("broadcast blocked on a slow client")
	}
	select {
	case <-fast.send:
	default:
		t.Fatal("fast client did not receive the message")
	}
	if _, exists := scanner.wsClients[slow]; exists {
		t.Fatal("slow client remained registered")
	}
}

func TestQuoteBroadcastsAreRateLimitedPerMarketSource(t *testing.T) {
	scanner := NewFuturesScanner()
	client := &wsClient{send: make(chan any, 8)}
	scanner.wsClients[client] = struct{}{}
	start := time.Unix(100, 0)
	quote := Quote{
		Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 0.01217, BestAsk: 0.01218,
		Timestamp: start.UnixMilli(),
	}

	scanner.broadcastQuoteAt(quote, start)
	quote.BestBid = 0.01218
	quote.BestAsk = 0.01219
	scanner.broadcastQuoteAt(quote, start.Add(500*time.Millisecond))
	otherSource := quote
	otherSource.Source = "gate_spot"
	scanner.broadcastQuoteAt(otherSource, start.Add(500*time.Millisecond))
	scanner.broadcastQuoteAt(quote, start.Add(time.Second))

	if got := len(client.send); got != 3 {
		t.Fatalf("queued quote messages = %d, want 3", got)
	}
}

func TestSourceConnectionStateIsBroadcastAndSeededForNewClients(t *testing.T) {
	scanner := NewFuturesScanner()
	status := exchanges.ConnectionStatus{
		Source: "gate_spot", Connected: true, Symbols: []string{"BTCUSDT", "COTIUSDT"}, Timestamp: 20_000,
	}
	scanner.updateConnectionStatus(status)

	client := scanner.registerClient(nil)
	foundSnapshot := false
	for len(client.send) > 0 {
		message, ok := (<-client.send).(sourceStatusMessage)
		if ok && message.Type == "source_status" && message.Version == 1 && reflect.DeepEqual(message.Status, status) {
			foundSnapshot = true
		}
	}
	if !foundSnapshot {
		t.Fatal("new client did not receive the current source connection state")
	}

	scanner.updateConnectionStatus(exchanges.ConnectionStatus{
		Source: "gate_spot", Connected: false, Symbols: status.Symbols, Timestamp: 25_000,
	})
	message, ok := (<-client.send).(sourceStatusMessage)
	if !ok || message.Status.Connected || message.Status.Timestamp != 25_000 {
		t.Fatalf("disconnect message = %+v", message)
	}
}

func TestDuplicateSourceConnectionStateDoesNotFloodClients(t *testing.T) {
	scanner := NewFuturesScanner()
	client := scanner.registerClient(nil)
	status := exchanges.ConnectionStatus{
		Source: "kucoin_futures", Connected: true, Symbols: []string{"COTIUSDT"}, Timestamp: 20_000,
	}
	scanner.updateConnectionStatus(status)
	if len(client.send) != 1 {
		t.Fatalf("first status queued messages = %d, want 1", len(client.send))
	}
	<-client.send
	status.Timestamp = 21_000
	scanner.updateConnectionStatus(status)
	if len(client.send) != 0 {
		t.Fatalf("duplicate status queued %d messages", len(client.send))
	}
}

func TestPriceUpdateCarriesTheSourceObservationTimestamp(t *testing.T) {
	scanner := NewFuturesScanner()
	client := &wsClient{send: make(chan any, 1)}
	scanner.wsClients[client] = struct{}{}
	scanner.updatePrice(exchanges.PriceData{
		Symbol: "COTIUSDT", Source: "pyth", Price: 0.0114, Timestamp: 7_000,
	})

	message := (<-client.send).(map[string]any)
	if message["type"] != "price_update" || message["version"] != 1 {
		t.Fatalf("message metadata = %+v", message)
	}
	price := message["price"].(MarketPrice)
	if price.Timestamp != 7_000 || price.Source != "pyth" {
		t.Fatalf("price update = %+v", price)
	}
}

func TestScannerLivenessRequiresTwoFreshExecutableQuotesForOneSymbol(t *testing.T) {
	scanner := NewFuturesScanner()
	scanner.updatePrice(exchanges.PriceData{
		Symbol: "COTIUSDT", Source: "pyth", Price: 0.0114, Timestamp: 10_000,
	})
	if scanner.IsLiveAt(time.UnixMilli(10_001)) {
		t.Fatal("reference price incorrectly made scanner live")
	}

	scanner.quotes["COTIUSDT"] = map[string]Quote{
		"gate_futures": {
			Symbol: "COTIUSDT", Source: "gate_futures", BestBid: 0.0113, BestAsk: 0.01131, Timestamp: 10_000,
		},
	}
	if scanner.IsLiveAt(time.UnixMilli(10_001)) {
		t.Fatal("single executable source incorrectly made scanner live")
	}
	scanner.quotes["COTIUSDT"]["binance_spot"] = Quote{
		Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 0.0114, BestAsk: 0.01141, Timestamp: 10_000,
	}
	if !scanner.IsLiveAt(time.UnixMilli(24_999)) {
		t.Fatal("fresh scanner reported stale")
	}
	if scanner.IsLiveAt(time.UnixMilli(25_001)) {
		t.Fatal("stale scanner reported live")
	}
}

func TestCheckArbitrageBroadcastsEveryExecutableRoute(t *testing.T) {
	now := time.Now().UnixMilli()
	scanner := NewFuturesScanner()
	client := &wsClient{send: make(chan any, 32)}
	scanner.wsClients[client] = struct{}{}
	scanner.quotes["COTIUSDT"] = map[string]Quote{
		"gate_spot":       {Symbol: "COTIUSDT", Source: "gate_spot", BestBid: 99, BestAsk: 100, Timestamp: now},
		"binance_spot":    {Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 101, BestAsk: 102, Timestamp: now},
		"gate_futures":    {Symbol: "COTIUSDT", Source: "gate_futures", BestBid: 96, BestAsk: 97, Timestamp: now},
		"binance_futures": {Symbol: "COTIUSDT", Source: "binance_futures", BestBid: 99, BestAsk: 100, Timestamp: now},
	}

	scanner.checkArbitrage("COTIUSDT")
	routes := make(map[string]bool)
	for len(client.send) > 0 {
		message, ok := (<-client.send).(map[string]interface{})
		if !ok {
			continue
		}
		opportunity := message["opportunity"].(ArbitrageOpportunity)
		routes[opportunity.BuySource+"->"+opportunity.SellSource] = true
	}
	for _, route := range []string{
		"gate_spot->binance_spot",
		"gate_futures->binance_futures",
		"gate_futures->binance_spot",
	} {
		if !routes[route] {
			t.Fatalf("route %s was not broadcast; got %+v", route, routes)
		}
	}
}

func TestRegisterClientReceivesCurrentOpportunitySnapshots(t *testing.T) {
	now := time.Now().UnixMilli()
	scanner := NewFuturesScanner()
	scanner.quotes["COTIUSDT"] = map[string]Quote{
		"gate_spot":    {Symbol: "COTIUSDT", Source: "gate_spot", BestBid: 99, BestAsk: 100, Timestamp: now},
		"binance_spot": {Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 101, BestAsk: 102, Timestamp: now},
	}
	scanner.checkArbitrage("COTIUSDT")

	client := scanner.registerClient(nil)
	var message opportunitiesSnapshot
	for len(client.send) > 0 {
		candidate, ok := (<-client.send).(opportunitiesSnapshot)
		if ok {
			message = candidate
			break
		}
	}
	if message.Type != "opportunities_snapshot" || message.Version != 1 || message.Symbol != "COTIUSDT" {
		t.Fatalf("snapshot metadata = %+v", message)
	}
	if len(message.Opportunities) != 1 || message.Opportunities[0].BuySource != "gate_spot" {
		t.Fatalf("snapshot opportunities = %+v", message.Opportunities)
	}
}

func TestRegisterClientReceivesCurrentValidQuoteSnapshots(t *testing.T) {
	now := time.Now()
	scanner := NewFuturesScanner()
	scanner.quotes["COTIUSDT"] = map[string]Quote{
		"binance_spot": {
			Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 0.01262, BestAsk: 0.01263,
			Timestamp: now.UnixMilli(),
		},
		"gate_spot": {
			Symbol: "COTIUSDT", Source: "gate_spot", BestBid: 0.01102, BestAsk: 0.01104,
			Timestamp: now.UnixMilli(),
		},
		"stale_spot": {
			Symbol: "COTIUSDT", Source: "stale_spot", BestBid: 0.01, BestAsk: 0.011,
			Timestamp: now.Add(-time.Minute).UnixMilli(),
		},
	}

	client := scanner.registerClient(nil)
	if len(client.send) != 2 {
		t.Fatalf("initial client messages = %d, want two current quote snapshots", len(client.send))
	}
	sources := make(map[string]bool)
	for len(client.send) > 0 {
		message, ok := (<-client.send).(map[string]interface{})
		if !ok || message["type"] != "quote_update" || message["version"] != 1 {
			t.Fatalf("quote snapshot message = %+v", message)
		}
		quote := message["quote"].(Quote)
		sources[quote.Source] = true
	}
	if !sources["binance_spot"] || !sources["gate_spot"] || sources["stale_spot"] {
		t.Fatalf("quote snapshot sources = %+v", sources)
	}
}

func TestCheckArbitrageBroadcastsEmptySnapshotWhenRoutesClose(t *testing.T) {
	now := time.Now().UnixMilli()
	scanner := NewFuturesScanner()
	client := &wsClient{send: make(chan any, 32)}
	scanner.wsClients[client] = struct{}{}
	scanner.quotes["COTIUSDT"] = map[string]Quote{
		"gate_spot":    {Symbol: "COTIUSDT", Source: "gate_spot", BestBid: 99, BestAsk: 100, Timestamp: now},
		"binance_spot": {Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 101, BestAsk: 102, Timestamp: now},
	}
	scanner.checkArbitrage("COTIUSDT")
	for len(client.send) > 0 {
		<-client.send
	}

	scanner.quotes["COTIUSDT"]["binance_spot"] = Quote{
		Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 99, BestAsk: 100, Timestamp: now,
	}
	scanner.checkArbitrage("COTIUSDT")

	foundEmptySnapshot := false
	for len(client.send) > 0 {
		if message, ok := (<-client.send).(opportunitiesSnapshot); ok && len(message.Opportunities) == 0 {
			foundEmptySnapshot = true
		}
	}
	if !foundEmptySnapshot {
		t.Fatal("route removal snapshot was not broadcast")
	}
}

func TestPeriodicRevalidationRefreshesAValidRouteAfterFeedsGoSilent(t *testing.T) {
	now := time.UnixMilli(100_000)
	scanner := NewFuturesScanner()
	client := &wsClient{send: make(chan any, 32)}
	scanner.wsClients[client] = struct{}{}
	scanner.quotes["COTIUSDT"] = map[string]Quote{
		"gate_spot": {
			Symbol: "COTIUSDT", Source: "gate_spot", BestBid: 99, BestAsk: 100,
			Timestamp: now.UnixMilli(),
		},
		"binance_spot": {
			Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 101, BestAsk: 102,
			Timestamp: now.UnixMilli(),
		},
	}
	scanner.checkArbitrageAt("COTIUSDT", now)
	for len(client.send) > 0 {
		<-client.send
	}

	for source, quote := range scanner.quotes["COTIUSDT"] {
		quote.Timestamp = now.Add(5 * time.Second).UnixMilli()
		scanner.quotes["COTIUSDT"][source] = quote
	}
	scanner.checkArbitrageAt("COTIUSDT", now.Add(5*time.Second))
	for len(client.send) > 0 {
		<-client.send
	}

	scanner.revalidateOpportunitiesAt(now.Add(opportunityAlertInterval + time.Millisecond))

	foundRefresh := false
	for len(client.send) > 0 {
		message, ok := (<-client.send).(map[string]interface{})
		if ok && message["type"] == "arbitrage" {
			foundRefresh = true
		}
	}
	if !foundRefresh {
		t.Fatal("periodic revalidation did not refresh a valid silent route")
	}
}

func TestPeriodicRevalidationRefreshesBeforeThePublishedTimestampExpires(t *testing.T) {
	now := time.UnixMilli(100_000)
	scanner := NewFuturesScanner()
	client := &wsClient{send: make(chan any, 32)}
	scanner.wsClients[client] = struct{}{}
	scanner.quotes["COTIUSDT"] = map[string]Quote{
		"gate_spot": {
			Symbol: "COTIUSDT", Source: "gate_spot", BestBid: 99, BestAsk: 100,
			Timestamp: now.Add(-14 * time.Second).UnixMilli(),
		},
		"binance_spot": {
			Symbol: "COTIUSDT", Source: "binance_spot", BestBid: 101, BestAsk: 102,
			Timestamp: now.Add(-14 * time.Second).UnixMilli(),
		},
	}
	scanner.checkArbitrageAt("COTIUSDT", now)
	for len(client.send) > 0 {
		<-client.send
	}

	for source, quote := range scanner.quotes["COTIUSDT"] {
		quote.Timestamp = now.UnixMilli()
		scanner.quotes["COTIUSDT"][source] = quote
	}
	scanner.revalidateOpportunitiesAt(now.Add(time.Second))

	foundRefresh := false
	for len(client.send) > 0 {
		message, ok := (<-client.send).(map[string]interface{})
		if ok && message["type"] == "arbitrage" {
			foundRefresh = true
		}
	}
	if !foundRefresh {
		t.Fatal("periodic revalidation did not refresh before the published route timestamp expired")
	}
}
