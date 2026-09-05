package futures

import (
	"context"
	"errors"
	"math"
	"sync"
)

const maxOrderProbeNotional = 10.0

var (
	ErrInsufficientMargin   = errors.New("insufficient futures margin")
	ErrExistingPosition     = errors.New("existing futures position")
	ErrTradeNotAllowed      = errors.New("futures trade permission unavailable")
	ErrOrderProbeFailed     = errors.New("futures order probe failed")
	ErrOrderProbeExposed    = errors.New("futures order probe left exposure")
	ErrOrderProbeInProgress = errors.New("futures order probe already in progress")
)

type AccessRequest struct {
	Contract         string  `json:"contract"`
	RequiredNotional float64 `json:"required_notional"`
}

type VenueAccessReport struct {
	Venue      string `json:"venue"`
	Ready      bool   `json:"ready"`
	ReasonCode string `json:"reason_code,omitempty"`
}

type AccessReport struct {
	Contract         string              `json:"contract"`
	RequiredNotional float64             `json:"required_notional"`
	Venues           []VenueAccessReport `json:"venues"`
}

type OrderProbeStatus string

const (
	OrderProbeCancelled   OrderProbeStatus = "cancelled"
	OrderProbeCompensated OrderProbeStatus = "compensated"
	OrderProbeExposed     OrderProbeStatus = "exposed"
	OrderProbeFailed      OrderProbeStatus = "failed"
)

type OrderProbeRequest struct {
	Venue       string `json:"venue"`
	Contract    string `json:"contract"`
	ConfirmLive bool   `json:"confirm_live"`
}

type OrderProbePlan struct {
	Venue     string    `json:"venue"`
	Contract  string    `json:"contract"`
	Side      OrderSide `json:"side"`
	Price     float64   `json:"price"`
	Contracts float64   `json:"contracts"`
	Notional  float64   `json:"notional"`
}

type OrderProbeResult struct {
	Venue           string           `json:"venue"`
	Contract        string           `json:"contract"`
	Status          OrderProbeStatus `json:"status"`
	OrderID         string           `json:"order_id,omitempty"`
	Side            OrderSide        `json:"side"`
	Price           float64          `json:"price"`
	Contracts       float64          `json:"contracts"`
	Notional        float64          `json:"notional"`
	FilledContracts float64          `json:"filled_contracts"`
	CleanupVerified bool             `json:"cleanup_verified"`
	ReasonCode      string           `json:"reason_code,omitempty"`
}

type AccessService interface {
	Access(context.Context, AccessRequest) (AccessReport, error)
	Probe(context.Context, OrderProbeRequest) (OrderProbeResult, error)
}

type orderProbeTrader interface {
	Venue() string
	Preflight(context.Context, string, float64) error
	ProbePostOnly(context.Context, OrderProbePlan) (OrderProbeResult, error)
}

type DualVenueAccessService struct {
	enabled   bool
	providers map[string]venueMarketProvider
	traders   map[string]orderProbeTrader
	venues    []string
	mutex     sync.Mutex
	inFlight  map[string]struct{}
}

func NewAccessService(enabled bool, gateMarket, binanceMarket venueMarketProvider, traders ...orderProbeTrader) *DualVenueAccessService {
	service := &DualVenueAccessService{
		enabled: enabled,
		providers: map[string]venueMarketProvider{
			"gate_futures": gateMarket, "binance_futures": binanceMarket,
		},
		traders:  make(map[string]orderProbeTrader),
		venues:   []string{"gate_futures", "binance_futures"},
		inFlight: make(map[string]struct{}),
	}
	for _, trader := range traders {
		if trader != nil {
			service.traders[trader.Venue()] = trader
		}
	}
	return service
}

func (service *DualVenueAccessService) Access(ctx context.Context, request AccessRequest) (AccessReport, error) {
	if !contractPattern.MatchString(request.Contract) || !finitePositive(request.RequiredNotional) {
		return AccessReport{}, errors.New("invalid futures access request")
	}
	report := AccessReport{Contract: request.Contract, RequiredNotional: request.RequiredNotional, Venues: make([]VenueAccessReport, 0, len(service.venues))}
	for _, venue := range service.venues {
		item := VenueAccessReport{Venue: venue}
		trader, ok := service.traders[venue]
		if !ok {
			item.ReasonCode = "credentials_unavailable"
		} else if err := trader.Preflight(ctx, request.Contract, request.RequiredNotional); err != nil {
			item.ReasonCode = accessReasonCode(err)
		} else {
			item.Ready = true
		}
		report.Venues = append(report.Venues, item)
	}
	return report, nil
}

func (service *DualVenueAccessService) Probe(ctx context.Context, request OrderProbeRequest) (OrderProbeResult, error) {
	if !service.enabled {
		return OrderProbeResult{Status: OrderProbeFailed}, ErrLiveTradingDisabled
	}
	if !request.ConfirmLive {
		return OrderProbeResult{Status: OrderProbeFailed}, ErrLiveConfirmationRequired
	}
	if !contractPattern.MatchString(request.Contract) {
		return OrderProbeResult{Status: OrderProbeFailed}, ErrOrderProbeFailed
	}
	provider, providerOK := service.providers[request.Venue]
	trader, traderOK := service.traders[request.Venue]
	if !providerOK || !traderOK {
		return OrderProbeResult{Status: OrderProbeFailed}, ErrLiveTradingDisabled
	}
	probeKey := request.Venue + ":" + request.Contract
	if !service.beginProbe(probeKey) {
		return OrderProbeResult{Venue: request.Venue, Contract: request.Contract, Status: OrderProbeFailed}, ErrOrderProbeInProgress
	}
	defer service.finishProbe(probeKey)
	market, err := provider.VenueMarket(ctx, request.Contract, 5)
	if err != nil {
		return OrderProbeResult{Status: OrderProbeFailed}, ErrOrderProbeFailed
	}
	plan, err := buildPostOnlyProbePlan(request.Contract, market)
	if err != nil {
		return OrderProbeResult{Status: OrderProbeFailed}, ErrOrderProbeFailed
	}
	if err := trader.Preflight(ctx, request.Contract, plan.Notional); err != nil {
		return OrderProbeResult{Venue: request.Venue, Contract: request.Contract, Status: OrderProbeFailed, ReasonCode: accessReasonCode(err)}, ErrPreflightFailed
	}
	result, err := trader.ProbePostOnly(ctx, plan)
	if result.Venue == "" {
		result.Venue = plan.Venue
	}
	if result.Contract == "" {
		result.Contract = plan.Contract
	}
	result.Side, result.Price, result.Contracts, result.Notional = plan.Side, plan.Price, plan.Contracts, plan.Notional
	if err != nil {
		return result, err
	}
	if !result.CleanupVerified {
		result.Status = OrderProbeExposed
		return result, ErrOrderProbeExposed
	}
	return result, nil
}

func (service *DualVenueAccessService) beginProbe(key string) bool {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if _, exists := service.inFlight[key]; exists {
		return false
	}
	service.inFlight[key] = struct{}{}
	return true
}

func (service *DualVenueAccessService) finishProbe(key string) {
	service.mutex.Lock()
	delete(service.inFlight, key)
	service.mutex.Unlock()
}

func accessReasonCode(err error) string {
	switch {
	case errors.Is(err, ErrInsufficientMargin):
		return "insufficient_margin"
	case errors.Is(err, ErrExistingPosition):
		return "existing_position"
	case errors.Is(err, ErrTradeNotAllowed):
		return "trade_permission_unavailable"
	case errors.Is(err, ErrLiveTradingDisabled):
		return "credentials_unavailable"
	default:
		return "private_access_failed"
	}
}

func buildPostOnlyProbePlan(contract string, market VenueMarket) (OrderProbePlan, error) {
	if !contractPattern.MatchString(contract) || market.Venue == "" || len(market.Book.Bids) == 0 ||
		!finitePositive(market.Book.Bids[0].Price) || !finitePositive(market.ContractMultiplier) ||
		!finitePositive(market.QuantityStep) || !finitePositive(market.MinQuantity) || !finitePositive(market.PriceIncrement) {
		return OrderProbePlan{}, errors.New("incomplete futures probe market")
	}
	price := math.Floor((market.Book.Bids[0].Price*0.98)/market.PriceIncrement+1e-10) * market.PriceIncrement
	if !finitePositive(price) || price >= market.Book.Bids[0].Price {
		return OrderProbePlan{}, errors.New("invalid futures probe price")
	}
	contracts := market.MinQuantity
	if market.MinNotional > 0 {
		requiredContracts := market.MinNotional / (price * market.ContractMultiplier)
		contracts = math.Max(contracts, math.Ceil((requiredContracts-1e-12)/market.QuantityStep)*market.QuantityStep)
	}
	contracts = math.Ceil((contracts-1e-12)/market.QuantityStep) * market.QuantityStep
	notional := contracts * market.ContractMultiplier * price
	if !finitePositive(contracts) || !finitePositive(notional) || notional > maxOrderProbeNotional {
		return OrderProbePlan{}, errors.New("minimum futures probe exceeds safety cap")
	}
	return OrderProbePlan{
		Venue: market.Venue, Contract: contract, Side: OrderBuy,
		Price: price, Contracts: contracts, Notional: notional,
	}, nil
}
