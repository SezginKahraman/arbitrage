package futures

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGateServiceUsesOnlyPublicFuturesMarketRoutes(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/futures/usdt/contracts":
			_, _ = w.Write([]byte(`[{"name":"BTC_USDT","status":"trading","quanto_multiplier":"0.001","mark_price":"128","index_price":"127.9","last_price":"128","taker_fee_rate":"0.00075","maker_fee_rate":"-0.0001","order_size_min":1,"order_size_max":100000,"order_price_round":"0.1","leverage_min":"1","leverage_max":"100","funding_rate":"0.0001","funding_interval":28800}]`))
		case "/api/v4/futures/usdt/contracts/BTC_USDT":
			_, _ = w.Write([]byte(`{"name":"BTC_USDT","status":"trading","quanto_multiplier":"0.001","mark_price":"128","index_price":"127.9","last_price":"128","taker_fee_rate":"0.00075","maker_fee_rate":"-0.0001","order_size_min":1,"order_size_max":100000,"order_price_round":"0.1","leverage_min":"1","leverage_max":"100","funding_rate":"0.0001","funding_interval":28800}`))
		case "/api/v4/futures/usdt/candlesticks":
			_, _ = w.Write([]byte(`[{"t":1700000000,"o":"100","h":"101","l":"99","c":"100.5","v":1000,"sum":"100500"}]`))
		case "/api/v4/futures/usdt/order_book":
			_, _ = w.Write([]byte(`{"current":1700000001,"update":1700000000.5,"asks":[{"p":"101","s":50}],"bids":[{"p":"100","s":60}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewGateServiceWithBaseURL(server.URL, server.Client())
	contracts, err := service.Contracts(context.Background())
	if err != nil || len(contracts) != 1 || contracts[0].Symbol != "BTC_USDT" {
		t.Fatalf("Contracts = %+v, %v", contracts, err)
	}
	_, err = service.Analyze(context.Background(), AnalyzeRequest{
		Contract: "BTC_USDT", Interval: "15m", Strategy: "trend_pullback", AccountBalance: 1_000, RiskPercent: 1,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	want := []string{
		"/api/v4/futures/usdt/contracts",
		"/api/v4/futures/usdt/contracts/BTC_USDT",
		"/api/v4/futures/usdt/candlesticks?contract=BTC_USDT&interval=15m&limit=200",
		"/api/v4/futures/usdt/order_book?contract=BTC_USDT&limit=50&with_id=true",
	}
	if strings.Join(paths, "\n") != strings.Join(want, "\n") {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}

func TestGateServiceSanitizesUpstreamFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"label":"internal","detail":"credential-like upstream body"}`, http.StatusBadGateway)
	}))
	defer server.Close()

	service := NewGateServiceWithBaseURL(server.URL, server.Client())
	_, err := service.Contracts(context.Background())
	if err == nil {
		t.Fatal("upstream failure was accepted")
	}
	if strings.Contains(err.Error(), "credential-like") || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("error was not sanitized: %q", err)
	}
}

func TestGateServiceRetriesOneTransientPublicFailure(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			http.Error(w, "temporary upstream failure", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"BTC_USDT","status":"trading","quanto_multiplier":"0.001","mark_price":"128","index_price":"127.9","last_price":"128","taker_fee_rate":"0.00075","maker_fee_rate":"-0.0001","order_size_min":1,"order_size_max":100000,"order_price_round":"0.1","leverage_min":"1","leverage_max":"100","funding_rate":"0.0001","funding_interval":28800}]`))
	}))
	defer server.Close()

	service := NewGateServiceWithBaseURL(server.URL, server.Client())
	contracts, err := service.Contracts(context.Background())

	if err != nil || len(contracts) != 1 {
		t.Fatalf("Contracts = %+v, err = %v", contracts, err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestGateServiceReusesContractAndBookWithinWorkspaceAnalysis(t *testing.T) {
	var contractCalls atomic.Int32
	var bookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/futures/usdt/contracts/BTC_USDT":
			contractCalls.Add(1)
			_, _ = w.Write([]byte(`{"name":"BTC_USDT","status":"trading","quanto_multiplier":"0.001","mark_price":"100","index_price":"100","last_price":"100","taker_fee_rate":"0.00075","maker_fee_rate":"-0.0001","order_size_min":1,"order_size_max":100000,"order_price_round":"0.1","leverage_min":"1","leverage_max":"100","funding_rate":"0.0001","funding_interval":28800}`))
		case "/api/v4/futures/usdt/candlesticks":
			_, _ = w.Write([]byte(`[{"t":1700000000,"o":"100","h":"101","l":"99","c":"100.5","v":"1000"}]`))
		case "/api/v4/futures/usdt/order_book":
			bookCalls.Add(1)
			_, _ = w.Write([]byte(`{"current":1700000001,"update":1700000000.5,"asks":[{"p":"100","s":1000}],"bids":[{"p":"99.9","s":1000}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	gate := NewGateServiceWithBaseURL(server.URL, server.Client())
	binance := &fakeVenueProvider{market: VenueMarket{
		Venue: "binance_futures", Symbol: "BTCUSDT", IndexPrice: 100, MarkPrice: 100,
		ContractMultiplier: 1, QuantityStep: 0.001, MinQuantity: 0.001, TakerFeeRate: 0.0005,
		Book: OrderBook{Bids: []BookLevel{{Price: 100.5, Size: 100}}, Asks: []BookLevel{{Price: 100.6, Size: 100}}},
	}}
	service := NewWorkspaceService(gate, binance)
	_, err := service.Analyze(context.Background(), AnalyzeRequest{
		Contract: "BTC_USDT", Interval: "15m", Strategy: StrategyTrendPullback,
		AccountBalance: 1_000, RiskPercent: 1, TradeNotional: 100, MinNetSpreadPct: 0.1,
	})

	if err != nil {
		t.Fatal(err)
	}
	if contractCalls.Load() != 1 || bookCalls.Load() != 1 {
		t.Fatalf("duplicate Gate calls: contract=%d book=%d", contractCalls.Load(), bookCalls.Load())
	}
}

func TestGateServiceBuildsVenueMarketAndLatestOpenCandle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/futures/usdt/contracts/BTC_USDT":
			_, _ = w.Write([]byte(`{"name":"BTC_USDT","status":"trading","quanto_multiplier":"0.001","mark_price":"101","index_price":"100.9","last_price":"101","taker_fee_rate":"0.00075","maker_fee_rate":"-0.0001","order_size_min":1,"order_size_max":100000,"order_price_round":"0.1","leverage_min":"1","leverage_max":"100","funding_rate":"0.0001","funding_interval":28800,"funding_next_apply":1700006400}`))
		case "/api/v4/futures/usdt/order_book":
			_, _ = w.Write([]byte(`{"current":1700000001,"update":1700000000.5,"asks":[{"p":"101.2","s":50}],"bids":[{"p":"100.8","s":60}]}`))
		case "/api/v4/futures/usdt/candlesticks":
			_, _ = w.Write([]byte(`[{"t":1700000100,"o":"100","h":"102","l":"99","c":"101.5","v":"1200"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewGateServiceWithBaseURL(server.URL, server.Client())
	market, err := service.VenueMarket(context.Background(), "BTC_USDT", 20)
	if err != nil {
		t.Fatal(err)
	}
	if market.Venue != "gate_futures" || market.Symbol != "BTCUSDT" || market.ContractMultiplier != 0.001 || market.QuantityStep != 1 {
		t.Fatalf("market = %+v", market)
	}
	if market.PriceIncrement != 0.1 {
		t.Fatalf("price increment = %+v", market)
	}
	candle, err := service.LatestCandle(context.Background(), "BTC_USDT", "15m")
	if err != nil || candle.Time != 1700000100 || candle.Close != 101.5 {
		t.Fatalf("candle = %+v, err = %v", candle, err)
	}
}
