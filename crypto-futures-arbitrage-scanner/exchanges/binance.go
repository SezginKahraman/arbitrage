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

type BinanceFuturesTrade struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	TradeID   int64  `json:"a"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	TradeTime int64  `json:"T"`
	IsMaker   bool   `json:"m"`
}

type BinanceFuturesBookTicker struct {
	EventType    string `json:"e"`
	EventTime    int64  `json:"E"`
	Symbol       string `json:"s"`
	BestBidPrice string `json:"b"`
	BestBidQty   string `json:"B"`
	BestAskPrice string `json:"a"`
	BestAskQty   string `json:"A"`
}

func ConnectBinanceFutures(symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	ConnectBinanceFuturesContext(context.Background(), symbols, priceChan, orderbookChan, tradeChan, statusChan)
}

func ConnectBinanceFuturesContext(ctx context.Context, symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	if len(symbols) == 0 {
		return
	}
	defer publishConnectionStatus(statusChan, "binance_futures", false, symbols)
	streamNames := make([]string, len(symbols)*2)
	for i, symbol := range symbols {
		streamNames[i*2] = strings.ToLower(symbol) + "@bookTicker"
		streamNames[i*2+1] = strings.ToLower(symbol) + "@aggTrade"
	}
	streamParam := strings.Join(streamNames, "/")

	wsURL := fmt.Sprintf("wss://fstream.binance.com/stream?streams=%s", streamParam)

	for {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			publishConnectionStatus(statusChan, "binance_futures", false, symbols)
			log.Printf("Binance futures connection error: %v", err)
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}
		stopClose := closeWebSocketOnCancel(ctx, conn)

		log.Printf("Connected to Binance futures WebSocket")
		publishConnectionStatus(statusChan, "binance_futures", true, symbols)

		for {
			var message struct {
				Stream string          `json:"stream"`
				Data   json.RawMessage `json:"data"`
			}

			err := conn.ReadJSON(&message)
			if err != nil {
				publishConnectionStatus(statusChan, "binance_futures", false, symbols)
				if ctx.Err() == nil {
					log.Printf("Binance futures read error: %v", err)
				}
				conn.Close()
				break
			}

			if strings.Contains(message.Stream, "@bookTicker") {
				var bookTicker BinanceFuturesBookTicker
				if err := json.Unmarshal(message.Data, &bookTicker); err != nil {
					continue
				}

				bidPrice, err1 := strconv.ParseFloat(bookTicker.BestBidPrice, 64)
				askPrice, err2 := strconv.ParseFloat(bookTicker.BestAskPrice, 64)
				if err1 != nil || err2 != nil {
					continue
				}

				orderbookData := OrderbookData{
					Symbol:    bookTicker.Symbol,
					Source:    "binance_futures",
					BestBid:   bidPrice,
					BestAsk:   askPrice,
					Timestamp: bookTicker.EventTime,
				}

				orderbookChan <- orderbookData

			} else if strings.Contains(message.Stream, "@aggTrade") {
				var trade BinanceFuturesTrade
				if err := json.Unmarshal(message.Data, &trade); err != nil {
					continue
				}

				price, err := strconv.ParseFloat(trade.Price, 64)
				if err != nil {
					continue
				}

				// Normalize trade side (isMaker: false = buy aggressor, true = sell aggressor)
				var side string
				if !trade.IsMaker {
					side = "buy"
				} else {
					side = "sell"
				}

				tradeData := TradeData{
					Symbol:    trade.Symbol,
					Source:    "binance_futures",
					Price:     price,
					Quantity:  trade.Quantity,
					Side:      side,
					Timestamp: trade.TradeTime,
				}

				tradeChan <- tradeData
			}
		}
		stopClose()
		if ctx.Err() != nil || !waitForReconnect(ctx, 2*time.Second) {
			return
		}
	}
}

// BinanceSpotTrade represents the structure for Binance spot trade data
type BinanceSpotTrade struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	TradeID   int64  `json:"a"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	TradeTime int64  `json:"T"`
	IsMaker   bool   `json:"m"`
}

type BinanceSpotBookTicker struct {
	EventType    string `json:"e"`
	EventTime    int64  `json:"E"`
	Symbol       string `json:"s"`
	BestBidPrice string `json:"b"`
	BestBidQty   string `json:"B"`
	BestAskPrice string `json:"a"`
	BestAskQty   string `json:"A"`
}

func parseBinanceSpotBookTicker(message []byte, receivedAt time.Time) (OrderbookData, bool) {
	var bookTicker BinanceSpotBookTicker
	if err := json.Unmarshal(message, &bookTicker); err != nil || bookTicker.Symbol == "" {
		return OrderbookData{}, false
	}

	bestBid, bidErr := strconv.ParseFloat(bookTicker.BestBidPrice, 64)
	bestAsk, askErr := strconv.ParseFloat(bookTicker.BestAskPrice, 64)
	if bidErr != nil || askErr != nil || bestBid <= 0 || bestAsk <= 0 || bestBid > bestAsk {
		return OrderbookData{}, false
	}

	timestamp := bookTicker.EventTime
	if timestamp <= 0 {
		timestamp = receivedAt.UnixMilli()
	}
	return OrderbookData{
		Symbol: bookTicker.Symbol, Source: "binance_spot", BestBid: bestBid, BestAsk: bestAsk, Timestamp: timestamp,
	}, true
}

// ConnectBinanceSpot connects to Binance spot trading WebSocket API
func ConnectBinanceSpot(symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	ConnectBinanceSpotContext(context.Background(), symbols, priceChan, orderbookChan, tradeChan, statusChan)
}

func ConnectBinanceSpotContext(ctx context.Context, symbols []string, priceChan chan<- PriceData, orderbookChan chan<- OrderbookData, tradeChan chan<- TradeData, statusChan chan<- ConnectionStatus) {
	if len(symbols) == 0 {
		return
	}
	defer publishConnectionStatus(statusChan, "binance_spot", false, symbols)
	streamNames := make([]string, len(symbols)*2)
	for i, symbol := range symbols {
		streamNames[i*2] = strings.ToLower(symbol) + "@bookTicker"
		streamNames[i*2+1] = strings.ToLower(symbol) + "@aggTrade"
	}
	streamParam := strings.Join(streamNames, "/")

	wsURL := fmt.Sprintf("wss://stream.binance.com:9443/stream?streams=%s", streamParam)

	for {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			publishConnectionStatus(statusChan, "binance_spot", false, symbols)
			log.Printf("Binance spot connection error: %v", err)
			if !waitForReconnect(ctx, 5*time.Second) {
				return
			}
			continue
		}
		stopClose := closeWebSocketOnCancel(ctx, conn)

		log.Printf("Connected to Binance spot WebSocket")
		publishConnectionStatus(statusChan, "binance_spot", true, symbols)

		for {
			var message struct {
				Stream string          `json:"stream"`
				Data   json.RawMessage `json:"data"`
			}

			err := conn.ReadJSON(&message)
			if err != nil {
				publishConnectionStatus(statusChan, "binance_spot", false, symbols)
				if ctx.Err() == nil {
					log.Printf("Binance spot read error: %v", err)
				}
				conn.Close()
				break
			}

			if strings.Contains(message.Stream, "@bookTicker") {
				orderbookData, ok := parseBinanceSpotBookTicker(message.Data, time.Now())
				if !ok {
					continue
				}

				orderbookChan <- orderbookData

			} else if strings.Contains(message.Stream, "@aggTrade") {
				var trade BinanceSpotTrade
				if err := json.Unmarshal(message.Data, &trade); err != nil {
					continue
				}

				price, err := strconv.ParseFloat(trade.Price, 64)
				if err != nil {
					continue
				}

				// Normalize trade side (isMaker: false = buy aggressor, true = sell aggressor)
				var side string
				if !trade.IsMaker {
					side = "buy"
				} else {
					side = "sell"
				}

				tradeData := TradeData{
					Symbol:    trade.Symbol,
					Source:    "binance_spot",
					Price:     price,
					Quantity:  trade.Quantity,
					Side:      side,
					Timestamp: trade.TradeTime,
				}

				tradeChan <- tradeData
			}
		}
		stopClose()
		if ctx.Err() != nil || !waitForReconnect(ctx, 2*time.Second) {
			return
		}
	}
}
