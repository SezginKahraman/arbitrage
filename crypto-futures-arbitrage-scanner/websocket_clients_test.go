package main

import (
	"testing"
	"time"

	"futures-arbitrage-scanner/exchanges"
)

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
