package futures

import (
	"strings"
	"testing"
	"time"
)

func trendingCandles(count int) []Candle {
	items := make([]Candle, 0, count)
	price := 100.0
	for index := 0; index < count; index++ {
		open := price
		price += 0.35
		items = append(items, Candle{
			Time: int64(1_700_000_000 + index*900), Open: open, High: price + 0.45,
			Low: open - 0.30, Close: price, Volume: 1_000 + float64(index*10),
		})
	}
	return items
}

func analysisFixture() (AnalyzeRequest, Contract, []Candle, OrderBook) {
	candles := trendingCandles(80)
	last := candles[len(candles)-1].Close
	return AnalyzeRequest{
			Contract: "BTC_USDT", Interval: "15m", Strategy: "trend_pullback",
			AccountBalance: 1_000, RiskPercent: 1,
		}, Contract{
			Symbol: "BTC_USDT", MarkPrice: last, IndexPrice: last - 0.05,
			QuantoMultiplier: 0.001, OrderSizeMin: 1, TakerFeeRate: 0.00075,
			FundingRate: 0.0001, LeverageMin: 1, LeverageMax: 100,
		}, candles, OrderBook{
			UpdatedAt: 1_700_100_000,
			Bids:      []BookLevel{{Price: last - 0.1, Size: 80}, {Price: last - 1.4, Size: 4_000}},
			Asks:      []BookLevel{{Price: last + 0.1, Size: 75}, {Price: last + 3.0, Size: 3_500}},
		}
}

func TestAnalyzeMarketBuildsAQualifiedTrendPlanAndRejectsTheCountertrend(t *testing.T) {
	request, contract, candles, book := analysisFixture()

	analysis, err := AnalyzeMarket(request, contract, candles, book, time.Unix(1_700_100_000, 0))
	if err != nil {
		t.Fatalf("AnalyzeMarket: %v", err)
	}
	if analysis.Long.Status != ScenarioReady {
		t.Fatalf("long status = %q, want %q; reasons=%v", analysis.Long.Status, ScenarioReady, analysis.Long.Reasons)
	}
	if analysis.Short.Status == ScenarioReady {
		t.Fatalf("countertrend short unexpectedly ready: %+v", analysis.Short)
	}
	if !(analysis.Long.Stop < analysis.Long.Entry && analysis.Long.Targets[0] > analysis.Long.Entry) {
		t.Fatalf("long levels are not ordered: %+v", analysis.Long)
	}
	if analysis.Long.RiskReward < 1.5 {
		t.Fatalf("risk/reward = %.3f, want at least 1.5", analysis.Long.RiskReward)
	}
	if analysis.Long.RiskAmount != 10 {
		t.Fatalf("risk amount = %.4f, want 10", analysis.Long.RiskAmount)
	}
	if analysis.Long.Contracts < contract.OrderSizeMin || analysis.Long.EstimatedFees <= 0 {
		t.Fatalf("position sizing was not calculated: %+v", analysis.Long)
	}
	if analysis.Indicators.EMA20 <= analysis.Indicators.EMA50 || analysis.Indicators.ATR14 <= 0 {
		t.Fatalf("trend indicators = %+v", analysis.Indicators)
	}
	if len(analysis.Liquidity.AskWalls) == 0 || analysis.Liquidity.AskWalls[0].Notional <= 0 {
		t.Fatalf("ask liquidity walls = %+v", analysis.Liquidity.AskWalls)
	}
}

func TestAnalyzeMarketReturnsNoTradeWhenHistoryCannotWarmIndicators(t *testing.T) {
	request, contract, _, book := analysisFixture()

	analysis, err := AnalyzeMarket(request, contract, trendingCandles(20), book, time.Unix(1_700_100_000, 0))
	if err != nil {
		t.Fatalf("AnalyzeMarket: %v", err)
	}
	if analysis.Long.Status != ScenarioNoTrade || analysis.Short.Status != ScenarioNoTrade {
		t.Fatalf("insufficient history scenarios = long:%q short:%q", analysis.Long.Status, analysis.Short.Status)
	}
	if !strings.Contains(strings.Join(analysis.Long.Reasons, " "), "50") {
		t.Fatalf("long reasons do not explain warm-up requirement: %v", analysis.Long.Reasons)
	}
}

func TestAnalyzeMarketValidatesRiskInputs(t *testing.T) {
	request, contract, candles, book := analysisFixture()
	request.RiskPercent = 0

	if _, err := AnalyzeMarket(request, contract, candles, book, time.Now()); err == nil {
		t.Fatal("zero risk percent was accepted")
	}
	request.RiskPercent = 1
	request.Strategy = "oracle"
	if _, err := AnalyzeMarket(request, contract, candles, book, time.Now()); err == nil {
		t.Fatal("unknown strategy was accepted")
	}
}

func TestValidateAnalyzeRequestAcceptsSingleCharacterBaseAsset(t *testing.T) {
	request, _, _, _ := analysisFixture()
	request.Contract = "H_USDT"
	if err := ValidateAnalyzeRequest(request); err != nil {
		t.Fatalf("H_USDT request rejected: %v", err)
	}
}

func TestAnalyzeMarketKeepsShortLevelsDirectionallySymmetric(t *testing.T) {
	request, contract, candles, book := analysisFixture()
	for left, right := 0, len(candles)-1; left < right; left, right = left+1, right-1 {
		candles[left], candles[right] = candles[right], candles[left]
	}
	for index := range candles {
		candles[index].Time = int64(1_700_000_000 + index*900)
	}
	contract.MarkPrice = candles[len(candles)-1].Close
	contract.IndexPrice = contract.MarkPrice

	analysis, err := AnalyzeMarket(request, contract, candles, book, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Short.Status != ScenarioReady {
		t.Fatalf("short status = %q, want ready; reasons=%v", analysis.Short.Status, analysis.Short.Reasons)
	}
	if !(analysis.Short.Stop > analysis.Short.Entry && analysis.Short.Targets[0] < analysis.Short.Entry) {
		t.Fatalf("short levels are not ordered: %+v", analysis.Short)
	}
}

func TestAnalyzeMarketExcludesTheStillOpenCandleFromIndicators(t *testing.T) {
	request, contract, candles, book := analysisFixture()
	baselineNow := time.Unix(candles[len(candles)-1].Time+901, 0)
	baseline, err := AnalyzeMarket(request, contract, candles, book, baselineNow)
	if err != nil {
		t.Fatal(err)
	}
	withOpen := append(append([]Candle(nil), candles...), Candle{
		Time: baselineNow.Unix() - 300, Open: contract.MarkPrice, High: contract.MarkPrice * 3,
		Low: contract.MarkPrice / 2, Close: contract.MarkPrice * 2, Volume: 9_000_000,
	})

	result, err := AnalyzeMarket(request, contract, withOpen, book, baselineNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candles) != len(candles) {
		t.Fatalf("closed candles = %d, want %d", len(result.Candles), len(candles))
	}
	if result.Indicators.EMA20 != baseline.Indicators.EMA20 || result.Indicators.ATR14 != baseline.Indicators.ATR14 {
		t.Fatalf("open candle changed indicators: got %+v baseline %+v", result.Indicators, baseline.Indicators)
	}
}
