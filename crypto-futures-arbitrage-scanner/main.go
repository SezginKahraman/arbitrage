package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"futures-arbitrage-scanner/exchanges"
	"futures-arbitrage-scanner/storage"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

type ArbitrageOpportunity struct {
	Symbol     string  `json:"symbol"`
	BuySource  string  `json:"buy_source"`
	SellSource string  `json:"sell_source"`
	BuyPrice   float64 `json:"buy_price"`
	SellPrice  float64 `json:"sell_price"`
	ProfitPct  float64 `json:"profit_pct"`
	Timestamp  int64   `json:"timestamp"`
}

type MarketPrice struct {
	Symbol    string  `json:"symbol"`
	Source    string  `json:"source"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
}

type opportunitiesSnapshot struct {
	Type          string                 `json:"type"`
	Version       int                    `json:"version"`
	Symbol        string                 `json:"symbol"`
	Opportunities []ArbitrageOpportunity `json:"opportunities"`
}

const (
	opportunityAlertInterval        = 10 * time.Second
	opportunityRevalidationInterval = time.Second
	quoteClientPublishInterval      = time.Second
	routeRefreshMargin              = 5 * time.Second
)

type FuturesScanner struct {
	watchlist             map[string]struct{}
	watchlistSet          bool
	watchlistMutex        sync.RWMutex
	expectedSubscriptions map[string][]string
	subscriptionMutex     sync.RWMutex
	quotes                map[string]map[string]Quote
	quotesMutex           sync.RWMutex
	quoteBroadcastAt      map[string]time.Time
	quoteBroadcastMu      sync.Mutex
	wsClients             map[*wsClient]struct{}
	clientsMutex          sync.RWMutex
	upgrader              websocket.Upgrader
	priceChan             chan exchanges.PriceData
	orderbookChan         chan exchanges.OrderbookData
	tradeChan             chan exchanges.TradeData
	connectionChan        chan exchanges.ConnectionStatus
	connections           map[string]exchanges.ConnectionStatus
	connectionMutex       sync.RWMutex
	lastOpportunity       map[string]time.Time // Track last alert per symbol
	lastRouteTime         map[string]int64
	currentRoutes         map[string]map[string]ArbitrageOpportunity
	opportunityMutex      sync.RWMutex
	history               *opportunityHistory
	alerts                storage.AlertStore
}

func NewFuturesScanner() *FuturesScanner {
	return &FuturesScanner{
		watchlist:             make(map[string]struct{}),
		expectedSubscriptions: make(map[string][]string),
		quotes:                make(map[string]map[string]Quote),
		quoteBroadcastAt:      make(map[string]time.Time),
		wsClients:             make(map[*wsClient]struct{}),
		priceChan:             make(chan exchanges.PriceData, 1000),
		orderbookChan:         make(chan exchanges.OrderbookData, 1000),
		tradeChan:             make(chan exchanges.TradeData, 1000),
		connectionChan:        make(chan exchanges.ConnectionStatus, 128),
		connections:           make(map[string]exchanges.ConnectionStatus),
		lastOpportunity:       make(map[string]time.Time),
		lastRouteTime:         make(map[string]int64),
		currentRoutes:         make(map[string]map[string]ArbitrageOpportunity),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *FuturesScanner) processPrices() {
	for priceData := range s.priceChan {
		s.updatePrice(priceData)
	}
}

func (s *FuturesScanner) processOrderbooks() {
	ticker := time.NewTicker(opportunityRevalidationInterval)
	defer ticker.Stop()
	for {
		select {
		case orderbookData, ok := <-s.orderbookChan:
			if !ok {
				return
			}
			s.updateQuote(Quote{
				Symbol:    orderbookData.Symbol,
				Source:    orderbookData.Source,
				BestBid:   orderbookData.BestBid,
				BestAsk:   orderbookData.BestAsk,
				Timestamp: orderbookData.Timestamp,
			})
		case now := <-ticker.C:
			s.revalidateOpportunitiesAt(now)
		}
	}
}

func (s *FuturesScanner) processTrades() {
	for range s.tradeChan {
		// Keep trade data for future use but don't use for pricing
	}
}

func (s *FuturesScanner) processConnections() {
	for status := range s.connectionChan {
		s.updateConnectionStatus(status)
	}
}

func (s *FuturesScanner) updatePrice(data exchanges.PriceData) {
	if !s.isWatched(data.Symbol) {
		return
	}
	s.broadcastMessage(map[string]any{
		"type":    "price_update",
		"version": 1,
		"price": MarketPrice{
			Symbol: data.Symbol, Source: data.Source, Price: data.Price, Timestamp: data.Timestamp,
		},
	})
}

func (s *FuturesScanner) updateQuote(quote Quote) {
	if !s.isWatched(quote.Symbol) {
		return
	}
	s.quotesMutex.Lock()
	if s.quotes[quote.Symbol] == nil {
		s.quotes[quote.Symbol] = make(map[string]Quote)
	}
	s.quotes[quote.Symbol][quote.Source] = quote
	s.quotesMutex.Unlock()

	s.broadcastQuote(quote)
	s.checkArbitrage(quote.Symbol)
}

func (s *FuturesScanner) isWatched(symbol string) bool {
	s.watchlistMutex.RLock()
	defer s.watchlistMutex.RUnlock()
	if !s.watchlistSet {
		return true
	}
	_, exists := s.watchlist[symbol]
	return exists
}

func (s *FuturesScanner) SetWatchlist(symbols []string) {
	next := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		next[symbol] = struct{}{}
	}

	s.watchlistMutex.Lock()
	previous := s.watchlist
	s.watchlist = next
	s.watchlistSet = true
	s.watchlistMutex.Unlock()

	removed := make([]string, 0)
	for symbol := range previous {
		if _, remains := next[symbol]; !remains {
			removed = append(removed, symbol)
		}
	}
	if len(removed) == 0 {
		return
	}
	sort.Strings(removed)
	s.quotesMutex.Lock()
	for _, symbol := range removed {
		delete(s.quotes, symbol)
	}
	s.quotesMutex.Unlock()
	s.opportunityMutex.Lock()
	for _, symbol := range removed {
		delete(s.currentRoutes, symbol)
		for key := range s.lastOpportunity {
			if strings.HasPrefix(key, symbol+"_") {
				delete(s.lastOpportunity, key)
				delete(s.lastRouteTime, key)
			}
		}
	}
	s.opportunityMutex.Unlock()
	for _, symbol := range removed {
		s.broadcastMessage(opportunitiesSnapshot{
			Type: "opportunities_snapshot", Version: 1, Symbol: symbol, Opportunities: []ArbitrageOpportunity{},
		})
	}
}

func (s *FuturesScanner) checkArbitrage(symbol string) {
	s.checkArbitrageAt(symbol, time.Now())
}

func (s *FuturesScanner) checkArbitrageAt(symbol string, now time.Time) {
	s.evaluateArbitrageAt(symbol, now, true)
}

func (s *FuturesScanner) evaluateArbitrageAt(symbol string, now time.Time, observeHistory bool) {
	s.quotesMutex.RLock()
	sourceQuotes, exists := s.quotes[symbol]
	quotesCopy := make(map[string]Quote, len(sourceQuotes))
	for source, quote := range sourceQuotes {
		quotesCopy[source] = quote
	}
	s.quotesMutex.RUnlock()

	opportunities := make([]ArbitrageOpportunity, 0)
	if exists && len(quotesCopy) >= 2 {
		opportunities = FindOpportunitiesAt(symbol, quotesCopy, now)
	}
	s.replaceCurrentRoutes(symbol, opportunities)

	for _, opportunity := range opportunities {
		if observeHistory && s.history != nil && !s.history.Observe(opportunity) {
			log.Printf("Opportunity history closed; skipping %s %s -> %s", opportunity.Symbol, opportunity.BuySource, opportunity.SellSource)
		}
		opportunityKey := opportunityRouteKey(opportunity)

		s.opportunityMutex.RLock()
		lastAlert, exists := s.lastOpportunity[opportunityKey]
		lastRouteTime := s.lastRouteTime[opportunityKey]
		s.opportunityMutex.RUnlock()

		publishedTimestampExpiresAt := time.UnixMilli(lastRouteTime).Add(quoteFreshnessWindow)
		refreshDue := opportunity.Timestamp > lastRouteTime &&
			!now.Add(routeRefreshMargin).Before(publishedTimestampExpiresAt)
		// Only send alert if it's been more than 10 seconds since last alert for this pair
		// or the periodically checked route needs a fresher client timestamp before expiry.
		if !exists || now.Sub(lastAlert) >= opportunityAlertInterval || refreshDue {
			s.opportunityMutex.Lock()
			s.lastOpportunity[opportunityKey] = now
			s.lastRouteTime[opportunityKey] = opportunity.Timestamp
			s.opportunityMutex.Unlock()

			s.broadcastOpportunity(opportunity)
		}
	}
}

func (s *FuturesScanner) revalidateOpportunitiesAt(now time.Time) {
	s.quotesMutex.RLock()
	symbols := make([]string, 0, len(s.quotes))
	for symbol := range s.quotes {
		symbols = append(symbols, symbol)
	}
	s.quotesMutex.RUnlock()
	sort.Strings(symbols)
	for _, symbol := range symbols {
		s.evaluateArbitrageAt(symbol, now, false)
	}
}

func opportunityRouteKey(opportunity ArbitrageOpportunity) string {
	return fmt.Sprintf("%s_%s_%s", opportunity.Symbol, opportunity.BuySource, opportunity.SellSource)
}

func (s *FuturesScanner) replaceCurrentRoutes(symbol string, opportunities []ArbitrageOpportunity) {
	next := make(map[string]ArbitrageOpportunity, len(opportunities))
	for _, opportunity := range opportunities {
		next[opportunityRouteKey(opportunity)] = opportunity
	}

	s.opportunityMutex.Lock()
	previous := s.currentRoutes[symbol]
	setChanged := len(previous) != len(next)
	if !setChanged {
		for key := range next {
			if _, exists := previous[key]; !exists {
				setChanged = true
				break
			}
		}
	}
	s.currentRoutes[symbol] = next
	s.opportunityMutex.Unlock()

	if setChanged {
		s.broadcastMessage(opportunitiesSnapshot{
			Type: "opportunities_snapshot", Version: 1, Symbol: symbol, Opportunities: opportunities,
		})
	}
}

func (s *FuturesScanner) IsLiveAt(now time.Time) bool {
	s.quotesMutex.RLock()
	defer s.quotesMutex.RUnlock()

	for symbol, sourceQuotes := range s.quotes {
		freshSources := 0
		for _, quote := range sourceQuotes {
			if validQuote(symbol, quote, now) {
				freshSources++
				if freshSources >= 2 {
					return true
				}
			}
		}
	}
	return false
}

func (s *FuturesScanner) IsLive() bool {
	return s.IsLiveAt(time.Now())
}

func (s *FuturesScanner) broadcastQuote(quote Quote) {
	s.broadcastQuoteAt(quote, time.Now())
}

func (s *FuturesScanner) broadcastQuoteAt(quote Quote, now time.Time) {
	key := quote.Symbol + "\x00" + quote.Source
	s.quoteBroadcastMu.Lock()
	lastPublished, exists := s.quoteBroadcastAt[key]
	if exists && now.Sub(lastPublished) < quoteClientPublishInterval {
		s.quoteBroadcastMu.Unlock()
		return
	}
	s.quoteBroadcastAt[key] = now
	s.quoteBroadcastMu.Unlock()
	s.broadcastMessage(quoteUpdateMessage(quote))
}

func quoteUpdateMessage(quote Quote) map[string]interface{} {
	return map[string]interface{}{
		"type":    "quote_update",
		"version": 1,
		"quote":   quote,
	}
}

func (s *FuturesScanner) broadcastOpportunity(opportunity ArbitrageOpportunity) {
	s.broadcastMessage(map[string]interface{}{
		"type":        "arbitrage",
		"opportunity": opportunity,
	})
	if s.alerts == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	triggers, err := s.alerts.EvaluateAlerts(ctx, storage.AlertObservation{
		Symbol: opportunity.Symbol, BuySource: opportunity.BuySource, SellSource: opportunity.SellSource,
		BuyPrice: opportunity.BuyPrice, SellPrice: opportunity.SellPrice,
		GrossSpreadPct: opportunity.ProfitPct, ObservedAtMS: opportunity.Timestamp,
	})
	if err != nil {
		log.Printf("Alert evaluation unavailable for %s: %v", opportunity.Symbol, err)
		return
	}
	for _, trigger := range triggers {
		s.broadcastMessage(map[string]any{"type": "alert_trigger", "version": 1, "trigger": trigger})
	}
}

func (s *FuturesScanner) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket connection attempt from %s", r.RemoteAddr)

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error from %s: %v", r.RemoteAddr, err)
		return
	}
	client := s.registerClient(conn)
	s.clientsMutex.RLock()
	clientCount := len(s.wsClients)
	s.clientsMutex.RUnlock()

	log.Printf("WebSocket client connected from %s. Total clients: %d", r.RemoteAddr, clientCount)

	defer func() {
		_ = conn.Close()
		log.Printf("WebSocket client disconnected. Total clients: %d", s.removeClient(client))
	}()

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-readDone:
			return
		case message := <-client.send:
			if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			if err := conn.WriteJSON(message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}
}

func newSPAHandler(directory string) http.Handler {
	fileServer := http.FileServer(http.Dir(directory))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := path.Clean("/" + r.URL.Path)
		relativePath := strings.TrimPrefix(cleanPath, "/")
		filePath := filepath.Join(directory, filepath.FromSlash(relativePath))

		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			if strings.HasPrefix(cleanPath, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-store")
			}
			fileServer.ServeHTTP(w, cloneRequestPath(r, cleanPath))
			return
		}

		if cleanPath != "/" && filepath.Ext(cleanPath) != "" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		fileServer.ServeHTTP(w, cloneRequestPath(r, "/"))
	})
}

func cloneRequestPath(r *http.Request, requestPath string) *http.Request {
	clone := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = requestPath
	clone.URL = &urlCopy
	return clone
}

func productionSubscriptionRunners(scanner *FuturesScanner) map[string]sourceSubscriptionRunner {
	withSpotREST := func(source string, connect sourceSubscriptionRunner) sourceSubscriptionRunner {
		return func(ctx context.Context, symbols []string) {
			exchanges.StartSpotRESTFallbacks(ctx, map[string][]string{source: symbols}, scanner.orderbookChan)
			connect(ctx, symbols)
		}
	}
	return map[string]sourceSubscriptionRunner{
		sourceBinanceFutures: func(ctx context.Context, symbols []string) {
			exchanges.ConnectBinanceFuturesContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		},
		sourceBybitFutures: func(ctx context.Context, symbols []string) {
			exchanges.ConnectBybitFuturesContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		},
		sourceHyperliquidFutures: func(ctx context.Context, symbols []string) {
			exchanges.ConnectHyperliquidFuturesContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		},
		sourceKrakenFutures: func(ctx context.Context, symbols []string) {
			exchanges.ConnectKrakenFuturesContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		},
		sourceOKXFutures: func(ctx context.Context, symbols []string) {
			exchanges.ConnectOKXFuturesContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		},
		sourceGateFutures: func(ctx context.Context, symbols []string) {
			exchanges.ConnectGateFuturesContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		},
		sourceKuCoinFutures: func(ctx context.Context, symbols []string) {
			exchanges.ConnectKuCoinFuturesContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		},
		sourceParadexFutures: func(ctx context.Context, symbols []string) {
			exchanges.ConnectParadexFuturesContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		},
		sourceBinanceSpot: withSpotREST(sourceBinanceSpot, func(ctx context.Context, symbols []string) {
			exchanges.ConnectBinanceSpotContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		}),
		sourceBybitSpot: withSpotREST(sourceBybitSpot, func(ctx context.Context, symbols []string) {
			exchanges.ConnectBybitSpotContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		}),
		sourceGateSpot: withSpotREST(sourceGateSpot, func(ctx context.Context, symbols []string) {
			exchanges.ConnectGateSpotContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		}),
		sourceKuCoinSpot: withSpotREST(sourceKuCoinSpot, func(ctx context.Context, symbols []string) {
			exchanges.ConnectKuCoinSpotContext(ctx, symbols, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)
		}),
	}
}

func subscriptionSymbols(catalog *marketCatalog, watchlist []string) map[string][]string {
	result := make(map[string][]string)
	for _, source := range configuredSources {
		if source == sourcePyth {
			continue
		}
		result[source] = catalog.SymbolsForSource(watchlist, source)
	}
	return result
}

func run() error {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	scanner := NewFuturesScanner()
	databasePath := os.Getenv("SCANNER_DB_PATH")
	if databasePath == "" {
		databasePath = "./data/scanner.db"
	}
	var opportunityStore storage.OpportunityStore
	var alertStore storage.AlertStore
	var watchlistRepository watchlistRepository
	sqliteStore, err := storage.OpenSQLite(databasePath)
	if err != nil {
		log.Printf("SQLite history unavailable; live scanner will continue: %v", err)
		opportunityStore = storage.NewUnavailable(err)
		alertStore = storage.NewUnavailableAlerts(err)
		watchlistRepository = newMemoryWatchlistRepository(defaultWatchlistSymbols)
	} else {
		opportunityStore = sqliteStore
		alertStore = sqliteStore
		if err := sqliteStore.SeedWatchlist(context.Background(), defaultWatchlistSymbols); err != nil {
			log.Printf("SQLite watchlist unavailable; using in-memory defaults: %v", err)
			watchlistRepository = newMemoryWatchlistRepository(defaultWatchlistSymbols)
		} else {
			watchlistRepository = sqliteStore
		}
		log.Printf("Opportunity history ready")
	}
	scanner.history = newOpportunityHistory(opportunityStore, 512)
	scanner.alerts = alertStore

	// Start processing goroutines
	go scanner.processPrices()
	go scanner.processOrderbooks()
	go scanner.processTrades()
	go scanner.processConnections()

	marketCatalog := newProductionMarketCatalog()
	initialMarketContext, cancelInitialMarkets := context.WithTimeout(context.Background(), 25*time.Second)
	marketCatalog.RefreshAt(initialMarketContext, time.Now())
	cancelInitialMarkets()
	marketContext, stopMarketCatalog := context.WithCancel(context.Background())
	defer stopMarketCatalog()
	go marketCatalog.RunPeriodic(marketContext)

	activeSymbols, err := watchlistRepository.ListWatchlist(context.Background())
	if err != nil || len(activeSymbols) == 0 {
		activeSymbols = append([]string(nil), defaultWatchlistSymbols...)
	}
	scanner.SetWatchlist(activeSymbols)
	for _, source := range configuredSources {
		scanner.updateConnectionStatus(exchanges.ConnectionStatus{
			Source: source, Connected: false, Symbols: marketCatalog.SymbolsForSource(activeSymbols, source), Timestamp: time.Now().UnixMilli(),
		})
	}
	subscriptions := newSubscriptionSupervisor(productionSubscriptionRunners(scanner))
	defer subscriptions.Stop()
	initialSubscriptions := subscriptionSymbols(marketCatalog, activeSymbols)
	scanner.SetExpectedSubscriptions(initialSubscriptions)
	subscriptions.Reconcile(initialSubscriptions)
	networkCatalog := newProductionNetworkCatalog(
		symbolsToAssets(activeSymbols), os.Getenv("BINANCE_API_KEY"), os.Getenv("BINANCE_API_SECRET"),
	)
	networkContext, stopNetworkCatalog := context.WithCancel(context.Background())
	defer stopNetworkCatalog()
	go networkCatalog.Run(networkContext)

	watchlist := newWatchlistService(watchlistRepository, marketCatalog, func(symbols []string) {
		scanner.SetWatchlist(symbols)
		nextSubscriptions := subscriptionSymbols(marketCatalog, symbols)
		scanner.SetExpectedSubscriptions(nextSubscriptions)
		subscriptions.Reconcile(nextSubscriptions)
		networkCatalog.SetAssets(symbolsToAssets(symbols))
		go func() {
			refreshContext, cancel := context.WithTimeout(networkContext, networkMetadataRequestTimeout)
			defer cancel()
			networkCatalog.RefreshAt(refreshContext, time.Now())
		}()
	})
	marketCatalog.SetOnRefresh(func() {
		symbols, listErr := watchlistRepository.ListWatchlist(marketContext)
		if listErr != nil || len(symbols) == 0 {
			return
		}
		nextSubscriptions := subscriptionSymbols(marketCatalog, symbols)
		scanner.SetExpectedSubscriptions(nextSubscriptions)
		subscriptions.Reconcile(nextSubscriptions)
	})

	// Start Pyth price feed connection
	go exchanges.ConnectPythPrices([]string{"BTCUSDT"}, scanner.priceChan, scanner.orderbookChan, scanner.tradeChan, scanner.connectionChan)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", scanner.handleWebSocket)
	mux.Handle("/api/", newAPIHandler(opportunityStore, alertStore, scanner.IsLive, networkCatalog, watchlist))
	mux.Handle("/", newSPAHandler("./web/dist"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Server starting on http://localhost:%s", port)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		closeContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		closeErr := scanner.history.Close(closeContext)
		if errors.Is(err, http.ErrServerClosed) {
			return closeErr
		}
		if closeErr != nil {
			log.Printf("Opportunity history close failed: %v", closeErr)
		}
		return err
	case <-shutdownSignal.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		serverErr := server.Shutdown(shutdownContext)
		historyErr := scanner.history.Close(shutdownContext)
		if serverErr != nil {
			return serverErr
		}
		return historyErr
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
