package exchanges

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSpotRESTParsersNormalizeAllowedBestBidAndAsk(t *testing.T) {
	receivedAt := time.UnixMilli(20_000)
	tests := []struct {
		name    string
		payload string
		parse   spotRESTParser
		symbol  string
		source  string
		bid     float64
		ask     float64
	}{
		{
			name: "Binance", source: "binance_spot", symbol: "COTIUSDT", bid: 0.01217, ask: 0.01218,
			payload: `[{"symbol":"COTIUSDT","bidPrice":"0.01217","askPrice":"0.01218"},{"symbol":"OTHERUSDT","bidPrice":"1","askPrice":"2"}]`,
			parse:   parseBinanceSpotRESTBooks,
		},
		{
			name: "Gate.io", source: "gate_spot", symbol: "COTIUSDT", bid: 0.010768, ask: 0.010781,
			payload: `[{"currency_pair":"COTI_USDT","highest_bid":"0.010768","lowest_ask":"0.010781"}]`,
			parse:   parseGateSpotRESTBooks,
		},
		{
			name: "KuCoin", source: "kucoin_spot", symbol: "COTIUSDT", bid: 0.01075, ask: 0.01087,
			payload: `{"code":"200000","data":{"ticker":[{"symbol":"COTI-USDT","buy":"0.01075","sell":"0.01087"}]}}`,
			parse:   parseKuCoinSpotRESTBooks,
		},
		{
			name: "Bybit", source: "bybit_spot", symbol: "BTCUSDT", bid: 65219.9, ask: 65220,
			payload: `{"retCode":0,"result":{"list":[{"symbol":"BTCUSDT","bid1Price":"65219.9","ask1Price":"65220"}]}}`,
			parse:   parseBybitSpotRESTBooks,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			books, err := test.parse([]byte(test.payload), map[string]struct{}{test.symbol: {}}, receivedAt)
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}
			if len(books) != 1 {
				t.Fatalf("books = %+v", books)
			}
			book := books[0]
			if book.Source != test.source || book.Symbol != test.symbol || book.BestBid != test.bid || book.BestAsk != test.ask || book.Timestamp != receivedAt.UnixMilli() {
				t.Fatalf("book = %+v", book)
			}
		})
	}
}

func TestFetchSpotRESTBooksUsesSuccessfulResponseAsCurrentValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"symbol":"COTIUSDT","bidPrice":"0.01217","askPrice":"0.01218"}]`))
	}))
	defer server.Close()

	receivedAt := time.UnixMilli(25_000)
	books, err := fetchSpotRESTBooks(
		context.Background(), server.Client(), server.URL, []string{"COTIUSDT"}, parseBinanceSpotRESTBooks, receivedAt,
	)
	if err != nil {
		t.Fatalf("fetchSpotRESTBooks() error = %v", err)
	}
	if len(books) != 1 || books[0].Timestamp != receivedAt.UnixMilli() {
		t.Fatalf("books = %+v", books)
	}
}

func TestFetchSpotRESTBooksRejectsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := fetchSpotRESTBooks(
		context.Background(), server.Client(), server.URL, []string{"COTIUSDT"}, parseBinanceSpotRESTBooks, time.Now(),
	)
	if err == nil {
		t.Fatal("HTTP failure was accepted as a book validation")
	}
}
