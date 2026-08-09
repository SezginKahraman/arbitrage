package main

import (
	"math"
	"testing"
	"time"
)

func TestFindBestOpportunityUsesAskForBuyAndBidForSell(t *testing.T) {
	now := time.UnixMilli(20_000)
	quotes := map[string]Quote{
		"exchange_a": {Symbol: "COTIUSDT", Source: "exchange_a", BestBid: 0.01129, BestAsk: 0.01131, Timestamp: now.UnixMilli()},
		"exchange_b": {Symbol: "COTIUSDT", Source: "exchange_b", BestBid: 0.01140723, BestAsk: 0.01142, Timestamp: now.UnixMilli()},
	}

	opportunity, ok := FindBestOpportunityAt("COTIUSDT", quotes, now)
	if !ok {
		t.Fatal("FindBestOpportunityAt returned no opportunity")
	}
	if opportunity.BuySource != "exchange_a" || opportunity.SellSource != "exchange_b" {
		t.Fatalf("route = %s -> %s, want exchange_a -> exchange_b", opportunity.BuySource, opportunity.SellSource)
	}
	if opportunity.BuyPrice != 0.01131 {
		t.Fatalf("buy price = %.8f, want ask %.8f", opportunity.BuyPrice, 0.01131)
	}
	if opportunity.SellPrice != 0.01140723 {
		t.Fatalf("sell price = %.8f, want bid %.8f", opportunity.SellPrice, 0.01140723)
	}
	wantSpread := ((0.01140723 - 0.01131) / 0.01131) * 100
	if math.Abs(opportunity.ProfitPct-wantSpread) > 1e-9 {
		t.Fatalf("spread = %.12f, want %.12f", opportunity.ProfitPct, wantSpread)
	}
}

func TestFindBestOpportunityRejectsSameSourceMidpointFalsePositive(t *testing.T) {
	now := time.UnixMilli(20_000)
	quotes := map[string]Quote{
		"crossed": {Symbol: "COTIUSDT", Source: "crossed", BestBid: 105, BestAsk: 100, Timestamp: now.UnixMilli()},
		"normal":  {Symbol: "COTIUSDT", Source: "normal", BestBid: 99, BestAsk: 106, Timestamp: now.UnixMilli()},
	}

	if _, ok := FindBestOpportunityAt("COTIUSDT", quotes, now); ok {
		t.Fatal("invalid crossed same-source book produced an opportunity")
	}
}

func TestFindBestOpportunityExcludesStaleAndInvalidQuotes(t *testing.T) {
	now := time.UnixMilli(40_000)
	quotes := map[string]Quote{
		"fresh":   {Symbol: "COTIUSDT", Source: "fresh", BestBid: 99, BestAsk: 100, Timestamp: now.UnixMilli()},
		"stale":   {Symbol: "COTIUSDT", Source: "stale", BestBid: 120, BestAsk: 121, Timestamp: now.Add(-16 * time.Second).UnixMilli()},
		"invalid": {Symbol: "COTIUSDT", Source: "invalid", BestBid: 110, BestAsk: 0, Timestamp: now.UnixMilli()},
	}

	if _, ok := FindBestOpportunityAt("COTIUSDT", quotes, now); ok {
		t.Fatal("stale or invalid quote produced an opportunity")
	}
}
