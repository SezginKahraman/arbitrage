package exchanges

import "time"

type ConnectionStatus struct {
	Source    string   `json:"source"`
	Connected bool     `json:"connected"`
	Symbols   []string `json:"symbols"`
	Timestamp int64    `json:"timestamp"`
}

func publishConnectionStatus(statusChan chan<- ConnectionStatus, source string, connected bool, symbols []string) {
	if statusChan == nil {
		return
	}
	status := ConnectionStatus{
		Source: source, Connected: connected, Symbols: append([]string(nil), symbols...), Timestamp: time.Now().UnixMilli(),
	}
	select {
	case statusChan <- status:
	default:
		// Connection health is a latest-state signal; a slow consumer must never block market data.
	}
}

type PriceData struct {
	Symbol    string
	Source    string
	Price     float64
	Timestamp int64
}

type OrderbookData struct {
	Symbol    string
	Source    string
	BestBid   float64
	BestAsk   float64
	Timestamp int64
}

type TradeData struct {
	Symbol    string
	Source    string
	Price     float64
	Quantity  string
	Side      string // "buy" or "sell" (normalized)
	Timestamp int64
}
