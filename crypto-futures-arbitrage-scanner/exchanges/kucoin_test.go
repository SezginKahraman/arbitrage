package exchanges

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRequestKuCoinWebSocketConfigUsesPublicBulletEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"code":"200000",
			"data":{
				"token":"public-token",
				"instanceServers":[{
					"endpoint":"wss://ws-api-spot.kucoin.test/",
					"encrypt":true,
					"protocol":"websocket",
					"pingInterval":18000,
					"pingTimeout":10000
				}]
			}
		}`))
	}))
	defer server.Close()

	config, err := requestKuCoinWebSocketConfig(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("requestKuCoinWebSocketConfig() error = %v", err)
	}
	if config.Endpoint != "wss://ws-api-spot.kucoin.test/" || config.Token != "public-token" {
		t.Fatalf("config identity = %+v", config)
	}
	if config.PingInterval != 18*time.Second || config.PingTimeout != 10*time.Second {
		t.Fatalf("config heartbeat = %+v", config)
	}
}

func TestInitializeKuCoinConnectionWaitsForWelcomeBeforeSubscribing(t *testing.T) {
	serverResult := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(writer, request, nil)
		if err != nil {
			serverResult <- err
			return
		}
		defer conn.Close()

		clientMessages := make(chan []byte, 1)
		readErrors := make(chan error, 1)
		go func() {
			_, message, readErr := conn.ReadMessage()
			if readErr != nil {
				readErrors <- readErr
				return
			}
			clientMessages <- message
		}()

		select {
		case message := <-clientMessages:
			serverResult <- fmt.Errorf("received subscription before welcome: %s", message)
			return
		case err := <-readErrors:
			serverResult <- err
			return
		case <-time.After(50 * time.Millisecond):
		}

		if err := conn.WriteJSON(map[string]string{"id": "connection-id", "type": "welcome"}); err != nil {
			serverResult <- err
			return
		}

		select {
		case message := <-clientMessages:
			var subscription map[string]interface{}
			if err := json.Unmarshal(message, &subscription); err != nil {
				serverResult <- err
				return
			}
			if subscription["type"] != "subscribe" || subscription["topic"] != "/market/ticker:COTI-USDT" {
				serverResult <- fmt.Errorf("subscription = %+v", subscription)
				return
			}
			serverResult <- nil
		case err := <-readErrors:
			serverResult <- err
		case <-time.After(time.Second):
			serverResult <- fmt.Errorf("subscription was not received")
		}
	}))
	defer server.Close()

	webSocketURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(webSocketURL, nil)
	if err != nil {
		t.Fatalf("dial test WebSocket: %v", err)
	}
	defer conn.Close()

	err = initializeKuCoinConnection(
		conn,
		"connection-id",
		[]string{"COTIUSDT"},
		func(symbol string) string { return "/market/ticker:" + toKuCoinSpotSymbol(symbol) },
		time.Second,
	)
	if err != nil {
		t.Fatalf("initializeKuCoinConnection() error = %v", err)
	}
	if err := <-serverResult; err != nil {
		t.Fatal(err)
	}
}

func TestParseKuCoinSpotTickerProducesExecutableCOTIQuote(t *testing.T) {
	receivedAt := time.UnixMilli(1786292250000)
	message := []byte(`{
		"type":"message",
		"topic":"/market/ticker:COTI-USDT",
		"subject":"trade.ticker",
		"data":{
			"sequence":"1545896668986",
			"bestAsk":"0.011194",
			"bestBid":"0.011181",
			"time":1786291899065
		}
	}`)

	quote, ok := parseKuCoinSpotTicker(message, receivedAt)
	if !ok {
		t.Fatal("valid KuCoin spot ticker was rejected")
	}
	if quote.Symbol != "COTIUSDT" || quote.Source != "kucoin_spot" {
		t.Fatalf("quote identity = %+v", quote)
	}
	if quote.BestBid != 0.011181 || quote.BestAsk != 0.011194 || quote.Timestamp != receivedAt.UnixMilli() {
		t.Fatalf("quote prices/timestamp = %+v", quote)
	}
}

func TestParseKuCoinFuturesTickerNormalizesSymbolAndNanoseconds(t *testing.T) {
	message := []byte(`{
		"type":"message",
		"topic":"/contractMarket/tickerV2:COTIUSDTM",
		"subject":"tickerV2",
		"data":{
			"symbol":"COTIUSDTM",
			"bestBidPrice":"0.011180",
			"bestAskPrice":"0.011195",
			"ts":1786291899065000000
		}
	}`)

	quote, ok := parseKuCoinFuturesTicker(message, time.UnixMilli(1))
	if !ok {
		t.Fatal("valid KuCoin futures ticker was rejected")
	}
	if quote.Symbol != "COTIUSDT" || quote.Source != "kucoin_futures" {
		t.Fatalf("quote identity = %+v", quote)
	}
	if quote.BestBid != 0.011180 || quote.BestAsk != 0.011195 || quote.Timestamp != 1786291899065 {
		t.Fatalf("quote prices/timestamp = %+v", quote)
	}
}

func TestKuCoinSymbolConversionHandlesBTCAndUSDTContracts(t *testing.T) {
	tests := []struct {
		standard string
		spot     string
		futures  string
	}{
		{standard: "BTCUSDT", spot: "BTC-USDT", futures: "XBTUSDTM"},
		{standard: "COTIUSDT", spot: "COTI-USDT", futures: "COTIUSDTM"},
	}

	for _, test := range tests {
		if got := toKuCoinSpotSymbol(test.standard); got != test.spot {
			t.Fatalf("toKuCoinSpotSymbol(%q) = %q, want %q", test.standard, got, test.spot)
		}
		if got := toKuCoinFuturesSymbol(test.standard); got != test.futures {
			t.Fatalf("toKuCoinFuturesSymbol(%q) = %q, want %q", test.standard, got, test.futures)
		}
		if got := fromKuCoinSymbol(test.futures); got != test.standard {
			t.Fatalf("fromKuCoinSymbol(%q) = %q, want %q", test.futures, got, test.standard)
		}
	}
}
