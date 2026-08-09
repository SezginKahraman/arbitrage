package exchanges

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	kuCoinSpotBulletURL    = "https://api.kucoin.com/api/v1/bullet-public"
	kuCoinFuturesBulletURL = "https://api-futures.kucoin.com/api/v1/bullet-public"
)

type kuCoinWebSocketConfig struct {
	Endpoint     string
	Token        string
	PingInterval time.Duration
	PingTimeout  time.Duration
}

type kuCoinBulletResponse struct {
	Code string `json:"code"`
	Data struct {
		Token           string `json:"token"`
		InstanceServers []struct {
			Endpoint     string `json:"endpoint"`
			Encrypt      bool   `json:"encrypt"`
			Protocol     string `json:"protocol"`
			PingInterval int64  `json:"pingInterval"`
			PingTimeout  int64  `json:"pingTimeout"`
		} `json:"instanceServers"`
	} `json:"data"`
}

type kuCoinSpotTickerMessage struct {
	Type    string `json:"type"`
	Topic   string `json:"topic"`
	Subject string `json:"subject"`
	Data    struct {
		BestAsk string `json:"bestAsk"`
		BestBid string `json:"bestBid"`
		Time    int64  `json:"time"`
	} `json:"data"`
}

type kuCoinFuturesTickerMessage struct {
	Type    string `json:"type"`
	Topic   string `json:"topic"`
	Subject string `json:"subject"`
	Data    struct {
		Symbol       string `json:"symbol"`
		BestBidPrice string `json:"bestBidPrice"`
		BestAskPrice string `json:"bestAskPrice"`
		Timestamp    int64  `json:"ts"`
	} `json:"data"`
}

func requestKuCoinWebSocketConfig(ctx context.Context, client *http.Client, bulletURL string) (kuCoinWebSocketConfig, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, bulletURL, nil)
	if err != nil {
		return kuCoinWebSocketConfig{}, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return kuCoinWebSocketConfig{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return kuCoinWebSocketConfig{}, fmt.Errorf("public bullet endpoint returned HTTP %d", response.StatusCode)
	}

	var payload kuCoinBulletResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return kuCoinWebSocketConfig{}, err
	}
	if payload.Code != "200000" || payload.Data.Token == "" {
		return kuCoinWebSocketConfig{}, fmt.Errorf("public bullet endpoint returned code %q", payload.Code)
	}

	for _, server := range payload.Data.InstanceServers {
		if server.Protocol != "websocket" || !server.Encrypt || server.Endpoint == "" {
			continue
		}
		pingInterval := time.Duration(server.PingInterval) * time.Millisecond
		pingTimeout := time.Duration(server.PingTimeout) * time.Millisecond
		if pingInterval <= 0 {
			pingInterval = 18 * time.Second
		}
		if pingTimeout <= 0 {
			pingTimeout = 10 * time.Second
		}
		return kuCoinWebSocketConfig{
			Endpoint: server.Endpoint, Token: payload.Data.Token,
			PingInterval: pingInterval, PingTimeout: pingTimeout,
		}, nil
	}

	return kuCoinWebSocketConfig{}, fmt.Errorf("public bullet endpoint returned no encrypted WebSocket server")
}

func parseKuCoinSpotTicker(message []byte, receivedAt time.Time) (OrderbookData, bool) {
	var ticker kuCoinSpotTickerMessage
	if err := json.Unmarshal(message, &ticker); err != nil || ticker.Type != "message" ||
		ticker.Subject != "trade.ticker" || !strings.HasPrefix(ticker.Topic, "/market/ticker:") {
		return OrderbookData{}, false
	}

	bestBid, bidErr := strconv.ParseFloat(ticker.Data.BestBid, 64)
	bestAsk, askErr := strconv.ParseFloat(ticker.Data.BestAsk, 64)
	if bidErr != nil || askErr != nil || bestBid <= 0 || bestAsk <= 0 || bestBid > bestAsk {
		return OrderbookData{}, false
	}

	return OrderbookData{
		Symbol: fromKuCoinSymbol(strings.TrimPrefix(ticker.Topic, "/market/ticker:")),
		Source: "kucoin_spot", BestBid: bestBid, BestAsk: bestAsk,
		// KuCoin's spot `time` is the latest trade time, not the BBO push time.
		Timestamp: receivedAt.UnixMilli(),
	}, true
}

func parseKuCoinFuturesTicker(message []byte, receivedAt time.Time) (OrderbookData, bool) {
	var ticker kuCoinFuturesTickerMessage
	if err := json.Unmarshal(message, &ticker); err != nil || ticker.Type != "message" ||
		ticker.Subject != "tickerV2" || ticker.Data.Symbol == "" {
		return OrderbookData{}, false
	}

	bestBid, bidErr := strconv.ParseFloat(ticker.Data.BestBidPrice, 64)
	bestAsk, askErr := strconv.ParseFloat(ticker.Data.BestAskPrice, 64)
	if bidErr != nil || askErr != nil || bestBid <= 0 || bestAsk <= 0 || bestBid > bestAsk {
		return OrderbookData{}, false
	}

	return OrderbookData{
		Symbol: fromKuCoinSymbol(ticker.Data.Symbol), Source: "kucoin_futures",
		BestBid: bestBid, BestAsk: bestAsk,
		Timestamp: kuCoinTimestampMillis(ticker.Data.Timestamp, receivedAt),
	}, true
}

func kuCoinTimestampMillis(timestamp int64, receivedAt time.Time) int64 {
	switch {
	case timestamp >= 1_000_000_000_000_000_000:
		return timestamp / 1_000_000
	case timestamp >= 1_000_000_000_000_000:
		return timestamp / 1_000
	case timestamp >= 1_000_000_000_000:
		return timestamp
	case timestamp >= 1_000_000_000:
		return timestamp * 1_000
	default:
		return receivedAt.UnixMilli()
	}
}

func toKuCoinSpotSymbol(symbol string) string {
	if !strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return strings.TrimSuffix(symbol, "USDT") + "-USDT"
}

func toKuCoinFuturesSymbol(symbol string) string {
	if symbol == "BTCUSDT" {
		return "XBTUSDTM"
	}
	if strings.HasSuffix(symbol, "USDT") {
		return symbol + "M"
	}
	return symbol
}

func fromKuCoinSymbol(symbol string) string {
	standard := strings.ReplaceAll(symbol, "-", "")
	if strings.HasSuffix(standard, "USDTM") {
		standard = strings.TrimSuffix(standard, "M")
	}
	if strings.HasPrefix(standard, "XBT") {
		standard = "BTC" + strings.TrimPrefix(standard, "XBT")
	}
	return standard
}

func kuCoinWebSocketURL(config kuCoinWebSocketConfig, connectID string) (string, error) {
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("token", config.Token)
	query.Set("connectId", connectID)
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

type kuCoinTopicBuilder func(string) string
type kuCoinTickerParser func([]byte, time.Time) (OrderbookData, bool)

func initializeKuCoinConnection(
	conn *websocket.Conn,
	connectID string,
	symbols []string,
	topicForSymbol kuCoinTopicBuilder,
	welcomeTimeout time.Duration,
) error {
	if err := conn.SetReadDeadline(time.Now().Add(welcomeTimeout)); err != nil {
		return err
	}
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("waiting for welcome: %w", err)
		}
		var envelope struct {
			Type string `json:"type"`
			Code string `json:"code"`
		}
		if err := json.Unmarshal(message, &envelope); err != nil {
			continue
		}
		if envelope.Type == "error" {
			return fmt.Errorf("welcome returned WebSocket error code %q", envelope.Code)
		}
		if envelope.Type == "welcome" {
			break
		}
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return err
	}

	for index, symbol := range symbols {
		subscription := map[string]interface{}{
			"id":       fmt.Sprintf("%s-%d", connectID, index),
			"type":     "subscribe",
			"topic":    topicForSymbol(symbol),
			"response": true,
		}
		if err := conn.WriteJSON(subscription); err != nil {
			return err
		}
	}
	return nil
}

func connectKuCoinPublic(
	name string,
	source string,
	bulletURL string,
	symbols []string,
	topicForSymbol kuCoinTopicBuilder,
	parseTicker kuCoinTickerParser,
	orderbookChan chan<- OrderbookData,
	statusChan chan<- ConnectionStatus,
) {
	client := &http.Client{Timeout: 10 * time.Second}

	for {
		config, err := requestKuCoinWebSocketConfig(context.Background(), client, bulletURL)
		if err != nil {
			publishConnectionStatus(statusChan, source, false, symbols)
			log.Printf("%s public token error: %v", name, err)
			time.Sleep(5 * time.Second)
			continue
		}

		connectID := strconv.FormatInt(time.Now().UnixNano(), 10)
		webSocketURL, err := kuCoinWebSocketURL(config, connectID)
		if err != nil {
			publishConnectionStatus(statusChan, source, false, symbols)
			log.Printf("%s WebSocket URL error: %v", name, err)
			time.Sleep(5 * time.Second)
			continue
		}
		conn, _, err := websocket.DefaultDialer.Dial(webSocketURL, nil)
		if err != nil {
			publishConnectionStatus(statusChan, source, false, symbols)
			log.Printf("%s connection error: %v", name, err)
			time.Sleep(5 * time.Second)
			continue
		}

		if err := initializeKuCoinConnection(conn, connectID, symbols, topicForSymbol, 10*time.Second); err != nil {
			publishConnectionStatus(statusChan, source, false, symbols)
			log.Printf("%s initialization error: %v", name, err)
			_ = conn.Close()
			time.Sleep(2 * time.Second)
			continue
		}
		log.Printf("Connected to %s WebSocket", name)
		publishConnectionStatus(statusChan, source, true, symbols)

		heartbeatDone := make(chan struct{})
		go keepKuCoinConnectionAlive(conn, connectID, config, heartbeatDone, name)

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("%s read error: %v", name, err)
				break
			}
			if orderbook, ok := parseTicker(message, time.Now()); ok {
				orderbookChan <- orderbook
			}
		}

		close(heartbeatDone)
		publishConnectionStatus(statusChan, source, false, symbols)
		_ = conn.Close()
		time.Sleep(2 * time.Second)
	}
}

func keepKuCoinConnectionAlive(
	conn *websocket.Conn,
	connectID string,
	config kuCoinWebSocketConfig,
	done <-chan struct{},
	name string,
) {
	interval := config.PingInterval - time.Second
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			_ = conn.SetWriteDeadline(now.Add(config.PingTimeout))
			if err := conn.WriteJSON(map[string]string{
				"id": connectID + "-ping-" + strconv.FormatInt(now.UnixMilli(), 10), "type": "ping",
			}); err != nil {
				log.Printf("%s heartbeat error: %v", name, err)
				_ = conn.Close()
				return
			}
		}
	}
}

// ConnectKuCoinSpot streams public best-bid/best-ask updates. It does not use API credentials.
func ConnectKuCoinSpot(symbols []string, _ chan<- PriceData, orderbookChan chan<- OrderbookData, _ chan<- TradeData, statusChan chan<- ConnectionStatus) {
	connectKuCoinPublic(
		"KuCoin spot", "kucoin_spot", kuCoinSpotBulletURL, symbols,
		func(symbol string) string { return "/market/ticker:" + toKuCoinSpotSymbol(symbol) },
		parseKuCoinSpotTicker, orderbookChan, statusChan,
	)
}

// ConnectKuCoinFutures streams public best-bid/best-ask updates. It does not use API credentials.
func ConnectKuCoinFutures(symbols []string, _ chan<- PriceData, orderbookChan chan<- OrderbookData, _ chan<- TradeData, statusChan chan<- ConnectionStatus) {
	connectKuCoinPublic(
		"KuCoin futures", "kucoin_futures", kuCoinFuturesBulletURL, symbols,
		func(symbol string) string { return "/contractMarket/tickerV2:" + toKuCoinFuturesSymbol(symbol) },
		parseKuCoinFuturesTicker, orderbookChan, statusChan,
	)
}
