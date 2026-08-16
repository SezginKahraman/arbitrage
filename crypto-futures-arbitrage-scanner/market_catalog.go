package main

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	marketSpot    = "spot"
	marketFutures = "futures"

	marketSourceLoading     = "loading"
	marketSourceReady       = "ready"
	marketSourceStale       = "stale"
	marketSourceUnavailable = "unavailable"

	marketCatalogRefreshInterval = 10 * time.Minute
)

var discoveredSymbolPattern = regexp.MustCompile(`^[A-Z0-9]{2,24}USDT$`)

type marketSourceFetcher func(context.Context) ([]string, error)

type marketSourceDefinition struct {
	Source string
	Market string
	Fetch  marketSourceFetcher
}

type marketSourceState struct {
	Source    string   `json:"source"`
	Market    string   `json:"market"`
	Status    string   `json:"status"`
	Symbols   []string `json:"symbols"`
	CheckedAt int64    `json:"checkedAt"`
	ErrorCode string   `json:"errorCode,omitempty"`
}

type marketCandidate struct {
	Symbol         string   `json:"symbol"`
	Base           string   `json:"base"`
	SpotSources    []string `json:"spotSources"`
	FuturesSources []string `json:"futuresSources"`
	Sources        []string `json:"sources"`
}

type marketCatalog struct {
	definitions []marketSourceDefinition
	mutex       sync.RWMutex
	states      map[string]marketSourceState
	onRefresh   func()
}

func newMarketCatalog(definitions []marketSourceDefinition) *marketCatalog {
	states := make(map[string]marketSourceState, len(definitions))
	for _, definition := range definitions {
		states[definition.Source] = marketSourceState{
			Source: definition.Source, Market: definition.Market, Status: marketSourceLoading, Symbols: []string{},
		}
	}
	return &marketCatalog{definitions: append([]marketSourceDefinition(nil), definitions...), states: states}
}

func normalizeDiscoveredSymbol(raw string) string {
	symbol := strings.ToUpper(strings.TrimSpace(raw))
	symbol = strings.NewReplacer("-", "", "_", "", "/", "").Replace(symbol)
	if !discoveredSymbolPattern.MatchString(symbol) || symbol == "USDTUSDT" {
		return ""
	}
	return symbol
}

func normalizeDiscoveredSymbols(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		symbol := normalizeDiscoveredSymbol(item)
		if symbol == "" {
			continue
		}
		if _, exists := seen[symbol]; exists {
			continue
		}
		seen[symbol] = struct{}{}
		result = append(result, symbol)
	}
	sort.Strings(result)
	return result
}

func (c *marketCatalog) RefreshAt(ctx context.Context, checkedAt time.Time) {
	type result struct {
		definition marketSourceDefinition
		symbols    []string
		err        error
	}
	results := make(chan result, len(c.definitions))
	var waitGroup sync.WaitGroup
	for _, definition := range c.definitions {
		definition := definition
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			symbols, err := definition.Fetch(ctx)
			results <- result{definition: definition, symbols: normalizeDiscoveredSymbols(symbols), err: err}
		}()
	}
	waitGroup.Wait()
	close(results)

	c.mutex.Lock()
	for refresh := range results {
		previous := c.states[refresh.definition.Source]
		if refresh.err != nil {
			previous.Source = refresh.definition.Source
			previous.Market = refresh.definition.Market
			previous.CheckedAt = checkedAt.UnixMilli()
			previous.ErrorCode = "request_failed"
			if len(previous.Symbols) > 0 {
				previous.Status = marketSourceStale
			} else {
				previous.Status = marketSourceUnavailable
			}
			c.states[refresh.definition.Source] = previous
			continue
		}
		c.states[refresh.definition.Source] = marketSourceState{
			Source: refresh.definition.Source, Market: refresh.definition.Market, Status: marketSourceReady,
			Symbols: refresh.symbols, CheckedAt: checkedAt.UnixMilli(),
		}
	}
	onRefresh := c.onRefresh
	c.mutex.Unlock()
	if onRefresh != nil {
		onRefresh()
	}
}

func (c *marketCatalog) SetOnRefresh(onRefresh func()) {
	c.mutex.Lock()
	c.onRefresh = onRefresh
	c.mutex.Unlock()
}

func (c *marketCatalog) Run(ctx context.Context) {
	c.RefreshAt(ctx, time.Now())
	c.RunPeriodic(ctx)
}

func (c *marketCatalog) RunPeriodic(ctx context.Context) {
	ticker := time.NewTicker(marketCatalogRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case checkedAt := <-ticker.C:
			c.RefreshAt(ctx, checkedAt)
		}
	}
}

func (c *marketCatalog) SourceStates() []marketSourceState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	result := make([]marketSourceState, 0, len(c.states))
	for _, state := range c.states {
		state.Symbols = append([]string(nil), state.Symbols...)
		result = append(result, state)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Source < result[right].Source })
	return result
}

func (c *marketCatalog) Candidates() []marketCandidate {
	states := c.SourceStates()
	type coverage struct {
		spot    []string
		futures []string
	}
	bySymbol := make(map[string]*coverage)
	for _, state := range states {
		for _, symbol := range state.Symbols {
			item := bySymbol[symbol]
			if item == nil {
				item = &coverage{}
				bySymbol[symbol] = item
			}
			if state.Market == marketSpot {
				item.spot = append(item.spot, state.Source)
			} else {
				item.futures = append(item.futures, state.Source)
			}
		}
	}
	result := make([]marketCandidate, 0, len(bySymbol))
	for symbol, item := range bySymbol {
		spotSources := append([]string{}, item.spot...)
		futuresSources := append([]string{}, item.futures...)
		sources := append(append([]string{}, spotSources...), futuresSources...)
		if len(sources) < 2 {
			continue
		}
		sort.Strings(spotSources)
		sort.Strings(futuresSources)
		sort.Strings(sources)
		result = append(result, marketCandidate{
			Symbol: symbol, Base: strings.TrimSuffix(symbol, "USDT"), SpotSources: spotSources,
			FuturesSources: futuresSources, Sources: sources,
		})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Symbol < result[right].Symbol })
	return result
}

func (c *marketCatalog) Supports(symbol string) bool {
	symbol = normalizeDiscoveredSymbol(symbol)
	if symbol == "" {
		return false
	}
	for _, candidate := range c.Candidates() {
		if candidate.Symbol == symbol {
			return true
		}
	}
	return false
}

func (c *marketCatalog) SymbolsForSource(watchlist []string, source string) []string {
	states := c.SourceStates()
	available := make(map[string]struct{})
	for _, state := range states {
		if state.Source != source {
			continue
		}
		for _, symbol := range state.Symbols {
			available[symbol] = struct{}{}
		}
		break
	}
	result := make([]string, 0, len(watchlist))
	for _, symbol := range watchlist {
		if _, exists := available[symbol]; exists {
			result = append(result, symbol)
		}
	}
	return result
}
