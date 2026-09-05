package futures

import (
	"context"
	"errors"
	"time"
)

const defaultCrossVenueNotional = 100
const defaultCrossVenueNetSpreadPct = 0.10
const defaultContractIdentityDivergencePct = 3

type venueMarketProvider interface {
	VenueMarket(context.Context, string, int) (VenueMarket, error)
}

type gateWorkspaceProvider interface {
	Service
	venueMarketProvider
	LatestCandle(context.Context, string, string) (Candle, error)
}

type WorkspaceService struct {
	gate    gateWorkspaceProvider
	binance venueMarketProvider
	clock   func() time.Time
}

func NewWorkspaceService(gate gateWorkspaceProvider, binance venueMarketProvider) *WorkspaceService {
	return &WorkspaceService{gate: gate, binance: binance, clock: time.Now}
}

func (service *WorkspaceService) Contracts(ctx context.Context) ([]Contract, error) {
	return service.gate.Contracts(ctx)
}

func normalizeCrossRequest(notional, minNet float64) CrossVenueRequest {
	if !finitePositive(notional) {
		notional = defaultCrossVenueNotional
	}
	if minNet < 0 {
		minNet = defaultCrossVenueNetSpreadPct
	}
	return CrossVenueRequest{
		Notional: notional, MinNetSpreadPct: minNet,
		MaxIndexDivergencePct: defaultContractIdentityDivergencePct,
	}
}

func unavailableNeutralRoute(symbol, reasonCode, reason string) NeutralRoute {
	return NeutralRoute{
		Status: NeutralRouteRejected, Symbol: symbol, ReasonCode: reasonCode,
		Reasons: []string{reason},
	}
}

func (service *WorkspaceService) Analyze(ctx context.Context, request AnalyzeRequest) (Analysis, error) {
	analysis, err := service.gate.Analyze(ctx, request)
	if err != nil {
		return Analysis{}, err
	}
	gateMarket, err := service.gate.VenueMarket(ctx, request.Contract, 50)
	if err != nil {
		return Analysis{}, err
	}
	binanceMarket, binanceErr := service.binance.VenueMarket(ctx, request.Contract, 50)
	if binanceErr != nil {
		route := unavailableNeutralRoute(gateMarket.Symbol, "binance_market_unavailable", "The matching Binance USDT perpetual market is unavailable.")
		analysis.NeutralRoute = &route
		return analysis, nil
	}
	route := FindNeutralRoute(normalizeCrossRequest(request.TradeNotional, request.MinNetSpreadPct), gateMarket, binanceMarket)
	analysis.NeutralRoute = &route
	return analysis, nil
}

func (service *WorkspaceService) Live(ctx context.Context, request LiveRequest) (LiveSnapshot, error) {
	if !contractPattern.MatchString(request.Contract) || !validInterval(request.Interval) ||
		!finitePositive(request.TradeNotional) || request.TradeNotional < 5 || request.MinNetSpreadPct < 0 {
		return LiveSnapshot{}, errors.New("invalid futures live request")
	}
	gateMarket, err := service.gate.VenueMarket(ctx, request.Contract, 20)
	if err != nil {
		return LiveSnapshot{}, err
	}
	candle, err := service.gate.LatestCandle(ctx, request.Contract, request.Interval)
	if err != nil {
		return LiveSnapshot{}, err
	}
	binanceMarket, binanceErr := service.binance.VenueMarket(ctx, request.Contract, 20)
	route := unavailableNeutralRoute(gateMarket.Symbol, "binance_market_unavailable", "The matching Binance USDT perpetual market is unavailable.")
	if binanceErr == nil {
		route = FindNeutralRoute(normalizeCrossRequest(request.TradeNotional, request.MinNetSpreadPct), gateMarket, binanceMarket)
	}
	return LiveSnapshot{
		CapturedAt: service.clock().UnixMilli(), Candle: candle, Gate: gateMarket, Binance: binanceMarket, Route: route,
	}, nil
}
