package futures

import (
	"context"
	"testing"
)

type fakeGateWorkspaceProvider struct {
	analysis Analysis
	market   VenueMarket
	candle   Candle
}

func (provider *fakeGateWorkspaceProvider) Contracts(context.Context) ([]Contract, error) {
	return []Contract{provider.analysis.Contract}, nil
}

func (provider *fakeGateWorkspaceProvider) Analyze(context.Context, AnalyzeRequest) (Analysis, error) {
	return provider.analysis, nil
}

func (provider *fakeGateWorkspaceProvider) VenueMarket(context.Context, string, int) (VenueMarket, error) {
	return provider.market, nil
}

func (provider *fakeGateWorkspaceProvider) LatestCandle(context.Context, string, string) (Candle, error) {
	return provider.candle, nil
}

type fakeVenueProvider struct{ market VenueMarket }

func (provider *fakeVenueProvider) VenueMarket(context.Context, string, int) (VenueMarket, error) {
	return provider.market, nil
}

func TestWorkspaceAnalyzeAddsNeutralRouteAndLiveSnapshotAdvancesCandle(t *testing.T) {
	gate := &fakeGateWorkspaceProvider{
		analysis: Analysis{Contract: Contract{Symbol: "COTI_USDT"}, Interval: "15m", Strategy: StrategyTrendPullback},
		market: VenueMarket{
			Venue: "gate_futures", Symbol: "COTIUSDT", IndexPrice: 100, MarkPrice: 100,
			ContractMultiplier: 1, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.00075,
			Book: OrderBook{Bids: []BookLevel{{Price: 99.8, Size: 10}}, Asks: []BookLevel{{Price: 100, Size: 10}}},
		},
		candle: Candle{Time: 1_700_000_900, Open: 100, High: 102, Low: 99, Close: 101, Volume: 400},
	}
	binance := &fakeVenueProvider{market: VenueMarket{
		Venue: "binance_futures", Symbol: "COTIUSDT", IndexPrice: 100.2, MarkPrice: 101,
		ContractMultiplier: 1, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.0005,
		Book: OrderBook{Bids: []BookLevel{{Price: 101, Size: 10}}, Asks: []BookLevel{{Price: 101.2, Size: 10}}},
	}}
	service := NewWorkspaceService(gate, binance)
	request := AnalyzeRequest{
		Contract: "COTI_USDT", Interval: "15m", Strategy: StrategyTrendPullback,
		AccountBalance: 1_000, RiskPercent: 1, TradeNotional: 100, MinNetSpreadPct: 0.5,
	}

	analysis, err := service.Analyze(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.NeutralRoute == nil || analysis.NeutralRoute.Status != NeutralRouteExecutable {
		t.Fatalf("neutral route = %+v", analysis.NeutralRoute)
	}

	live, err := service.Live(context.Background(), LiveRequest{
		Contract: "COTI_USDT", Interval: "15m", TradeNotional: 100, MinNetSpreadPct: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if live.Candle.Time != 1_700_000_900 || live.Candle.Close != 101 || live.Route.Status != NeutralRouteExecutable {
		t.Fatalf("live snapshot = %+v", live)
	}
}
