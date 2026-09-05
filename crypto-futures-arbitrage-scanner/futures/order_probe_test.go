package futures

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingProbeTrader struct {
	venue        string
	preflightErr error
	plans        []OrderProbePlan
	result       OrderProbeResult
	err          error
	probeStarted chan struct{}
	releaseProbe chan struct{}
}

func (trader *recordingProbeTrader) Venue() string { return trader.venue }
func (trader *recordingProbeTrader) Preflight(context.Context, string, float64) error {
	return trader.preflightErr
}
func (trader *recordingProbeTrader) ProbePostOnly(_ context.Context, plan OrderProbePlan) (OrderProbeResult, error) {
	trader.plans = append(trader.plans, plan)
	if trader.probeStarted != nil {
		select {
		case trader.probeStarted <- struct{}{}:
		default:
		}
	}
	if trader.releaseProbe != nil {
		<-trader.releaseProbe
	}
	return trader.result, trader.err
}

func TestBuildPostOnlyProbePlanUsesMinimumValidNotionalBelowTheBestBid(t *testing.T) {
	market := VenueMarket{
		Venue: "binance_futures", Symbol: "HUSDT", ContractMultiplier: 1,
		QuantityStep: 1, MinQuantity: 1, PriceIncrement: 0.00001, MinNotional: 5,
		Book: OrderBook{
			Bids: []BookLevel{{Price: 0.13611, Size: 10_000}},
			Asks: []BookLevel{{Price: 0.13613, Size: 10_000}},
		},
	}

	plan, err := buildPostOnlyProbePlan("H_USDT", market)

	if err != nil {
		t.Fatal(err)
	}
	if plan.Side != OrderBuy || plan.Price != 0.13338 || plan.Contracts != 38 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Notional > 10 || plan.Notional < 5 {
		t.Fatalf("probe notional = %.8f", plan.Notional)
	}
}

func TestAccessServiceReportsBothVenuesWithoutPlacingOrders(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	gateMarket.market.PriceIncrement = 0.01
	binanceMarket.market.PriceIncrement = 0.01
	gateTrader := &recordingProbeTrader{venue: "gate_futures"}
	binanceTrader := &recordingProbeTrader{venue: "binance_futures", preflightErr: ErrInsufficientMargin}
	service := NewAccessService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)

	report, err := service.Access(context.Background(), AccessRequest{Contract: "COTI_USDT", RequiredNotional: 5})

	if err != nil {
		t.Fatal(err)
	}
	if len(report.Venues) != 2 || !report.Venues[0].Ready || report.Venues[1].ReasonCode != "insufficient_margin" {
		t.Fatalf("report = %+v", report)
	}
	if len(gateTrader.plans) != 0 || len(binanceTrader.plans) != 0 {
		t.Fatalf("read-only access check placed orders: gate=%v binance=%v", gateTrader.plans, binanceTrader.plans)
	}
}

func TestAccessServiceRequiresKillSwitchAndExplicitConfirmationBeforeProbe(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	gateMarket.market.PriceIncrement = 0.01
	binanceMarket.market.PriceIncrement = 0.01
	gateTrader := &recordingProbeTrader{venue: "gate_futures"}
	binanceTrader := &recordingProbeTrader{venue: "binance_futures"}

	disabled := NewAccessService(false, gateMarket, binanceMarket, gateTrader, binanceTrader)
	_, err := disabled.Probe(context.Background(), OrderProbeRequest{Venue: "gate_futures", Contract: "COTI_USDT", ConfirmLive: true})
	if !errors.Is(err, ErrLiveTradingDisabled) {
		t.Fatalf("disabled error = %v", err)
	}

	enabled := NewAccessService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)
	_, err = enabled.Probe(context.Background(), OrderProbeRequest{Venue: "gate_futures", Contract: "COTI_USDT"})
	if !errors.Is(err, ErrLiveConfirmationRequired) {
		t.Fatalf("confirmation error = %v", err)
	}
	if len(gateTrader.plans) != 0 || len(binanceTrader.plans) != 0 {
		t.Fatalf("orders placed before confirmation: gate=%v binance=%v", gateTrader.plans, binanceTrader.plans)
	}
}

func TestAccessServicePlacesOnlyTheSelectedVenueProbe(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	gateMarket.market.ContractMultiplier = 0.001
	gateMarket.market.PriceIncrement = 0.01
	binanceMarket.market.PriceIncrement = 0.01
	gateTrader := &recordingProbeTrader{venue: "gate_futures", result: OrderProbeResult{Status: OrderProbeCancelled, CleanupVerified: true}}
	binanceTrader := &recordingProbeTrader{venue: "binance_futures"}
	service := NewAccessService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)

	result, err := service.Probe(context.Background(), OrderProbeRequest{
		Venue: "gate_futures", Contract: "COTI_USDT", ConfirmLive: true,
	})

	if err != nil {
		t.Fatal(err)
	}
	if result.Status != OrderProbeCancelled || !result.CleanupVerified || len(gateTrader.plans) != 1 || len(binanceTrader.plans) != 0 {
		t.Fatalf("result=%+v gate=%+v binance=%+v", result, gateTrader.plans, binanceTrader.plans)
	}
	if gateTrader.plans[0].Venue != "gate_futures" || gateTrader.plans[0].Side != OrderBuy {
		t.Fatalf("plan = %+v", gateTrader.plans[0])
	}
}

func TestAccessServiceRejectsConcurrentProbeForTheSameVenueAndContract(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	gateMarket.market.ContractMultiplier = 0.001
	gateMarket.market.PriceIncrement = 0.01
	binanceMarket.market.PriceIncrement = 0.01
	started, release := make(chan struct{}, 1), make(chan struct{})
	gateTrader := &recordingProbeTrader{
		venue: "gate_futures", result: OrderProbeResult{Status: OrderProbeCancelled, CleanupVerified: true},
		probeStarted: started, releaseProbe: release,
	}
	service := NewAccessService(true, gateMarket, binanceMarket, gateTrader, &recordingProbeTrader{venue: "binance_futures"})
	request := OrderProbeRequest{Venue: "gate_futures", Contract: "COTI_USDT", ConfirmLive: true}

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Probe(context.Background(), request)
		firstDone <- err
	}()
	<-started

	duplicateDone := make(chan error, 1)
	go func() {
		_, err := service.Probe(context.Background(), request)
		duplicateDone <- err
	}()
	select {
	case duplicateErr := <-duplicateDone:
		if !errors.Is(duplicateErr, ErrOrderProbeInProgress) {
			t.Fatalf("duplicate error = %v", duplicateErr)
		}
	case <-time.After(50 * time.Millisecond):
		close(release)
		<-duplicateDone
		<-firstDone
		t.Fatal("duplicate probe was not rejected before reaching the venue")
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first probe error = %v", err)
	}
	if len(gateTrader.plans) != 1 {
		t.Fatalf("probe count = %d", len(gateTrader.plans))
	}
}
