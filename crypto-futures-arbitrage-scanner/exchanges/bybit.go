package exchanges

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

type BybitFuturesTrade struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	Data  []struct {
		Symbol    string `json:"s"`
		Price     string `json:"p"`
		Size      string `json:"v"`
		Side      string `json:"S"`
		Timestamp int64  `json:"T"`
		TradeID   string `json:"i"`
	} `json:"data"`
}

type BybitFuturesOrderbook struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	Data  struct {
		Symbol   string     `json:"s"`
		Bids     [][]string `json:"b"`
		Asks     [][]string `json:"a"`
		UpdateID int64      `json:"u"`
		SeqNum   int64      `json:"seq"`
	} `json:"data"`
}

func ConnectBybitFutures(symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	ConnectBybitFuturesContext(context.Background(), symbols, priceChan, orderbookChan, tradeChan, statusChan)
}

func ConnectBybitFuturesContext(ctx context.Context, symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	if len(symbols) == 0 {
		return
	}
	defer publishConnectionStatus(statusChan, "bybit_futures", false, symbols)
	wsURL := "wss://stream.bybit.com/v5/public/linear"

	for {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			publishConnectionStatus(statusChan, "bybit_futures", false, symbols)
			log.Printf("Bybit futures connection error: %v", err)
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}

		log.Printf("Connected to Bybit futures WebSocket")

		subscribeMsg := map[string]interface{}{
			"op":   "subscribe",
			"args": make([]string, len(symbols)*2),
		}

		for i, symbol := range symbols {
			subscribeMsg["args"].([]string)[i*2] = fmt.Sprintf("orderbook.1.%s", symbol)
			subscribeMsg["args"].([]string)[i*2+1] = fmt.Sprintf("publicTrade.%s", symbol)
		}

		err = conn.WriteJSON(subscribeMsg)
		if err != nil {
			publishConnectionStatus(statusChan, "bybit_futures", false, symbols)
			log.Printf("Bybit futures subscription error: %v", err)
			conn.Close()
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}
		stopClose := closeWebSocketOnCancel(ctx, conn)
		publishConnectionStatus(statusChan, "bybit_futures", true, symbols)

		for {
			var message json.RawMessage
			err := conn.ReadJSON(&message)
			if err != nil {
				publishConnectionStatus(statusChan, "bybit_futures", false, symbols)
				if ctx.Err() == nil {
					log.Printf("Bybit futures read error: %v", err)
				}
				conn.Close()
				break
			}

			// Try to parse as orderbook first
			var orderbookMsg BybitFuturesOrderbook
			if err := json.Unmarshal(message, &orderbookMsg); err == nil &&
				len(orderbookMsg.Data.Asks) > 0 && len(orderbookMsg.Data.Bids) > 0 {

				bidPrice, err1 := strconv.ParseFloat(orderbookMsg.Data.Bids[0][0], 64)
				askPrice, err2 := strconv.ParseFloat(orderbookMsg.Data.Asks[0][0], 64)
				if err1 != nil || err2 != nil {
					continue
				}

				orderbookData := OrderbookData{
					Symbol:    orderbookMsg.Data.Symbol,
					Source:    "bybit_futures",
					BestBid:   bidPrice,
					BestAsk:   askPrice,
					Timestamp: time.Now().UnixMilli(),
				}

				orderbookChan <- orderbookData
				continue
			}

			// Try to parse as trade message
			var tradeMsg BybitFuturesTrade
			if err := json.Unmarshal(message, &tradeMsg); err == nil &&
				(tradeMsg.Type == "snapshot" || tradeMsg.Type == "delta") {

				for _, trade := range tradeMsg.Data {
					price, err := strconv.ParseFloat(trade.Price, 64)
					if err != nil {
						continue
					}

					// Normalize trade side (Bybit uses "Buy" and "Sell")
					var side string
					if trade.Side == "Buy" {
						side = "buy"
					} else {
						side = "sell"
					}

					tradeData := TradeData{
						Symbol:    trade.Symbol,
						Source:    "bybit_futures",
						Price:     price,
						Quantity:  trade.Size,
						Side:      side,
						Timestamp: trade.Timestamp,
					}

					tradeChan <- tradeData
				}
			}
		}

		stopClose()
		if ctx.Err() != nil || !waitForReconnect(ctx, 2*time.Second) {
			return
		}
	}
}

// BybitSpotTrade represents the structure for Bybit spot trade data
type BybitSpotTrade struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	Data  []struct {
		Symbol    string `json:"s"`
		Price     string `json:"p"`
		Size      string `json:"v"`
		Side      string `json:"S"`
		Timestamp int64  `json:"T"`
		TradeID   string `json:"i"`
	} `json:"data"`
}

type BybitSpotOrderbook struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	Data  struct {
		Symbol   string     `json:"s"`
		Bids     [][]string `json:"b"`
		Asks     [][]string `json:"a"`
		UpdateID int64      `json:"u"`
		SeqNum   int64      `json:"seq"`
	} `json:"data"`
}

// ConnectBybitSpot connects to Bybit spot trading WebSocket API
func ConnectBybitSpot(symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	ConnectBybitSpotContext(context.Background(), symbols, priceChan, orderbookChan, tradeChan, statusChan)
}

func ConnectBybitSpotContext(ctx context.Context, symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	if len(symbols) == 0 {
		return
	}
	defer publishConnectionStatus(statusChan, "bybit_spot", false, symbols)
	wsURL := "wss://stream.bybit.com/v5/public/spot"

	for {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			publishConnectionStatus(statusChan, "bybit_spot", false, symbols)
			log.Printf("Bybit spot connection error: %v", err)
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}

		log.Printf("Connected to Bybit spot WebSocket")

		subscribeMsg := map[string]interface{}{
			"op":   "subscribe",
			"args": make([]string, len(symbols)*2),
		}

		for i, symbol := range symbols {
			subscribeMsg["args"].([]string)[i*2] = fmt.Sprintf("orderbook.1.%s", symbol)
			subscribeMsg["args"].([]string)[i*2+1] = fmt.Sprintf("publicTrade.%s", symbol)
		}

		err = conn.WriteJSON(subscribeMsg)
		if err != nil {
			publishConnectionStatus(statusChan, "bybit_spot", false, symbols)
			log.Printf("Bybit spot subscription error: %v", err)
			conn.Close()
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}
		stopClose := closeWebSocketOnCancel(ctx, conn)
		publishConnectionStatus(statusChan, "bybit_spot", true, symbols)

		for {
			var message json.RawMessage
			err := conn.ReadJSON(&message)
			if err != nil {
				publishConnectionStatus(statusChan, "bybit_spot", false, symbols)
				if ctx.Err() == nil {
					log.Printf("Bybit spot read error: %v", err)
				}
				conn.Close()
				break
			}

			// Try to parse as orderbook first
			var orderbookMsg BybitSpotOrderbook
			if err := json.Unmarshal(message, &orderbookMsg); err == nil &&
				len(orderbookMsg.Data.Asks) > 0 && len(orderbookMsg.Data.Bids) > 0 {

				bidPrice, err1 := strconv.ParseFloat(orderbookMsg.Data.Bids[0][0], 64)
				askPrice, err2 := strconv.ParseFloat(orderbookMsg.Data.Asks[0][0], 64)
				if err1 != nil || err2 != nil {
					continue
				}

				orderbookData := OrderbookData{
					Symbol:    orderbookMsg.Data.Symbol,
					Source:    "bybit_spot",
					BestBid:   bidPrice,
					BestAsk:   askPrice,
					Timestamp: time.Now().UnixMilli(),
				}

				orderbookChan <- orderbookData
				continue
			}

			// Try to parse as trade message
			var tradeMsg BybitSpotTrade
			if err := json.Unmarshal(message, &tradeMsg); err == nil &&
				(tradeMsg.Type == "snapshot" || tradeMsg.Type == "delta") {

				for _, trade := range tradeMsg.Data {
					price, err := strconv.ParseFloat(trade.Price, 64)
					if err != nil {
						continue
					}

					// Normalize trade side (Bybit uses "Buy" and "Sell")
					var side string
					if trade.Side == "Buy" {
						side = "buy"
					} else {
						side = "sell"
					}

					tradeData := TradeData{
						Symbol:    trade.Symbol,
						Source:    "bybit_spot",
						Price:     price,
						Quantity:  trade.Size,
						Side:      side,
						Timestamp: trade.Timestamp,
					}

					tradeChan <- tradeData
				}
			}
		}

		stopClose()
		if ctx.Err() != nil || !waitForReconnect(ctx, 2*time.Second) {
			return
		}
	}
}
