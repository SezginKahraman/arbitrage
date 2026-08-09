package main

import (
	"log"
	"sort"

	"github.com/gorilla/websocket"
)

const clientQueueCapacity = 256

type wsClient struct {
	conn *websocket.Conn
	send chan any
}

func (s *FuturesScanner) registerClient(conn *websocket.Conn) *wsClient {
	client := &wsClient{conn: conn, send: make(chan any, clientQueueCapacity)}
	s.opportunityMutex.RLock()
	s.clientsMutex.Lock()
	s.wsClients[client] = struct{}{}
	symbols := make([]string, 0, len(s.currentRoutes))
	for symbol := range s.currentRoutes {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	for _, symbol := range symbols {
		routes := s.currentRoutes[symbol]
		opportunities := make([]ArbitrageOpportunity, 0, len(routes))
		for _, opportunity := range routes {
			opportunities = append(opportunities, opportunity)
		}
		sort.Slice(opportunities, func(left, right int) bool {
			return opportunities[left].ProfitPct > opportunities[right].ProfitPct
		})
		client.send <- opportunitiesSnapshot{
			Type: "opportunities_snapshot", Version: 1, Symbol: symbol, Opportunities: opportunities,
		}
	}
	s.clientsMutex.Unlock()
	s.opportunityMutex.RUnlock()
	return client
}

func (s *FuturesScanner) removeClient(client *wsClient) int {
	s.clientsMutex.Lock()
	delete(s.wsClients, client)
	remaining := len(s.wsClients)
	s.clientsMutex.Unlock()
	return remaining
}

func (s *FuturesScanner) broadcastMessage(message any) {
	var slowClients []*wsClient
	s.clientsMutex.Lock()
	for client := range s.wsClients {
		select {
		case client.send <- message:
		default:
			delete(s.wsClients, client)
			slowClients = append(slowClients, client)
		}
	}
	s.clientsMutex.Unlock()

	for _, client := range slowClients {
		log.Printf("WebSocket client queue full; disconnecting slow consumer")
		if client.conn != nil {
			_ = client.conn.Close()
		}
	}
}
