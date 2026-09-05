package futures

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestBinancePublicClientBuildsExecutableVenueMarket(t *testing.T) {
	var mutex sync.Mutex
	paths := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		paths = append(paths, r.URL.RequestURI())
		mutex.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/fapi/v1/exchangeInfo":
			_, _ = w.Write([]byte(`{"symbols":[{"symbol":"BTCUSDT","status":"TRADING","contractType":"PERPETUAL","baseAsset":"BTC","quoteAsset":"USDT","filters":[{"filterType":"PRICE_FILTER","tickSize":"0.1"},{"filterType":"LOT_SIZE","minQty":"0.001","stepSize":"0.001"},{"filterType":"MIN_NOTIONAL","notional":"5"}]}]}`))
		case "/fapi/v1/premiumIndex":
			_, _ = w.Write([]byte(`{"symbol":"BTCUSDT","markPrice":"101.1","indexPrice":"101","lastFundingRate":"0.0002","nextFundingTime":1700006400000,"time":1700000000000}`))
		case "/fapi/v1/depth":
			_, _ = w.Write([]byte(`{"lastUpdateId":42,"E":1700000000100,"T":1700000000050,"bids":[["100.9","2.5"]],"asks":[["101.2","3.5"]]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewBinancePublicClientWithBaseURL(server.URL, server.Client(), 0.0005)
	market, err := client.VenueMarket(context.Background(), "BTC_USDT", 50)
	if err != nil {
		t.Fatal(err)
	}
	if market.Venue != "binance_futures" || market.Symbol != "BTCUSDT" || market.ContractMultiplier != 1 {
		t.Fatalf("market identity = %+v", market)
	}
	if market.QuantityStep != 0.001 || market.MinQuantity != 0.001 || market.TakerFeeRate != 0.0005 {
		t.Fatalf("market rules = %+v", market)
	}
	if market.PriceIncrement != 0.1 || market.MinNotional != 5 {
		t.Fatalf("probe rules = %+v", market)
	}
	if market.Book.Bids[0].Price != 100.9 || market.Book.Bids[0].Size != 2.5 || market.Book.Asks[0].Price != 101.2 {
		t.Fatalf("book = %+v", market.Book)
	}
	mutex.Lock()
	sort.Strings(paths)
	gotPaths := strings.Join(paths, "\n")
	mutex.Unlock()
	wantPaths := strings.Join([]string{
		"/fapi/v1/depth?limit=50&symbol=BTCUSDT",
		"/fapi/v1/exchangeInfo",
		"/fapi/v1/premiumIndex?symbol=BTCUSDT",
	}, "\n")
	if gotPaths != wantPaths {
		t.Fatalf("paths = %q, want %q", gotPaths, wantPaths)
	}
}
