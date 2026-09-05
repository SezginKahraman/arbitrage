package futures

import (
	"math"
	"sort"
)

type NeutralRouteStatus string

const (
	NeutralRouteExecutable NeutralRouteStatus = "executable"
	NeutralRouteWatch      NeutralRouteStatus = "watch"
	NeutralRouteRejected   NeutralRouteStatus = "rejected"
)

type CrossVenueRequest struct {
	Notional              float64 `json:"notional"`
	MinNetSpreadPct       float64 `json:"min_net_spread_pct"`
	MaxIndexDivergencePct float64 `json:"max_index_divergence_pct"`
}

type VenueMarket struct {
	Venue              string    `json:"venue"`
	Symbol             string    `json:"symbol"`
	IndexPrice         float64   `json:"index_price"`
	MarkPrice          float64   `json:"mark_price"`
	ContractMultiplier float64   `json:"contract_multiplier"`
	QuantityStep       float64   `json:"quantity_step"`
	MinQuantity        float64   `json:"min_quantity"`
	PriceIncrement     float64   `json:"price_increment"`
	MinNotional        float64   `json:"min_notional"`
	TakerFeeRate       float64   `json:"taker_fee_rate"`
	FundingRate        float64   `json:"funding_rate"`
	NextFundingAt      int64     `json:"next_funding_at"`
	Book               OrderBook `json:"book"`
}

type NeutralRoute struct {
	Status                  NeutralRouteStatus `json:"status"`
	ReasonCode              string             `json:"reason_code"`
	Reasons                 []string           `json:"reasons"`
	Symbol                  string             `json:"symbol"`
	LongVenue               string             `json:"long_venue"`
	ShortVenue              string             `json:"short_venue"`
	LongEntry               float64            `json:"long_entry"`
	ShortEntry              float64            `json:"short_entry"`
	BaseQuantity            float64            `json:"base_quantity"`
	LongContracts           float64            `json:"long_contracts"`
	ShortContracts          float64            `json:"short_contracts"`
	LongNotional            float64            `json:"long_notional"`
	ShortNotional           float64            `json:"short_notional"`
	GrossSpreadPct          float64            `json:"gross_spread_pct"`
	OpenFees                float64            `json:"open_fees"`
	EstimatedCloseFees      float64            `json:"estimated_close_fees"`
	NetConvergenceSpreadPct float64            `json:"net_convergence_spread_pct"`
	NextFundingCarryPct     float64            `json:"next_funding_carry_pct"`
	IndexDivergencePct      float64            `json:"index_divergence_pct"`
}

func FindNeutralRoute(request CrossVenueRequest, left, right VenueMarket) NeutralRoute {
	base := NeutralRoute{Symbol: left.Symbol}
	if left.Symbol == "" || left.Symbol != right.Symbol {
		base.Status = NeutralRouteRejected
		base.ReasonCode = "contract_identity_unverified"
		base.Reasons = []string{"Venue symbols do not identify the same contract."}
		return base
	}
	if !finitePositive(left.IndexPrice) || !finitePositive(right.IndexPrice) ||
		!finitePositive(request.Notional) || !finitePositive(request.MaxIndexDivergencePct) {
		base.Status = NeutralRouteRejected
		base.ReasonCode = "invalid_market_data"
		base.Reasons = []string{"Cross-venue market metadata is incomplete."}
		return base
	}
	base.IndexDivergencePct = math.Abs(left.IndexPrice-right.IndexPrice) / math.Min(left.IndexPrice, right.IndexPrice) * 100
	if base.IndexDivergencePct > request.MaxIndexDivergencePct {
		base.Status = NeutralRouteRejected
		base.ReasonCode = "contract_identity_unverified"
		base.Reasons = []string{"Venue index prices diverge beyond the contract identity limit."}
		return base
	}

	candidates := []NeutralRoute{
		neutralCandidate(request, left, right, base.IndexDivergencePct),
		neutralCandidate(request, right, left, base.IndexDivergencePct),
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].NetConvergenceSpreadPct > candidates[j].NetConvergenceSpreadPct
	})
	if candidates[0].ReasonCode == "insufficient_depth" && candidates[1].ReasonCode != "insufficient_depth" {
		return candidates[1]
	}
	return candidates[0]
}

func neutralCandidate(request CrossVenueRequest, longMarket, shortMarket VenueMarket, indexDivergence float64) NeutralRoute {
	route := NeutralRoute{
		Status: NeutralRouteRejected, ReasonCode: "insufficient_depth", Symbol: longMarket.Symbol,
		LongVenue: longMarket.Venue, ShortVenue: shortMarket.Venue, IndexDivergencePct: indexDivergence,
		Reasons: []string{"The requested notional cannot be matched across both order books."},
	}
	if len(longMarket.Book.Asks) == 0 || len(shortMarket.Book.Bids) == 0 ||
		!finitePositive(longMarket.ContractMultiplier) || !finitePositive(shortMarket.ContractMultiplier) ||
		!finitePositive(longMarket.QuantityStep) || !finitePositive(shortMarket.QuantityStep) {
		return route
	}
	requestedBase := request.Notional / longMarket.Book.Asks[0].Price
	baseQuantity := matchedBaseQuantity(requestedBase, longMarket, shortMarket)
	if !finitePositive(baseQuantity) {
		return route
	}
	longNotional, longEntry, longOK := fillBook(longMarket.Book.Asks, baseQuantity, longMarket.ContractMultiplier)
	shortNotional, shortEntry, shortOK := fillBook(shortMarket.Book.Bids, baseQuantity, shortMarket.ContractMultiplier)
	if !longOK || !shortOK || longNotional+1e-9 < request.Notional*0.99 {
		return route
	}

	openFees := longNotional*longMarket.TakerFeeRate + shortNotional*shortMarket.TakerFeeRate
	convergencePrice := (longEntry + shortEntry) / 2
	closeFees := baseQuantity * convergencePrice * (longMarket.TakerFeeRate + shortMarket.TakerFeeRate)
	grossProfit := shortNotional - longNotional
	netProfit := grossProfit - openFees - closeFees
	route.LongEntry = longEntry
	route.ShortEntry = shortEntry
	route.BaseQuantity = baseQuantity
	route.LongContracts = baseQuantity / longMarket.ContractMultiplier
	route.ShortContracts = baseQuantity / shortMarket.ContractMultiplier
	route.LongNotional = longNotional
	route.ShortNotional = shortNotional
	route.GrossSpreadPct = grossProfit / longNotional * 100
	route.OpenFees = openFees
	route.EstimatedCloseFees = closeFees
	route.NetConvergenceSpreadPct = netProfit / longNotional * 100
	route.NextFundingCarryPct = (shortMarket.FundingRate - longMarket.FundingRate) * 100
	if route.NetConvergenceSpreadPct >= request.MinNetSpreadPct {
		route.Status = NeutralRouteExecutable
		route.ReasonCode = ""
		route.Reasons = []string{"Executable entry spread remains positive after estimated four-leg taker fees."}
	} else {
		route.Status = NeutralRouteWatch
		route.ReasonCode = "below_net_threshold"
		route.Reasons = []string{"Estimated convergence return is below the selected net threshold."}
	}
	return route
}

func matchedBaseQuantity(target float64, left, right VenueMarket) float64 {
	leftNative := math.Floor(target/left.ContractMultiplier/left.QuantityStep) * left.QuantityStep
	leftBase := leftNative * left.ContractMultiplier
	rightNative := math.Floor(leftBase/right.ContractMultiplier/right.QuantityStep) * right.QuantityStep
	rightBase := rightNative * right.ContractMultiplier
	leftNative = math.Floor(rightBase/left.ContractMultiplier/left.QuantityStep) * left.QuantityStep
	base := leftNative * left.ContractMultiplier
	if leftNative+1e-12 < left.MinQuantity || rightNative+1e-12 < right.MinQuantity {
		return 0
	}
	return base
}

func fillBook(levels []BookLevel, baseQuantity, multiplier float64) (float64, float64, bool) {
	remaining := baseQuantity
	total := 0.0
	for _, level := range levels {
		availableBase := math.Abs(level.Size) * multiplier
		if !finitePositive(level.Price) || !finitePositive(availableBase) {
			continue
		}
		filled := math.Min(remaining, availableBase)
		total += filled * level.Price
		remaining -= filled
		if remaining <= 1e-9 {
			return total, total / baseQuantity, true
		}
	}
	return total, 0, false
}
