package futures

import (
	"math"
	"testing"
)

func TestFindNeutralRouteUsesExecutableDepthAndFourTakerFees(t *testing.T) {
	gate := VenueMarket{
		Venue: "gate_futures", Symbol: "COTIUSDT", IndexPrice: 100,
		ContractMultiplier: 1, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.00075,
		FundingRate: 0.0001,
		Book: OrderBook{
			Bids: []BookLevel{{Price: 99.8, Size: 2}},
			Asks: []BookLevel{{Price: 100, Size: 2}},
		},
	}
	binance := VenueMarket{
		Venue: "binance_futures", Symbol: "COTIUSDT", IndexPrice: 100.2,
		ContractMultiplier: 1, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.0005,
		FundingRate: 0.0002,
		Book: OrderBook{
			Bids: []BookLevel{{Price: 101, Size: 2}},
			Asks: []BookLevel{{Price: 101.2, Size: 2}},
		},
	}

	route := FindNeutralRoute(CrossVenueRequest{
		Notional: 100, MinNetSpreadPct: 0.5, MaxIndexDivergencePct: 3,
	}, gate, binance)

	if route.Status != NeutralRouteExecutable {
		t.Fatalf("status = %q, reasons = %v", route.Status, route.Reasons)
	}
	if route.LongVenue != "gate_futures" || route.ShortVenue != "binance_futures" {
		t.Fatalf("route = %s -> %s", route.LongVenue, route.ShortVenue)
	}
	if route.BaseQuantity != 1 || route.LongContracts != 1 || route.ShortContracts != 1 {
		t.Fatalf("quantities = base %v, long %v, short %v", route.BaseQuantity, route.LongContracts, route.ShortContracts)
	}
	assertClose(t, "gross spread", route.GrossSpreadPct, 1)
	assertClose(t, "open fees", route.OpenFees, 0.1255)
	assertClose(t, "close fees", route.EstimatedCloseFees, 0.125625)
	assertClose(t, "net convergence spread", route.NetConvergenceSpreadPct, 0.748875)
	assertClose(t, "funding difference", route.NextFundingCarryPct, 0.01)
}

func TestFindNeutralRouteRejectsSymbolCollisionByIndexDivergence(t *testing.T) {
	left := VenueMarket{
		Venue: "gate_futures", Symbol: "EDGEUSDT", IndexPrice: 0.063,
		ContractMultiplier: 10, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.00075,
		Book: OrderBook{Bids: []BookLevel{{Price: 0.062, Size: 1000}}, Asks: []BookLevel{{Price: 0.064, Size: 1000}}},
	}
	right := VenueMarket{
		Venue: "binance_futures", Symbol: "EDGEUSDT", IndexPrice: 0.288,
		ContractMultiplier: 1, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.0005,
		Book: OrderBook{Bids: []BookLevel{{Price: 0.287, Size: 10000}}, Asks: []BookLevel{{Price: 0.289, Size: 10000}}},
	}

	route := FindNeutralRoute(CrossVenueRequest{
		Notional: 100, MinNetSpreadPct: 0.1, MaxIndexDivergencePct: 3,
	}, left, right)

	if route.Status != NeutralRouteRejected || route.ReasonCode != "contract_identity_unverified" {
		t.Fatalf("route = %+v", route)
	}
}

func TestFindNeutralRouteRejectsInsufficientMatchedDepth(t *testing.T) {
	left := VenueMarket{
		Venue: "gate_futures", Symbol: "SFPUSDT", IndexPrice: 1,
		ContractMultiplier: 10, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.00075,
		Book: OrderBook{Bids: []BookLevel{{Price: 0.99, Size: 1}}, Asks: []BookLevel{{Price: 1, Size: 1}}},
	}
	right := VenueMarket{
		Venue: "binance_futures", Symbol: "SFPUSDT", IndexPrice: 1,
		ContractMultiplier: 1, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.0005,
		Book: OrderBook{Bids: []BookLevel{{Price: 1.02, Size: 5}}, Asks: []BookLevel{{Price: 1.03, Size: 5}}},
	}

	route := FindNeutralRoute(CrossVenueRequest{
		Notional: 100, MinNetSpreadPct: 0.1, MaxIndexDivergencePct: 3,
	}, left, right)

	if route.Status != NeutralRouteRejected || route.ReasonCode != "insufficient_depth" {
		t.Fatalf("route = %+v", route)
	}
}

func assertClose(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("%s = %.9f, want %.9f", label, got, want)
	}
}
