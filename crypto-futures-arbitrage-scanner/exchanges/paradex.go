package exchanges

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type ParadexWSRequest struct {
	ID      int64                  `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

type ParadexWSResponse struct {
	ID      int64  `json:"id"`
	JSONRPC string `json:"jsonrpc"`
	Result  struct {
		Channel string `json:"channel"`
		Status  string `json:"status"`
	} `json:"result,omitempty"`
}

type ParadexTradeEvent struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Channel string `json:"channel"`
		Data    struct {
			ID        string `json:"id"`
			Market    string `json:"market"`
			Price     string `json:"price"`
			Size      string `json:"size"`
			Side      string `json:"side"`
			CreatedAt int64  `json:"created_at"`
			TradeType string `json:"trade_type"`
		} `json:"data"`
	} `json:"params"`
}

type ParadexMarketSummaryEvent struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Channel string `json:"channel"`
		Data    struct {
			Symbol string `json:"symbol"`
			Bid    string `json:"bid"`
			Ask    string `json:"ask"`
		} `json:"data"`
	} `json:"params"`
}

func ConnectParadexFutures(symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	ConnectParadexFuturesContext(context.Background(), symbols, priceChan, orderbookChan, tradeChan, statusChan)
}

func ConnectParadexFuturesContext(ctx context.Context, symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	if len(symbols) == 0 {
		return
	}
	defer publishConnectionStatus(statusChan, "paradex_futures", false, symbols)
	wsURL := "wss://ws.api.prod.paradex.trade/v1"
	allowed := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		allowed[symbol] = struct{}{}
	}

	for {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			publishConnectionStatus(statusChan, "paradex_futures", false, symbols)
			log.Printf("Paradex connection error: %v", err)
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}

		log.Printf("Connected to Paradex futures WebSocket")

		// Subscribe to markets_summary channel (provides bid/ask for all markets)

		subscribeReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "subscribe",
			"params": map[string]interface{}{
				"channel": "markets_summary",
			},
			"id": 1,
		}

		if err := conn.WriteJSON(subscribeReq); err != nil {
			publishConnectionStatus(statusChan, "paradex_futures", false, symbols)
			_ = conn.Close()
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}
		stopClose := closeWebSocketOnCancel(ctx, conn)
		publishConnectionStatus(statusChan, "paradex_futures", true, symbols)

		// Read messages
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				publishConnectionStatus(statusChan, "paradex_futures", false, symbols)
				if ctx.Err() == nil {
					log.Printf("Paradex read error: %v", err)
				}
				break
			}

			// Try to parse as subscription response first
			var subResponse ParadexWSResponse
			if err := json.Unmarshal(message, &subResponse); err == nil && subResponse.Result.Channel == "markets_summary" {
				continue
			}

			// Try to parse as market summary event
			var marketEvent ParadexMarketSummaryEvent
			if err := json.Unmarshal(message, &marketEvent); err == nil &&
				marketEvent.Method == "subscription" && marketEvent.Params.Channel == "markets_summary" {

				symbol := convertFromParadexSymbol(marketEvent.Params.Data.Symbol)
				if symbol == "" {
					continue // Skip unsupported symbols
				}
				if _, enabled := allowed[symbol]; !enabled {
					continue
				}

				// Parse bid and ask prices
				bidPrice, err1 := strconv.ParseFloat(marketEvent.Params.Data.Bid, 64)
				askPrice, err2 := strconv.ParseFloat(marketEvent.Params.Data.Ask, 64)

				if err1 != nil || err2 != nil {
					continue
				}

				// Send orderbook data
				orderbookChan <- OrderbookData{
					Symbol:    symbol,
					Source:    "paradex_futures",
					BestBid:   bidPrice,
					BestAsk:   askPrice,
					Timestamp: time.Now().UnixMilli(),
				}
			}
		}

		conn.Close()
		stopClose()
		log.Printf("Paradex connection closed, reconnecting in 5 seconds...")
		if ctx.Err() != nil || !waitForReconnect(ctx, 5*time.Second) {
			return
		}
	}
}

// Convert standard symbol format to Paradex format
// BTCUSDT -> BTC-USD-PERP
// ETHUSDT -> ETH-USD-PERP
func convertToParadexSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)

	if strings.HasSuffix(symbol, "USDT") {
		return strings.TrimSuffix(symbol, "USDT") + "-USD-PERP"
	}

	return ""
}

// Convert Paradex symbol format back to standard format
// BTC-USD-PERP -> BTCUSDT
func convertFromParadexSymbol(paradexSymbol string) string {
	if strings.HasSuffix(paradexSymbol, "-USD-PERP") {
		return strings.TrimSuffix(paradexSymbol, "-USD-PERP") + "USDT"
	}

	return ""
}
