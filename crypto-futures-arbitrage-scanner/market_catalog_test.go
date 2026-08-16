package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestMarketCatalogBuildsCommonUSDTMarketsAndCoverage(t *testing.T) {
	checkedAt := time.UnixMilli(42_000)
	catalog := newMarketCatalog([]marketSourceDefinition{
		{Source: sourceBinanceSpot, Market: marketSpot, Fetch: func(context.Context) ([]string, error) {
			return []string{"BTCUSDT", "ETHUSDT", "ONLYBINANCEUSDT", "BTCUSDT"}, nil
		}},
		{Source: sourceGateSpot, Market: marketSpot, Fetch: func(context.Context) ([]string, error) {
			return []string{"BTCUSDT", "ETHUSDT"}, nil
		}},
		{Source: sourceKuCoinFutures, Market: marketFutures, Fetch: func(context.Context) ([]string, error) {
			return []string{"BTCUSDT", "SOLUSDT"}, nil
		}},
	})

	catalog.RefreshAt(context.Background(), checkedAt)
	candidates := catalog.Candidates()
	if got := []string{candidates[0].Symbol, candidates[1].Symbol}; !reflect.DeepEqual(got, []string{"BTCUSDT", "ETHUSDT"}) {
		t.Fatalf("candidate symbols = %v, want common symbols only", got)
	}
	if got := candidates[0].SpotSources; !reflect.DeepEqual(got, []string{sourceBinanceSpot, sourceGateSpot}) {
		t.Fatalf("BTC spot sources = %v", got)
	}
	if got := candidates[0].FuturesSources; !reflect.DeepEqual(got, []string{sourceKuCoinFutures}) {
		t.Fatalf("BTC futures sources = %v", got)
	}
	if candidates[1].FuturesSources == nil || len(candidates[1].FuturesSources) != 0 {
		t.Fatalf("spot-only ETH futures sources = %#v, want non-nil empty list", candidates[1].FuturesSources)
	}
	if got := catalog.SymbolsForSource([]string{"ETHUSDT", "BTCUSDT", "SOLUSDT"}, sourceKuCoinFutures); !reflect.DeepEqual(got, []string{"BTCUSDT", "SOLUSDT"}) {
		t.Fatalf("KuCoin futures symbols = %v", got)
	}
}

func TestMarketCatalogRetainsLastGoodSourceDataWhenRefreshFails(t *testing.T) {
	fail := false
	catalog := newMarketCatalog([]marketSourceDefinition{{
		Source: sourceBinanceSpot, Market: marketSpot,
		Fetch: func(context.Context) ([]string, error) {
			if fail {
				return nil, errors.New("temporary outage")
			}
			return []string{"BTCUSDT"}, nil
		},
	}})
	catalog.RefreshAt(context.Background(), time.UnixMilli(1_000))
	fail = true
	catalog.RefreshAt(context.Background(), time.UnixMilli(2_000))

	states := catalog.SourceStates()
	if len(states) != 1 || states[0].Status != marketSourceStale || !reflect.DeepEqual(states[0].Symbols, []string{"BTCUSDT"}) {
		t.Fatalf("source states = %+v", states)
	}
}

func TestNormalizeDiscoveredSymbolAcceptsOnlySimpleUSDTMarkets(t *testing.T) {
	for input, want := range map[string]string{
		"btc-usdt":     "BTCUSDT",
		"SOL_USDT":     "SOLUSDT",
		"ETHUSDT":      "ETHUSDT",
		"BTC-USD":      "",
		"USDTUSDT":     "",
		"1000PEPEUSDT": "1000PEPEUSDT",
	} {
		if got := normalizeDiscoveredSymbol(input); got != want {
			t.Errorf("normalizeDiscoveredSymbol(%q) = %q, want %q", input, got, want)
		}
	}
}
