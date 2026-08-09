package exchanges

import (
	"testing"
	"time"
)

func TestParseBinanceSpotBookTickerUsesReceiveTimeWhenEventTimeIsAbsent(t *testing.T) {
	receivedAt := time.UnixMilli(1_786_290_460_270)
	message := []byte(`{
		"u": 400900217,
		"s": "COTIUSDT",
		"b": "0.01262",
		"B": "31.21",
		"a": "0.01263",
		"A": "40.66"
	}`)

	quote, ok := parseBinanceSpotBookTicker(message, receivedAt)
	if !ok {
		t.Fatal("valid Binance Spot book ticker was rejected")
	}
	if quote.Symbol != "COTIUSDT" || quote.Source != "binance_spot" {
		t.Fatalf("quote identity = %+v", quote)
	}
	if quote.BestBid != 0.01262 || quote.BestAsk != 0.01263 {
		t.Fatalf("quote prices = bid %.8f ask %.8f", quote.BestBid, quote.BestAsk)
	}
	if quote.Timestamp != receivedAt.UnixMilli() {
		t.Fatalf("quote timestamp = %d, want receive time %d", quote.Timestamp, receivedAt.UnixMilli())
	}
}
