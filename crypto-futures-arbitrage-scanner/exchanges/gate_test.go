package exchanges

import "testing"

func TestParseGateSpotBookTicker(t *testing.T) {
	message := []byte(`{
		"time":1606293275,
		"time_ms":1606293275723,
		"channel":"spot.book_ticker",
		"event":"update",
		"result":{"t":1606293275123,"u":48733182,"s":"COTI_USDT","b":"0.01130","B":"1200","a":"0.01131","A":"800"}
	}`)

	got, ok := parseGateSpotBookTicker(message)
	if !ok {
		t.Fatal("valid Gate spot book ticker was rejected")
	}
	if got.Symbol != "COTIUSDT" || got.Source != "gate_spot" || got.BestBid != 0.01130 || got.BestAsk != 0.01131 || got.Timestamp != 1606293275123 {
		t.Fatalf("parsed order book = %+v", got)
	}
}

func TestParseGateSpotBookTickerRejectsWrongChannelAndInvalidPrices(t *testing.T) {
	for _, message := range [][]byte{
		[]byte(`{"channel":"spot.tickers","event":"update","result":{"s":"COTI_USDT","b":"0.01130","a":"0.01131","t":1}}`),
		[]byte(`{"channel":"spot.book_ticker","event":"update","result":{"s":"COTI_USDT","b":"bad","a":"0.01131","t":1}}`),
	} {
		if _, ok := parseGateSpotBookTicker(message); ok {
			t.Fatalf("invalid message was accepted: %s", message)
		}
	}
}
