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
		{"Gate spot", sourceGateSpot, withCOTI},
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
