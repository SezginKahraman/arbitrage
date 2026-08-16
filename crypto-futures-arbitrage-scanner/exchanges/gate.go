package exchanges

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type GateFuturesTrade struct {
	Size         int64  `json:"size"`
	ID           int64  `json:"id"`
	CreateTime   int64  `json:"create_time"`
	CreateTimeMs int64  `json:"create_time_ms"`
	Price        string `json:"price"`
	Contract     string `json:"contract"`
	IsInternal   bool   `json:"is_internal,omitempty"`
}

type GateWebSocketMessage struct {
	Time    int64  `json:"time"`
	Channel string `json:"channel"`
	Event   string `json:"event"`
	Result  struct {
		Status string `json:"status"`
	} `json:"result,omitempty"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type GateBookTickerResult struct {
	Symbol      string `json:"s"` // Contract symbol
	BestBid     string `json:"b"` // Best bid price
	BestBidSize int64  `json:"B"` // Best bid size
	BestAsk     string `json:"a"` // Best ask price
	BestAskSize int64  `json:"A"` // Best ask size
	Timestamp   int64  `json:"t"` // Timestamp in milliseconds
}

type GateBookTickerMessage struct {
	Time    int64                `json:"time"`
	Channel string               `json:"channel"`
	Event   string               `json:"event"`
	Result  GateBookTickerResult `json:"result"`
}

type GateSpotBookTickerResult struct {
	Timestamp   int64  `json:"t"`
	Symbol      string `json:"s"`
	BestBid     string `json:"b"`
	BestBidSize string `json:"B"`
	BestAsk     string `json:"a"`
	BestAskSize string `json:"A"`
}

type GateSpotBookTickerMessage struct {
	TimeMS  int64                    `json:"time_ms"`
	Channel string                   `json:"channel"`
	Event   string                   `json:"event"`
	Result  GateSpotBookTickerResult `json:"result"`
}

type GateTradeMessage struct {
	Time    int64              `json:"time"`
	Channel string             `json:"channel"`
	Event   string             `json:"event"`
	Result  []GateFuturesTrade `json:"result"`
}

type GateFuturesOrderbook struct {
	Contract     string     `json:"contract"`
	Ask          [][]string `json:"asks"`
	Bid          [][]string `json:"bids"`
	UpdateTime   int64      `json:"update_time"`
	UpdateTimeMs int64      `json:"update_time_ms"`
	UpdateID     int64      `json:"update_id"`
}

type GateOrderbookMessage struct {
	Time    int64                  `json:"time"`
	Channel string                 `json:"channel"`
	Event   string                 `json:"event"`
	Result  []GateFuturesOrderbook `json:"result"`
}

type GateSubscribeMessage struct {
	Time    int64    `json:"time"`
	Channel string   `json:"channel"`
	Event   string   `json:"event"`
	Payload []string `json:"payload"`
}

func ConnectGateFutures(symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	ConnectGateFuturesContext(context.Background(), symbols, priceChan, orderbookChan, tradeChan, statusChan)
}

func ConnectGateFuturesContext(ctx context.Context, symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	if len(symbols) == 0 {
		return
	}
	defer publishConnectionStatus(statusChan, "gate_futures", false, symbols)
	wsURL := "wss://fx-ws.gateio.ws/v4/ws/usdt"

	for {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			publishConnectionStatus(statusChan, "gate_futures", false, symbols)
			log.Printf("Gate.io connection error: %v", err)
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}

		log.Printf("Connected to Gate.io futures WebSocket")

		// Convert symbols to Gate.io format
		gateSymbols := make([]string, len(symbols))
		for i, symbol := range symbols {
			gateSymbols[i] = convertToGateSymbol(symbol)
		}

		// Subscribe to book ticker for all symbols - this provides best bid/ask
		bookTickerSubscribeMsg := GateSubscribeMessage{
			Time:    time.Now().Unix(),
			Channel: "futures.book_ticker",
			Event:   "subscribe",
			Payload: gateSymbols,
		}

		err = conn.WriteJSON(bookTickerSubscribeMsg)
		if err != nil {
			publishConnectionStatus(statusChan, "gate_futures", false, symbols)
			log.Printf("Gate.io book ticker subscription error: %v", err)
			conn.Close()
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}
		stopClose := closeWebSocketOnCancel(ctx, conn)
		publishConnectionStatus(statusChan, "gate_futures", true, symbols)

		if err != nil {
			continue
		}

		for {
			var message json.RawMessage
			err := conn.ReadJSON(&message)
			if err != nil {
				publishConnectionStatus(statusChan, "gate_futures", false, symbols)
				if ctx.Err() == nil {
					log.Printf("Gate.io read error: %v", err)
				}
				conn.Close()
				break
			}

			// First, try to parse as a general WebSocket message to check for errors
			var wsMsg GateWebSocketMessage
			if err := json.Unmarshal(message, &wsMsg); err == nil {
				if wsMsg.Error != nil {
					log.Printf("Gate.io WebSocket error: %d - %s", wsMsg.Error.Code, wsMsg.Error.Message)
					continue
				}

				// Skip subscription confirmation messages
				if wsMsg.Event == "subscribe" {
					continue
				}
			}

			// Try to parse as book ticker message
			var bookTickerMsg GateBookTickerMessage
			if err := json.Unmarshal(message, &bookTickerMsg); err == nil &&
				bookTickerMsg.Channel == "futures.book_ticker" &&
				bookTickerMsg.Event == "update" {

				// Parse best bid and ask
				bestBid, err1 := strconv.ParseFloat(bookTickerMsg.Result.BestBid, 64)
				bestAsk, err2 := strconv.ParseFloat(bookTickerMsg.Result.BestAsk, 64)
				if err1 != nil || err2 != nil {
					log.Printf("Gate.io: Error parsing prices - bid: %v, ask: %v", err1, err2)
					continue
				}

				// Convert Gate.io symbol back to standard format
				standardSymbol := convertFromGateSymbol(bookTickerMsg.Result.Symbol)

				// Use timestamp from message
				var timestamp int64
				if bookTickerMsg.Result.Timestamp > 0 {
					timestamp = bookTickerMsg.Result.Timestamp
				} else {
					timestamp = time.Now().UnixMilli()
				}

				orderbookData := OrderbookData{
					Symbol:    standardSymbol,
					Source:    "gate_futures",
					BestBid:   bestBid,
					BestAsk:   bestAsk,
					Timestamp: timestamp,
				}

				orderbookChan <- orderbookData
				continue
			}

			// Silently ignore unhandled message types
		}

		stopClose()
		if ctx.Err() != nil || !waitForReconnect(ctx, 2*time.Second) {
			return
		}
	}
}

func parseGateSpotBookTicker(message []byte) (OrderbookData, bool) {
	var bookTicker GateSpotBookTickerMessage
	if err := json.Unmarshal(message, &bookTicker); err != nil ||
		bookTicker.Channel != "spot.book_ticker" || bookTicker.Event != "update" {
		return OrderbookData{}, false
	}

	bestBid, bidErr := strconv.ParseFloat(bookTicker.Result.BestBid, 64)
	bestAsk, askErr := strconv.ParseFloat(bookTicker.Result.BestAsk, 64)
	if bidErr != nil || askErr != nil || bestBid <= 0 || bestAsk <= 0 || bestBid > bestAsk {
		return OrderbookData{}, false
	}

	timestamp := bookTicker.Result.Timestamp
	if timestamp <= 0 {
		timestamp = bookTicker.TimeMS
	}
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}

	return OrderbookData{
		Symbol:    convertFromGateSymbol(bookTicker.Result.Symbol),
		Source:    "gate_spot",
		BestBid:   bestBid,
		BestAsk:   bestAsk,
		Timestamp: timestamp,
	}, true
}

// ConnectGateSpot streams public best-bid/best-ask updates. It does not use API credentials.
func ConnectGateSpot(symbols []string, _ chan<- PriceData, orderbookChan chan<- OrderbookData, _ chan<- TradeData, statusChan chan<- ConnectionStatus) {
	ConnectGateSpotContext(context.Background(), symbols, nil, orderbookChan, nil, statusChan)
}

func ConnectGateSpotContext(ctx context.Context, symbols []string, _ chan<- PriceData, orderbookChan chan<- OrderbookData, _ chan<- TradeData, statusChan chan<- ConnectionStatus) {
	if len(symbols) == 0 {
		return
	}
	defer publishConnectionStatus(statusChan, "gate_spot", false, symbols)
	const wsURL = "wss://api.gateio.ws/ws/v4/"

	for {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			publishConnectionStatus(statusChan, "gate_spot", false, symbols)
			log.Printf("Gate.io spot connection error: %v", err)
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}

		log.Printf("Connected to Gate.io spot WebSocket")
		gateSymbols := make([]string, len(symbols))
		for index, symbol := range symbols {
			gateSymbols[index] = convertToGateSymbol(symbol)
		}
		subscription := GateSubscribeMessage{
			Time:    time.Now().Unix(),
			Channel: "spot.book_ticker",
			Event:   "subscribe",
			Payload: gateSymbols,
		}
		if err := conn.WriteJSON(subscription); err != nil {
			publishConnectionStatus(statusChan, "gate_spot", false, symbols)
			log.Printf("Gate.io spot subscription error: %v", err)
			_ = conn.Close()
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}
		stopClose := closeWebSocketOnCancel(ctx, conn)
		publishConnectionStatus(statusChan, "gate_spot", true, symbols)

		for {
			var message json.RawMessage
			if err := conn.ReadJSON(&message); err != nil {
				publishConnectionStatus(statusChan, "gate_spot", false, symbols)
				if ctx.Err() == nil {
					log.Printf("Gate.io spot read error: %v", err)
				}
				_ = conn.Close()
				break
			}

			var envelope GateWebSocketMessage
			if err := json.Unmarshal(message, &envelope); err == nil && envelope.Error != nil {
				log.Printf("Gate.io spot WebSocket error: %d - %s", envelope.Error.Code, envelope.Error.Message)
				continue
			}
			if orderbook, ok := parseGateSpotBookTicker(message); ok {
				orderbookChan <- orderbook
			}
		}

		stopClose()
		if ctx.Err() != nil || !waitForReconnect(ctx, 2*time.Second) {
			return
		}
	}
}

// convertToGateSymbol converts standard symbol format to Gate.io format
// BTCUSDT -> BTC_USDT (for USDT perpetual futures)
func convertToGateSymbol(symbol string) string {
	// Handle common symbols for USDT perpetual futures
	switch symbol {
	case "BTCUSDT":
		return "BTC_USDT"
	case "ETHUSDT":
		return "ETH_USDT"
	case "ADAUSDT":
		return "ADA_USDT"
	case "SOLUSDT":
		return "SOL_USDT"
	case "DOTUSDT":
		return "DOT_USDT"
	case "LINKUSDT":
		return "LINK_USDT"
	case "AVAXUSDT":
		return "AVAX_USDT"
	case "MATICUSDT":
		return "MATIC_USDT"
	case "UNIUSDT":
		return "UNI_USDT"
	case "LTCUSDT":
		return "LTC_USDT"
	case "BCHUSDT":
		return "BCH_USDT"
	case "XRPUSDT":
		return "XRP_USDT"
	default:
		// Generic conversion for USDT perpetual futures
		if strings.HasSuffix(symbol, "USDT") {
			base := strings.TrimSuffix(symbol, "USDT")
			return fmt.Sprintf("%s_USDT", base)
		}
		// For other quote currencies, insert underscore before the last part
		if len(symbol) >= 6 {
			// Assume last 3-4 characters are quote currency
			if strings.HasSuffix(symbol, "USDC") {
				base := strings.TrimSuffix(symbol, "USDC")
				return fmt.Sprintf("%s_USDC", base)
			}
			if strings.HasSuffix(symbol, "USD") {
				base := strings.TrimSuffix(symbol, "USD")
				return fmt.Sprintf("%s_USD", base)
			}
		}
		return symbol
	}
}

// convertFromGateSymbol converts Gate.io symbol format back to standard format
// BTC_USDT -> BTCUSDT
func convertFromGateSymbol(gateSymbol string) string {
	// Remove underscore and convert to standard format
	parts := strings.Split(gateSymbol, "_")
	if len(parts) == 2 {
		return parts[0] + parts[1]
	}
	return gateSymbol
}
