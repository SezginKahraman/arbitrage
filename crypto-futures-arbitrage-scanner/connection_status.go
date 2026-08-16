package main

import (
	"slices"
	"sort"
	"time"

	"futures-arbitrage-scanner/exchanges"
)

type sourceStatusMessage struct {
	Type    string                     `json:"type"`
	Version int                        `json:"version"`
	Status  exchanges.ConnectionStatus `json:"status"`
}

func (s *FuturesScanner) updateConnectionStatus(status exchanges.ConnectionStatus) {
	if status.Source == "" {
		return
	}
	status.Symbols = append([]string(nil), status.Symbols...)
	sort.Strings(status.Symbols)
	s.subscriptionMutex.RLock()
	expected, constrained := s.expectedSubscriptions[status.Source]
	if constrained && !slices.Equal(expected, status.Symbols) {
		s.subscriptionMutex.RUnlock()
		return
	}
	s.subscriptionMutex.RUnlock()

	s.connectionMutex.Lock()
	previous, exists := s.connections[status.Source]
	unchanged := exists && previous.Connected == status.Connected && slices.Equal(previous.Symbols, status.Symbols)
	if !unchanged {
		s.connections[status.Source] = status
	}
	s.connectionMutex.Unlock()

	if !unchanged {
		s.broadcastMessage(sourceStatusMessage{Type: "source_status", Version: 1, Status: status})
	}
}

func (s *FuturesScanner) SetExpectedSubscriptions(symbolsBySource map[string][]string) {
	next := make(map[string][]string, len(symbolsBySource))
	for source, symbols := range symbolsBySource {
		next[source] = normalizedSubscriptionSymbols(symbols)
	}
	s.subscriptionMutex.Lock()
	previous := s.expectedSubscriptions
	s.expectedSubscriptions = next
	s.subscriptionMutex.Unlock()
	for source, symbols := range next {
		if slices.Equal(previous[source], symbols) {
			continue
		}
		s.updateConnectionStatus(exchanges.ConnectionStatus{
			Source: source, Connected: false, Symbols: symbols, Timestamp: time.Now().UnixMilli(),
		})
	}
}

func (s *FuturesScanner) connectionSnapshot() []sourceStatusMessage {
	s.connectionMutex.RLock()
	sources := make([]string, 0, len(s.connections))
	for source := range s.connections {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	result := make([]sourceStatusMessage, 0, len(sources))
	for _, source := range sources {
		status := s.connections[source]
		status.Symbols = append([]string(nil), status.Symbols...)
		result = append(result, sourceStatusMessage{Type: "source_status", Version: 1, Status: status})
	}
	s.connectionMutex.RUnlock()
	return result
}
