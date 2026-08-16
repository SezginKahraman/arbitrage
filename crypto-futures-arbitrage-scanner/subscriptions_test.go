package main

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestSubscriptionSupervisorRestartsOnlyChangedSources(t *testing.T) {
	type start struct {
		source  string
		symbols []string
		ctx     context.Context
	}
	starts := make(chan start, 8)
	runner := func(source string) sourceSubscriptionRunner {
		return func(ctx context.Context, symbols []string) {
			starts <- start{source: source, symbols: append([]string(nil), symbols...), ctx: ctx}
			<-ctx.Done()
		}
	}
	supervisor := newSubscriptionSupervisor(map[string]sourceSubscriptionRunner{
		"spot": runner("spot"), "futures": runner("futures"),
	})
	defer supervisor.Stop()

	supervisor.Reconcile(map[string][]string{"spot": {"BTCUSDT"}, "futures": {"BTCUSDT"}})
	first := []start{<-starts, <-starts}
	supervisor.Reconcile(map[string][]string{"spot": {"BTCUSDT", "ETHUSDT"}, "futures": {"BTCUSDT"}})
	restarted := <-starts
	if restarted.source != "spot" || !reflect.DeepEqual(restarted.symbols, []string{"BTCUSDT", "ETHUSDT"}) {
		t.Fatalf("restarted = %+v", restarted)
	}

	for _, item := range first {
		if item.source == "spot" {
			select {
			case <-item.ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("changed source context was not cancelled")
			}
		} else if item.ctx.Err() != nil {
			t.Fatal("unchanged source was cancelled")
		}
	}
	select {
	case unexpected := <-starts:
		t.Fatalf("unexpected restart: %+v", unexpected)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestScannerWatchlistPurgesRemovedQuotesAndRoutes(t *testing.T) {
	scanner := NewFuturesScanner()
	scanner.SetWatchlist([]string{"BTCUSDT", "COTIUSDT"})
	scanner.updateQuote(Quote{Symbol: "COTIUSDT", Source: sourceBinanceSpot, BestBid: 1, BestAsk: 1.01, Timestamp: 1})
	scanner.currentRoutes["COTIUSDT"] = map[string]ArbitrageOpportunity{"route": {Symbol: "COTIUSDT"}}
	scanner.SetWatchlist([]string{"BTCUSDT"})

	scanner.quotesMutex.RLock()
	_, quoteExists := scanner.quotes["COTIUSDT"]
	scanner.quotesMutex.RUnlock()
	scanner.opportunityMutex.RLock()
	_, routeExists := scanner.currentRoutes["COTIUSDT"]
	scanner.opportunityMutex.RUnlock()
	if quoteExists || routeExists {
		t.Fatalf("removed symbol remains: quotes=%v routes=%v", quoteExists, routeExists)
	}

	// A late frame from the cancelled generation must not recreate removed state.
	scanner.updateQuote(Quote{Symbol: "COTIUSDT", Source: sourceGateSpot, BestBid: 1, BestAsk: 1.01, Timestamp: 2})
	scanner.quotesMutex.RLock()
	_, quoteExists = scanner.quotes["COTIUSDT"]
	scanner.quotesMutex.RUnlock()
	if quoteExists {
		t.Fatal("late quote recreated a removed symbol")
	}
}

func TestSubscriptionSupervisorConcurrentReconcileIsSafe(t *testing.T) {
	supervisor := newSubscriptionSupervisor(map[string]sourceSubscriptionRunner{
		"spot": func(ctx context.Context, _ []string) { <-ctx.Done() },
	})
	defer supervisor.Stop()
	var waitGroup sync.WaitGroup
	for index := 0; index < 20; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			if index%2 == 0 {
				supervisor.Reconcile(map[string][]string{"spot": {"BTCUSDT"}})
			} else {
				supervisor.Reconcile(map[string][]string{"spot": {"ETHUSDT"}})
			}
		}(index)
	}
	waitGroup.Wait()
}
