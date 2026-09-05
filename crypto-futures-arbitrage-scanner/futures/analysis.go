package futures

import (
	"errors"
	"math"
	"regexp"
	"sort"
	"time"
)

var contractPattern = regexp.MustCompile(`^[A-Z0-9]{1,24}_USDT$`)

func ValidateAnalyzeRequest(request AnalyzeRequest) error {
	if !contractPattern.MatchString(request.Contract) {
		return errors.New("invalid futures contract")
	}
	switch request.Interval {
	case "15m", "1h", "4h":
	default:
		return errors.New("invalid futures interval")
	}
	switch request.Strategy {
	case StrategyTrendPullback, StrategyBreakout, StrategyMeanReversion:
	default:
		return errors.New("invalid futures strategy")
	}
	if !finitePositive(request.AccountBalance) || request.AccountBalance < 10 || request.AccountBalance > 100_000_000 {
		return errors.New("account balance must be between 10 and 100000000")
	}
	if !finitePositive(request.RiskPercent) || request.RiskPercent > 5 {
		return errors.New("risk percent must be greater than 0 and at most 5")
	}
	if request.TradeNotional != 0 && (!finitePositive(request.TradeNotional) || request.TradeNotional < 5 || request.TradeNotional > 1_000_000) {
		return errors.New("trade notional must be between 5 and 1000000")
	}
	if request.MinNetSpreadPct < 0 || math.IsNaN(request.MinNetSpreadPct) || math.IsInf(request.MinNetSpreadPct, 0) || request.MinNetSpreadPct > 100 {
		return errors.New("invalid minimum net spread")
	}
	return nil
}

func validateAnalyzeRequest(request AnalyzeRequest) error {
	return ValidateAnalyzeRequest(request)
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func noTradeScenario(direction, reason string) Scenario {
	return Scenario{Direction: direction, Status: ScenarioNoTrade, Targets: []float64{}, Reasons: []string{reason}}
}

func AnalyzeMarket(request AnalyzeRequest, contract Contract, candles []Candle, book OrderBook, now time.Time) (Analysis, error) {
	if err := validateAnalyzeRequest(request); err != nil {
		return Analysis{}, err
	}
	candles = closedCandles(candles, request.Interval, now)
	result := Analysis{
		Contract: contract, Interval: request.Interval, Strategy: request.Strategy,
		AnalyzedAt: now.UnixMilli(), Candles: append([]Candle(nil), candles...),
		Long:  noTradeScenario("long", "At least 50 closed candles are required for analysis."),
		Short: noTradeScenario("short", "At least 50 closed candles are required for analysis."),
	}
	if len(candles) < 50 {
		return result, nil
	}
	for _, candle := range candles {
		if candle.Time <= 0 || !finitePositive(candle.Open) || !finitePositive(candle.High) ||
			!finitePositive(candle.Low) || !finitePositive(candle.Close) || candle.High < candle.Low {
			return Analysis{}, errors.New("invalid candle data")
		}
	}

	closes := make([]float64, len(candles))
	for index, candle := range candles {
		closes[index] = candle.Close
	}
	ema20 := ema(closes, 20)
	ema50 := ema(closes, 50)
	atr14 := atr(candles, 14)
	rsi14 := rsi(closes, 14)
	mean20, deviation20 := meanDeviation(closes[len(closes)-20:])
	swingHigh, swingLow := recentRange(candles, 20)
	bias := "neutral"
	if ema20 > ema50 {
		bias = "bullish"
	} else if ema20 < ema50 {
		bias = "bearish"
	}
	result.Indicators = Indicators{
		EMA20: ema20, EMA50: ema50, ATR14: atr14, RSI14: rsi14, Mean20: mean20,
		UpperBand: mean20 + 2*deviation20, LowerBand: mean20 - 2*deviation20,
		SwingHigh: swingHigh, SwingLow: swingLow, MarketBias: bias,
	}
	result.Liquidity = liquidityWalls(book, contract.QuantoMultiplier)
	current := contract.MarkPrice
	if !finitePositive(current) {
		current = closes[len(closes)-1]
	}

	result.Long = buildScenario("long", request, contract, current, result.Indicators, result.Liquidity)
	result.Short = buildScenario("short", request, contract, current, result.Indicators, result.Liquidity)
	return result, nil
}

func closedCandles(candles []Candle, interval string, now time.Time) []Candle {
	seconds := map[string]int64{"15m": 15 * 60, "1h": 60 * 60, "4h": 4 * 60 * 60}[interval]
	items := make([]Candle, 0, len(candles))
	for _, candle := range candles {
		if candle.Time+seconds <= now.Unix() {
			items = append(items, candle)
		}
	}
	sort.Slice(items, func(left, right int) bool { return items[left].Time < items[right].Time })
	return items
}

func buildScenario(direction string, request AnalyzeRequest, contract Contract, current float64, indicators Indicators, liquidity Liquidity) Scenario {
	entry, stop, score, reasons := scenarioLevels(direction, request.Strategy, current, indicators)
	if !finitePositive(entry) || !finitePositive(stop) || entry == stop {
		return noTradeScenario(direction, "The selected strategy could not produce valid entry and stop levels.")
	}
	riskDistance := math.Abs(entry - stop)
	if direction == "long" && stop >= entry || direction == "short" && stop <= entry {
		return noTradeScenario(direction, "The invalidation level is on the wrong side of entry.")
	}
	target := liquidityTarget(direction, entry, riskDistance, liquidity)
	if !finitePositive(target) {
		if direction == "long" {
			target = entry + 2*riskDistance
		} else {
			target = entry - 2*riskDistance
		}
	}
	riskReward := math.Abs(target-entry) / riskDistance
	status := ScenarioNoTrade
	if score >= 65 && riskReward >= 1.5 {
		status = ScenarioReady
	} else if score >= 40 && riskReward >= 1.2 {
		status = ScenarioWatch
	}
	if riskReward < 1.5 {
		reasons = append(reasons, "The first usable liquidity target offers less than 1.5R.")
	}
	if status == ScenarioNoTrade {
		reasons = append(reasons, "Strategy confluence is not strong enough for a paper entry.")
	}

	riskAmount := request.AccountBalance * request.RiskPercent / 100
	baseQuantity := riskAmount / riskDistance
	contracts := 0.0
	if finitePositive(contract.QuantoMultiplier) {
		contracts = math.Floor(baseQuantity / contract.QuantoMultiplier)
	}
	if contract.OrderSizeMax > 0 {
		contracts = math.Min(contracts, contract.OrderSizeMax)
	}
	if contracts < contract.OrderSizeMin {
		status = ScenarioNoTrade
		reasons = append(reasons, "Calculated size is below the contract minimum.")
		contracts = 0
	}
	baseQuantity = contracts * contract.QuantoMultiplier
	notional := baseQuantity * entry
	fees := notional * math.Max(contract.TakerFeeRate, 0) * 2

	return Scenario{
		Direction: direction, Status: status, Score: score, Entry: entry, Stop: stop,
		Targets: []float64{target}, RiskReward: riskReward, RiskAmount: riskAmount,
		Contracts: contracts, BaseQuantity: baseQuantity, Notional: notional,
		EstimatedFees:   fees,
		LiquidationNote: "Liquidation price depends on leverage, margin mode, maintenance tier, and existing positions; it is intentionally not estimated here.",
		Reasons:         reasons,
	}
}

func scenarioLevels(direction, strategy string, current float64, indicators Indicators) (float64, float64, int, []string) {
	long := direction == "long"
	score := 0
	reasons := make([]string, 0, 4)
	entry := current
	stop := current

	switch strategy {
	case StrategyTrendPullback:
		entry = indicators.EMA20
		if long {
			stop = math.Max(indicators.SwingLow, entry-1.25*indicators.ATR14)
			if indicators.EMA20 > indicators.EMA50 {
				score += 45
				reasons = append(reasons, "EMA20 is above EMA50.")
			}
			if current >= indicators.EMA20 {
				score += 20
				reasons = append(reasons, "Mark price holds above the pullback mean.")
			}
			if indicators.RSI14 >= 40 && indicators.RSI14 <= 78 {
				score += 20
				reasons = append(reasons, "RSI supports continuation without an extreme reading.")
			}
		} else {
			stop = math.Min(indicators.SwingHigh, entry+1.25*indicators.ATR14)
			if indicators.EMA20 < indicators.EMA50 {
				score += 45
				reasons = append(reasons, "EMA20 is below EMA50.")
			}
			if current <= indicators.EMA20 {
				score += 20
				reasons = append(reasons, "Mark price holds below the pullback mean.")
			}
			if indicators.RSI14 >= 22 && indicators.RSI14 <= 60 {
				score += 20
				reasons = append(reasons, "RSI supports continuation without an extreme reading.")
			}
		}
	case StrategyBreakout:
		if long {
			entry = indicators.SwingHigh + 0.1*indicators.ATR14
			stop = indicators.SwingHigh - indicators.ATR14
			if current >= indicators.SwingHigh-indicators.ATR14 {
				score += 45
				reasons = append(reasons, "Price is pressing the recent range high.")
			}
			if indicators.EMA20 >= indicators.EMA50 {
				score += 30
				reasons = append(reasons, "Trend structure supports an upside break.")
			}
		} else {
			entry = indicators.SwingLow - 0.1*indicators.ATR14
			stop = indicators.SwingLow + indicators.ATR14
			if current <= indicators.SwingLow+indicators.ATR14 {
				score += 45
				reasons = append(reasons, "Price is pressing the recent range low.")
			}
			if indicators.EMA20 <= indicators.EMA50 {
				score += 30
				reasons = append(reasons, "Trend structure supports a downside break.")
			}
		}
	case StrategyMeanReversion:
		if long {
			entry = math.Min(current, indicators.LowerBand)
			stop = entry - 1.25*indicators.ATR14
			if current <= indicators.LowerBand+0.35*indicators.ATR14 {
				score += 45
				reasons = append(reasons, "Price is testing the lower volatility band.")
			}
			if indicators.RSI14 <= 40 {
				score += 35
				reasons = append(reasons, "RSI shows downside exhaustion.")
			}
		} else {
			entry = math.Max(current, indicators.UpperBand)
			stop = entry + 1.25*indicators.ATR14
			if current >= indicators.UpperBand-0.35*indicators.ATR14 {
				score += 45
				reasons = append(reasons, "Price is testing the upper volatility band.")
			}
			if indicators.RSI14 >= 60 {
				score += 35
				reasons = append(reasons, "RSI shows upside exhaustion.")
			}
		}
	}
	return entry, stop, score, reasons
}

func liquidityTarget(direction string, entry, riskDistance float64, liquidity Liquidity) float64 {
	levels := liquidity.AskWalls
	if direction == "short" {
		levels = liquidity.BidWalls
	}
	candidates := make([]float64, 0, len(levels))
	for _, level := range levels {
		reward := level.Price - entry
		if direction == "short" {
			reward = entry - level.Price
		}
		if reward/riskDistance >= 1.5 {
			candidates = append(candidates, level.Price)
		}
	}
	if len(candidates) == 0 {
		return 0
	}
	if direction == "long" {
		sort.Float64s(candidates)
		return candidates[0]
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(candidates)))
	return candidates[0]
}

func liquidityWalls(book OrderBook, multiplier float64) Liquidity {
	decorate := func(levels []BookLevel) []BookLevel {
		items := make([]BookLevel, 0, len(levels))
		for _, level := range levels {
			if !finitePositive(level.Price) || !finitePositive(math.Abs(level.Size)) {
				continue
			}
			level.Size = math.Abs(level.Size)
			level.Notional = level.Price * level.Size * multiplier
			items = append(items, level)
		}
		sort.Slice(items, func(left, right int) bool { return items[left].Notional > items[right].Notional })
		if len(items) > 5 {
			items = items[:5]
		}
		return items
	}
	return Liquidity{BidWalls: decorate(book.Bids), AskWalls: decorate(book.Asks)}
}

func ema(values []float64, period int) float64 {
	value := values[0]
	multiplier := 2.0 / float64(period+1)
	for _, item := range values[1:] {
		value = (item-value)*multiplier + value
	}
	return value
}

func atr(candles []Candle, period int) float64 {
	start := len(candles) - period
	total := 0.0
	for index := start; index < len(candles); index++ {
		previousClose := candles[index-1].Close
		rangeValue := math.Max(candles[index].High-candles[index].Low,
			math.Max(math.Abs(candles[index].High-previousClose), math.Abs(candles[index].Low-previousClose)))
		total += rangeValue
	}
	return total / float64(period)
}

func rsi(values []float64, period int) float64 {
	gains, losses := 0.0, 0.0
	for index := len(values) - period; index < len(values); index++ {
		change := values[index] - values[index-1]
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}
	if losses == 0 {
		return 100
	}
	rs := (gains / float64(period)) / (losses / float64(period))
	return 100 - 100/(1+rs)
}

func meanDeviation(values []float64) (float64, float64) {
	total := 0.0
	for _, value := range values {
		total += value
	}
	mean := total / float64(len(values))
	variance := 0.0
	for _, value := range values {
		variance += math.Pow(value-mean, 2)
	}
	return mean, math.Sqrt(variance / float64(len(values)))
}

func recentRange(candles []Candle, period int) (float64, float64) {
	items := candles[len(candles)-period:]
	high, low := items[0].High, items[0].Low
	for _, candle := range items[1:] {
		high = math.Max(high, candle.High)
		low = math.Min(low, candle.Low)
	}
	return high, low
}
