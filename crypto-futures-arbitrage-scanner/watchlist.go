package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const maxWatchlistSymbols = 20

var defaultWatchlistSymbols = []string{"BTCUSDT", "ETHUSDT", "XRPUSDT", "SOLUSDT", "COTIUSDT"}

type watchlistRepository interface {
	ListWatchlist(context.Context) ([]string, error)
	ReplaceWatchlist(context.Context, []string) error
}

type marketCatalogReader interface {
	Candidates() []marketCandidate
	SourceStates() []marketSourceState
	Supports(string) bool
}

type watchlistService struct {
	repository watchlistRepository
	markets    marketCatalogReader
	onChange   func([]string)
	mutex      sync.Mutex
}

type memoryWatchlistRepository struct {
	mutex   sync.RWMutex
	symbols []string
}

func newMemoryWatchlistRepository(symbols []string) *memoryWatchlistRepository {
	return &memoryWatchlistRepository{symbols: append([]string(nil), symbols...)}
}

func (m *memoryWatchlistRepository) ListWatchlist(context.Context) ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return append([]string(nil), m.symbols...), nil
}

func (m *memoryWatchlistRepository) ReplaceWatchlist(_ context.Context, symbols []string) error {
	m.mutex.Lock()
	m.symbols = append([]string(nil), symbols...)
	m.mutex.Unlock()
	return nil
}

func newWatchlistService(repository watchlistRepository, markets marketCatalogReader, onChange func([]string)) *watchlistService {
	return &watchlistService{repository: repository, markets: markets, onChange: onChange}
}

func (s *watchlistService) List(ctx context.Context) ([]string, error) {
	if s == nil || s.repository == nil {
		return nil, errors.New("watchlist unavailable")
	}
	return s.repository.ListWatchlist(ctx)
}

func normalizeWatchlistInput(symbols []string) ([]string, error) {
	if len(symbols) == 0 {
		return nil, errors.New("select at least one market")
	}
	if len(symbols) > maxWatchlistSymbols {
		return nil, fmt.Errorf("watchlist is limited to %d markets", maxWatchlistSymbols)
	}
	seen := make(map[string]struct{}, len(symbols))
	result := make([]string, 0, len(symbols))
	for _, raw := range symbols {
		symbol := normalizeDiscoveredSymbol(raw)
		if symbol == "" {
			return nil, fmt.Errorf("invalid USDT market %q", strings.TrimSpace(raw))
		}
		if _, exists := seen[symbol]; exists {
			return nil, fmt.Errorf("duplicate market %q", symbol)
		}
		seen[symbol] = struct{}{}
		result = append(result, symbol)
	}
	return result, nil
}

func (s *watchlistService) Replace(ctx context.Context, symbols []string) error {
	normalized, err := normalizeWatchlistInput(symbols)
	if err != nil {
		return err
	}
	if s == nil || s.repository == nil || s.markets == nil {
		return errors.New("watchlist unavailable")
	}
	for _, symbol := range normalized {
		if !s.markets.Supports(symbol) {
			return fmt.Errorf("market %s is not active on at least two scanner sources", symbol)
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err := s.repository.ReplaceWatchlist(ctx, normalized); err != nil {
		return err
	}
	if s.onChange != nil {
		s.onChange(append([]string(nil), normalized...))
	}
	return nil
}

func (s *watchlistService) IsActive(ctx context.Context, symbol string) bool {
	if symbol == "" {
		return true
	}
	items, err := s.List(ctx)
	if err != nil {
		return false
	}
	for _, item := range items {
		if item == symbol {
			return true
		}
	}
	return false
}
