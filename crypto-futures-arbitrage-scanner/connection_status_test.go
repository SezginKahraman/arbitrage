package main

import (
	"testing"

	"futures-arbitrage-scanner/exchanges"
)

func TestConnectionStatusIgnoresCancelledSubscriptionGenerations(t *testing.T) {
	scanner := NewFuturesScanner()
	scanner.SetExpectedSubscriptions(map[string][]string{sourceGateSpot: {"BTCUSDT", "ETHUSDT"}})
	scanner.updateConnectionStatus(exchanges.ConnectionStatus{
		Source: sourceGateSpot, Connected: false, Symbols: []string{"BTCUSDT"}, Timestamp: 20,
	})
	snapshot := scanner.connectionSnapshot()
	if len(snapshot) != 1 || snapshot[0].Status.Connected || len(snapshot[0].Status.Symbols) != 2 {
		t.Fatalf("stale generation replaced expected status: %+v", snapshot)
	}
	scanner.updateConnectionStatus(exchanges.ConnectionStatus{
		Source: sourceGateSpot, Connected: true, Symbols: []string{"ETHUSDT", "BTCUSDT"}, Timestamp: 30,
	})
	snapshot = scanner.connectionSnapshot()
	if !snapshot[0].Status.Connected || snapshot[0].Status.Timestamp != 30 {
		t.Fatalf("current generation was not accepted: %+v", snapshot)
	}
}

func TestExpectedSubscriptionsPreserveUnchangedConnectedSources(t *testing.T) {
	scanner := NewFuturesScanner()
	initial := map[string][]string{
		sourceGateSpot: {"BTCUSDT"}, sourceKuCoinSpot: {"BTCUSDT"},
	}
	scanner.SetExpectedSubscriptions(initial)
	for source, symbols := range initial {
		scanner.updateConnectionStatus(exchanges.ConnectionStatus{
			Source: source, Connected: true, Symbols: symbols, Timestamp: 10,
		})
	}
	scanner.SetExpectedSubscriptions(map[string][]string{
		sourceGateSpot: {"BTCUSDT", "ETHUSDT"}, sourceKuCoinSpot: {"BTCUSDT"},
	})

	states := scanner.connectionSnapshot()
	connected := make(map[string]bool, len(states))
	for _, state := range states {
		connected[state.Status.Source] = state.Status.Connected
	}
	if connected[sourceGateSpot] || !connected[sourceKuCoinSpot] {
		t.Fatalf("changed/unchanged connection states = %+v", connected)
	}
}
