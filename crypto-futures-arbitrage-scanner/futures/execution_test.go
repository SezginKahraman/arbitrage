package futures

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type recordingTrader struct {
	venue           string
	placeErr        error
	closeErr        error
	preflightErr    error
	filledContracts float64
	placeStarted    chan struct{}
	releasePlace    chan struct{}
	mutex           sync.Mutex
	placed          []OrderIntent
	closed          []OrderIntent
}

type sequenceVenueProvider struct {
	mutex   sync.Mutex
	markets []VenueMarket
	calls   int
}

func (provider *sequenceVenueProvider) VenueMarket(context.Context, string, int) (VenueMarket, error) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	index := min(provider.calls, len(provider.markets)-1)
	provider.calls++
	return provider.markets[index], nil
}

func (trader *recordingTrader) Venue() string { return trader.venue }
func (trader *recordingTrader) Preflight(context.Context, string, float64) error {
	return trader.preflightErr
}
func (trader *recordingTrader) PlaceMarket(_ context.Context, intent OrderIntent) (OrderFill, error) {
	trader.mutex.Lock()
	trader.placed = append(trader.placed, intent)
	trader.mutex.Unlock()
	if trader.placeStarted != nil {
		select {
		case trader.placeStarted <- struct{}{}:
		default:
		}
	}
	if trader.releasePlace != nil {
		<-trader.releasePlace
	}
	if trader.placeErr != nil {
		return OrderFill{}, trader.placeErr
	}
	filled := intent.Contracts
	if trader.filledContracts > 0 {
		filled = trader.filledContracts
	}
	return OrderFill{Venue: trader.venue, OrderID: trader.venue + "-1", FilledContracts: filled, AveragePrice: 100}, nil
}

func TestExecutionServiceRejectsConcurrentDuplicateExecution(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	gateTrader := &recordingTrader{venue: "gate_futures", placeStarted: started, releasePlace: release}
	binanceTrader := &recordingTrader{venue: "binance_futures"}
	service := NewExecutionService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)
	request := ExecuteRequest{
		Contract: "COTI_USDT", TradeNotional: 100, MinNetSpreadPct: 0.5,
		ExpectedLongVenue: "gate_futures", ExpectedShortVenue: "binance_futures", ConfirmLive: true,
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Execute(context.Background(), request)
		firstDone <- err
	}()
	<-started

	_, duplicateErr := service.Execute(context.Background(), request)
	if !errors.Is(duplicateErr, ErrExecutionInProgress) {
		t.Fatalf("duplicate error = %v", duplicateErr)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first execution error = %v", err)
	}
	if len(gateTrader.placed) != 1 || len(binanceTrader.placed) != 1 {
		t.Fatalf("duplicate orders placed: gate=%d binance=%d", len(gateTrader.placed), len(binanceTrader.placed))
	}
}
func (trader *recordingTrader) CloseFilled(_ context.Context, intent OrderIntent, fill OrderFill) (OrderFill, error) {
	trader.mutex.Lock()
	trader.closed = append(trader.closed, intent)
	trader.mutex.Unlock()
	if trader.closeErr != nil {
		return OrderFill{}, trader.closeErr
	}
	return OrderFill{Venue: trader.venue, OrderID: trader.venue + "-close", FilledContracts: fill.FilledContracts, AveragePrice: 100}, nil
}

func executableMarkets() (*fakeGateWorkspaceProvider, *fakeVenueProvider) {
	gate := &fakeGateWorkspaceProvider{market: VenueMarket{
		Venue: "gate_futures", Symbol: "COTIUSDT", IndexPrice: 100,
		ContractMultiplier: 1, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.00075,
		Book: OrderBook{Bids: []BookLevel{{Price: 99.8, Size: 10}}, Asks: []BookLevel{{Price: 100, Size: 10}}},
	}}
	binance := &fakeVenueProvider{market: VenueMarket{
		Venue: "binance_futures", Symbol: "COTIUSDT", IndexPrice: 100.2,
		ContractMultiplier: 1, QuantityStep: 1, MinQuantity: 1, TakerFeeRate: 0.0005,
		Book: OrderBook{Bids: []BookLevel{{Price: 101, Size: 10}}, Asks: []BookLevel{{Price: 101.2, Size: 10}}},
	}}
	return gate, binance
}

func TestExecutionServiceOpensBothFreshlyRevalidatedLegsAfterExplicitConfirmation(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	gateTrader := &recordingTrader{venue: "gate_futures"}
	binanceTrader := &recordingTrader{venue: "binance_futures"}
	service := NewExecutionService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)

	result, err := service.Execute(context.Background(), ExecuteRequest{
		Contract: "COTI_USDT", TradeNotional: 100, MinNetSpreadPct: 0.5,
		ExpectedLongVenue: "gate_futures", ExpectedShortVenue: "binance_futures", ConfirmLive: true,
	})

	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ExecutionOpened || len(result.Fills) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if len(gateTrader.placed) != 1 || gateTrader.placed[0].Side != OrderBuy || gateTrader.placed[0].Contracts != 1 {
		t.Fatalf("gate intents = %+v", gateTrader.placed)
	}
	if len(binanceTrader.placed) != 1 || binanceTrader.placed[0].Side != OrderSell || binanceTrader.placed[0].Contracts != 1 {
		t.Fatalf("binance intents = %+v", binanceTrader.placed)
	}
}

func TestExecutionServiceCompensatesTheFilledLegWhenTheOtherVenueFails(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	gateTrader := &recordingTrader{venue: "gate_futures"}
	binanceTrader := &recordingTrader{venue: "binance_futures", placeErr: errors.New("venue rejected")}
	service := NewExecutionService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)

	result, err := service.Execute(context.Background(), ExecuteRequest{
		Contract: "COTI_USDT", TradeNotional: 100, MinNetSpreadPct: 0.5,
		ExpectedLongVenue: "gate_futures", ExpectedShortVenue: "binance_futures", ConfirmLive: true,
	})

	if !errors.Is(err, ErrLegExecutionFailed) || result.Status != ExecutionCompensated {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if len(gateTrader.closed) != 1 || gateTrader.closed[0].Side != OrderSell {
		t.Fatalf("gate compensation = %+v", gateTrader.closed)
	}
}

func TestExecutionServiceReturnsSanitizedVenuePreflightFailureBeforeOrders(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	gateTrader := &recordingTrader{venue: "gate_futures"}
	binanceTrader := &recordingTrader{venue: "binance_futures", preflightErr: errors.New("private account detail")}
	service := NewExecutionService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)

	result, err := service.Execute(context.Background(), ExecuteRequest{
		Contract: "COTI_USDT", TradeNotional: 100, MinNetSpreadPct: 0.5,
		ExpectedLongVenue: "gate_futures", ExpectedShortVenue: "binance_futures", ConfirmLive: true,
	})

	if !errors.Is(err, ErrPreflightFailed) || result.ReasonCode != "binance_futures_preflight_failed" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if len(gateTrader.placed) != 0 || len(binanceTrader.placed) != 0 {
		t.Fatalf("orders placed after preflight failure: gate=%v binance=%v", gateTrader.placed, binanceTrader.placed)
	}
}

func TestExecutionServiceClosesBothLegsWhenEitherMarketOrderPartiallyFills(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	gateTrader := &recordingTrader{venue: "gate_futures", filledContracts: 0.5}
	binanceTrader := &recordingTrader{venue: "binance_futures"}
	service := NewExecutionService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)

	result, err := service.Execute(context.Background(), ExecuteRequest{
		Contract: "COTI_USDT", TradeNotional: 100, MinNetSpreadPct: 0.5,
		ExpectedLongVenue: "gate_futures", ExpectedShortVenue: "binance_futures", ConfirmLive: true,
	})

	if !errors.Is(err, ErrLegExecutionFailed) || result.Status != ExecutionCompensated {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if len(gateTrader.closed) != 1 || len(binanceTrader.closed) != 1 {
		t.Fatalf("partial-fill compensation: gate=%+v binance=%+v", gateTrader.closed, binanceTrader.closed)
	}
	if len(result.Compensations) != 2 {
		t.Fatalf("compensations = %+v", result.Compensations)
	}
}

func TestExecutionServiceRejectsDisabledOrChangedRoutesBeforeTrading(t *testing.T) {
	gateMarket, binanceMarket := executableMarkets()
	gateTrader := &recordingTrader{venue: "gate_futures"}
	binanceTrader := &recordingTrader{venue: "binance_futures"}
	disabled := NewExecutionService(false, gateMarket, binanceMarket, gateTrader, binanceTrader)

	_, err := disabled.Execute(context.Background(), ExecuteRequest{
		Contract: "COTI_USDT", TradeNotional: 100, MinNetSpreadPct: 0.5,
		ExpectedLongVenue: "gate_futures", ExpectedShortVenue: "binance_futures", ConfirmLive: true,
	})
	if !errors.Is(err, ErrLiveTradingDisabled) {
		t.Fatalf("disabled error = %v", err)
	}

	enabled := NewExecutionService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)
	_, err = enabled.Execute(context.Background(), ExecuteRequest{
		Contract: "COTI_USDT", TradeNotional: 100, MinNetSpreadPct: 0.5,
		ExpectedLongVenue: "binance_futures", ExpectedShortVenue: "gate_futures", ConfirmLive: true,
	})
	if !errors.Is(err, ErrRouteChanged) {
		t.Fatalf("changed route error = %v", err)
	}
	if len(gateTrader.placed) != 0 || len(binanceTrader.placed) != 0 {
		t.Fatalf("orders placed before rejection: gate=%v binance=%v", gateTrader.placed, binanceTrader.placed)
	}
}

func TestExecutionServiceRevalidatesRouteAgainAfterPrivatePreflight(t *testing.T) {
	initialGate, initialBinance := executableMarkets()
	closedSpread := initialBinance.market
	closedSpread.Book.Bids = []BookLevel{{Price: 100.05, Size: 10}}
	gateMarket := &sequenceVenueProvider{markets: []VenueMarket{initialGate.market, initialGate.market}}
	binanceMarket := &sequenceVenueProvider{markets: []VenueMarket{initialBinance.market, closedSpread}}
	gateTrader := &recordingTrader{venue: "gate_futures"}
	binanceTrader := &recordingTrader{venue: "binance_futures"}
	service := NewExecutionService(true, gateMarket, binanceMarket, gateTrader, binanceTrader)

	_, err := service.Execute(context.Background(), ExecuteRequest{
		Contract: "COTI_USDT", TradeNotional: 100, MinNetSpreadPct: 0.5,
		ExpectedLongVenue: "gate_futures", ExpectedShortVenue: "binance_futures", ConfirmLive: true,
	})

	if !errors.Is(err, ErrRouteChanged) {
		t.Fatalf("route change error = %v", err)
	}
	if len(gateTrader.placed) != 0 || len(binanceTrader.placed) != 0 {
		t.Fatalf("orders placed after spread closed: gate=%v binance=%v", gateTrader.placed, binanceTrader.placed)
	}
}
