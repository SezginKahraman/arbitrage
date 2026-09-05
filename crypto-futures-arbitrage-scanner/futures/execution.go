package futures

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrLiveTradingDisabled      = errors.New("live futures trading disabled")
	ErrLiveConfirmationRequired = errors.New("live futures confirmation required")
	ErrExecutionInProgress      = errors.New("futures execution already in progress")
	ErrPreflightFailed          = errors.New("futures account preflight failed")
	ErrRouteChanged             = errors.New("futures route changed")
	ErrLegExecutionFailed       = errors.New("futures leg execution failed")
	ErrCompensationFailed       = errors.New("futures compensation failed")
)

type OrderSide string

const (
	OrderBuy  OrderSide = "buy"
	OrderSell OrderSide = "sell"
)

type ExecutionStatus string

const (
	ExecutionOpened      ExecutionStatus = "opened"
	ExecutionFailed      ExecutionStatus = "failed"
	ExecutionCompensated ExecutionStatus = "compensated"
	ExecutionExposed     ExecutionStatus = "exposed"
)

type ExecuteRequest struct {
	Contract           string  `json:"contract"`
	TradeNotional      float64 `json:"trade_notional"`
	MinNetSpreadPct    float64 `json:"min_net_spread_pct"`
	ExpectedLongVenue  string  `json:"expected_long_venue"`
	ExpectedShortVenue string  `json:"expected_short_venue"`
	ConfirmLive        bool    `json:"confirm_live"`
}

type OrderIntent struct {
	Venue      string    `json:"venue"`
	Contract   string    `json:"contract"`
	Side       OrderSide `json:"side"`
	Contracts  float64   `json:"contracts"`
	ReduceOnly bool      `json:"reduce_only"`
}

type OrderFill struct {
	Venue           string  `json:"venue"`
	OrderID         string  `json:"order_id"`
	FilledContracts float64 `json:"filled_contracts"`
	AveragePrice    float64 `json:"average_price"`
}

type ExecutionResult struct {
	Status        ExecutionStatus `json:"status"`
	ReasonCode    string          `json:"reason_code,omitempty"`
	Route         NeutralRoute    `json:"route"`
	Fills         []OrderFill     `json:"fills"`
	Compensation  *OrderFill      `json:"compensation,omitempty"`
	Compensations []OrderFill     `json:"compensations,omitempty"`
}

type ExecutionService interface {
	Execute(context.Context, ExecuteRequest) (ExecutionResult, error)
}

type venueTrader interface {
	Venue() string
	Preflight(context.Context, string, float64) error
	PlaceMarket(context.Context, OrderIntent) (OrderFill, error)
	CloseFilled(context.Context, OrderIntent, OrderFill) (OrderFill, error)
}

type DualVenueExecutionService struct {
	enabled       bool
	gateMarket    venueMarketProvider
	binanceMarket venueMarketProvider
	traders       map[string]venueTrader
	mutex         sync.Mutex
	inFlight      map[string]struct{}
}

func NewExecutionService(enabled bool, gateMarket, binanceMarket venueMarketProvider, traders ...venueTrader) *DualVenueExecutionService {
	items := make(map[string]venueTrader, len(traders))
	for _, trader := range traders {
		if trader != nil {
			items[trader.Venue()] = trader
		}
	}
	return &DualVenueExecutionService{
		enabled: enabled, gateMarket: gateMarket, binanceMarket: binanceMarket,
		traders: items, inFlight: make(map[string]struct{}),
	}
}

func (service *DualVenueExecutionService) Execute(ctx context.Context, request ExecuteRequest) (ExecutionResult, error) {
	if !service.enabled {
		return ExecutionResult{Status: ExecutionFailed}, ErrLiveTradingDisabled
	}
	if !request.ConfirmLive {
		return ExecutionResult{Status: ExecutionFailed}, ErrLiveConfirmationRequired
	}
	if !contractPattern.MatchString(request.Contract) || request.TradeNotional < 5 || request.MinNetSpreadPct < 0 {
		return ExecutionResult{Status: ExecutionFailed}, ErrRouteChanged
	}
	if !service.beginExecution(request.Contract) {
		return ExecutionResult{Status: ExecutionFailed}, ErrExecutionInProgress
	}
	defer service.finishExecution(request.Contract)
	gate, gateErr := service.gateMarket.VenueMarket(ctx, request.Contract, 50)
	binance, binanceErr := service.binanceMarket.VenueMarket(ctx, request.Contract, 50)
	if gateErr != nil || binanceErr != nil {
		return ExecutionResult{Status: ExecutionFailed}, ErrRouteChanged
	}
	route := FindNeutralRoute(normalizeCrossRequest(request.TradeNotional, request.MinNetSpreadPct), gate, binance)
	result := ExecutionResult{Status: ExecutionFailed, Route: route, Fills: []OrderFill{}}
	if route.Status != NeutralRouteExecutable || route.LongVenue != request.ExpectedLongVenue || route.ShortVenue != request.ExpectedShortVenue {
		return result, ErrRouteChanged
	}
	longTrader, longOK := service.traders[route.LongVenue]
	shortTrader, shortOK := service.traders[route.ShortVenue]
	if !longOK || !shortOK {
		return result, ErrLiveTradingDisabled
	}
	if err := longTrader.Preflight(ctx, request.Contract, route.LongNotional); err != nil {
		result.ReasonCode = route.LongVenue + "_preflight_failed"
		return result, ErrPreflightFailed
	}
	if err := shortTrader.Preflight(ctx, request.Contract, route.ShortNotional); err != nil {
		result.ReasonCode = route.ShortVenue + "_preflight_failed"
		return result, ErrPreflightFailed
	}

	// Private account checks can take long enough for a thin futures spread to disappear.
	// Fetch both books once more immediately before constructing the market orders.
	gate, gateErr = service.gateMarket.VenueMarket(ctx, request.Contract, 50)
	binance, binanceErr = service.binanceMarket.VenueMarket(ctx, request.Contract, 50)
	if gateErr != nil || binanceErr != nil {
		return result, ErrRouteChanged
	}
	route = FindNeutralRoute(normalizeCrossRequest(request.TradeNotional, request.MinNetSpreadPct), gate, binance)
	result.Route = route
	if route.Status != NeutralRouteExecutable || route.LongVenue != request.ExpectedLongVenue || route.ShortVenue != request.ExpectedShortVenue {
		return result, ErrRouteChanged
	}

	longIntent := OrderIntent{Venue: route.LongVenue, Contract: request.Contract, Side: OrderBuy, Contracts: route.LongContracts}
	shortIntent := OrderIntent{Venue: route.ShortVenue, Contract: request.Contract, Side: OrderSell, Contracts: route.ShortContracts}
	type legResult struct {
		intent OrderIntent
		trader venueTrader
		fill   OrderFill
		err    error
	}
	results := make(chan legResult, 2)
	var wait sync.WaitGroup
	for _, item := range []struct {
		intent OrderIntent
		trader venueTrader
	}{{longIntent, longTrader}, {shortIntent, shortTrader}} {
		wait.Add(1)
		go func(intent OrderIntent, trader venueTrader) {
			defer wait.Done()
			fill, err := trader.PlaceMarket(ctx, intent)
			results <- legResult{intent: intent, trader: trader, fill: fill, err: err}
		}(item.intent, item.trader)
	}
	wait.Wait()
	close(results)
	legs := make([]legResult, 0, 2)
	for leg := range results {
		legs = append(legs, leg)
	}
	succeeded := make([]legResult, 0, 2)
	failed := false
	for _, leg := range legs {
		if leg.err != nil || !finitePositive(leg.fill.FilledContracts) {
			failed = true
			continue
		}
		if leg.fill.FilledContracts+1e-9 < leg.intent.Contracts {
			failed = true
		}
		succeeded = append(succeeded, leg)
		result.Fills = append(result.Fills, leg.fill)
	}
	if !failed && len(succeeded) == 2 {
		result.Status = ExecutionOpened
		return result, nil
	}
	if len(succeeded) == 0 {
		return result, ErrLegExecutionFailed
	}
	compensations := make(chan legResult, len(succeeded))
	wait = sync.WaitGroup{}
	for _, opened := range succeeded {
		wait.Add(1)
		go func(opened legResult) {
			defer wait.Done()
			closeSide := OrderSell
			if opened.intent.Side == OrderSell {
				closeSide = OrderBuy
			}
			closeIntent := OrderIntent{
				Venue: opened.intent.Venue, Contract: opened.intent.Contract, Side: closeSide,
				Contracts: opened.fill.FilledContracts, ReduceOnly: true,
			}
			fill, err := opened.trader.CloseFilled(ctx, closeIntent, opened.fill)
			compensations <- legResult{intent: closeIntent, trader: opened.trader, fill: fill, err: err}
		}(opened)
	}
	wait.Wait()
	close(compensations)
	compensationFailed := false
	for closed := range compensations {
		if closed.err != nil || !finitePositive(closed.fill.FilledContracts) {
			compensationFailed = true
			continue
		}
		result.Compensations = append(result.Compensations, closed.fill)
	}
	if compensationFailed || len(result.Compensations) != len(succeeded) {
		result.Status = ExecutionExposed
		return result, errors.Join(ErrLegExecutionFailed, ErrCompensationFailed)
	}
	result.Status = ExecutionCompensated
	if len(result.Compensations) == 1 {
		result.Compensation = &result.Compensations[0]
	}
	return result, ErrLegExecutionFailed
}

func (service *DualVenueExecutionService) beginExecution(contract string) bool {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if _, exists := service.inFlight[contract]; exists {
		return false
	}
	service.inFlight[contract] = struct{}{}
	return true
}

func (service *DualVenueExecutionService) finishExecution(contract string) {
	service.mutex.Lock()
	delete(service.inFlight, contract)
	service.mutex.Unlock()
}
