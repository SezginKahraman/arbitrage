package main

import (
	"log"

	"github.com/gorilla/websocket"
)

const clientQueueCapacity = 256

type wsClient struct {
	conn *websocket.Conn
	send chan any
}

func (s *FuturesScanner) registerClient(conn *websocket.Conn) *wsClient {
	client := &wsClient{conn: conn, send: make(chan any, clientQueueCapacity)}
	s.clientsMutex.Lock()
	s.wsClients[client] = struct{}{}
	s.clientsMutex.Unlock()
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
