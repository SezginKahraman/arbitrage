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

type FuturesScanner struct {
	quotes           map[string]map[string]Quote
	quotesMutex      sync.RWMutex
	wsClients        map[*wsClient]struct{}
	clientsMutex     sync.RWMutex
	upgrader         websocket.Upgrader
	priceChan        chan exchanges.PriceData
	orderbookChan    chan exchanges.OrderbookData
	tradeChan        chan exchanges.TradeData
	lastOpportunity  map[string]time.Time // Track last alert per symbol
	opportunityMutex sync.RWMutex
	history          *opportunityHistory
}

func NewFuturesScanner() *FuturesScanner {
	return &FuturesScanner{
		quotes:          make(map[string]map[string]Quote),
		wsClients:       make(map[*wsClient]struct{}),
		priceChan:       make(chan exchanges.PriceData, 1000),
		orderbookChan:   make(chan exchanges.OrderbookData, 1000),
		tradeChan:       make(chan exchanges.TradeData, 1000),
		lastOpportunity: make(map[string]time.Time),
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
	for orderbookData := range s.orderbookChan {
		s.updateQuote(Quote{
			Symbol:    orderbookData.Symbol,
			Source:    orderbookData.Source,
			BestBid:   orderbookData.BestBid,
			BestAsk:   orderbookData.BestAsk,
			Timestamp: orderbookData.Timestamp,
		})
	}
}

func (s *FuturesScanner) processTrades() {
	for range s.tradeChan {
		// Keep trade data for future use but don't use for pricing
	}
}

func (s *FuturesScanner) updatePrice(data exchanges.PriceData) {
	s.broadcastMessage(map[string]any{
		"type":    "price_update",
		"version": 1,
		"price": MarketPrice{
			Symbol: data.Symbol, Source: data.Source, Price: data.Price, Timestamp: data.Timestamp,
		},
	})
}

func (s *FuturesScanner) updateQuote(quote Quote) {
	s.quotesMutex.Lock()
	if s.quotes[quote.Symbol] == nil {
		s.quotes[quote.Symbol] = make(map[string]Quote)
	}
	s.quotes[quote.Symbol][quote.Source] = quote
	s.quotesMutex.Unlock()

	s.broadcastQuote(quote)
	s.checkArbitrage(quote.Symbol)
}

func (s *FuturesScanner) checkArbitrage(symbol string) {
	s.quotesMutex.RLock()
	sourceQuotes, exists := s.quotes[symbol]
	quotesCopy := make(map[string]Quote, len(sourceQuotes))
	for source, quote := range sourceQuotes {
		quotesCopy[source] = quote
	}
	s.quotesMutex.RUnlock()

	if !exists || len(quotesCopy) < 2 {
		return
	}

	opportunity, found := FindBestOpportunity(symbol, quotesCopy)
	if found {
		if s.history != nil && !s.history.Observe(opportunity) {
			log.Printf("Opportunity history closed; skipping %s %s -> %s", opportunity.Symbol, opportunity.BuySource, opportunity.SellSource)
		}
		opportunityKey := fmt.Sprintf("%s_%s_%s", symbol, opportunity.BuySource, opportunity.SellSource)

		s.opportunityMutex.RLock()
		lastAlert, exists := s.lastOpportunity[opportunityKey]
		s.opportunityMutex.RUnlock()

		now := time.Now()
		// Only send alert if it's been more than 10 seconds since last alert for this pair
		// This prevents spam while still allowing frequent updates for crypto markets
		if !exists || now.Sub(lastAlert) > 10*time.Second {
			s.opportunityMutex.Lock()
			s.lastOpportunity[opportunityKey] = now
			s.opportunityMutex.Unlock()

			s.broadcastOpportunity(opportunity)
		}
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
	s.broadcastMessage(map[string]interface{}{
		"type":    "quote_update",
		"version": 1,
		"quote":   quote,
	})
}

func (s *FuturesScanner) broadcastOpportunity(opportunity ArbitrageOpportunity) {
	s.broadcastMessage(map[string]interface{}{
		"type":        "arbitrage",
		"opportunity": opportunity,
	})
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
	sqliteStore, err := storage.OpenSQLite(databasePath)
	if err != nil {
		log.Printf("SQLite history unavailable; live scanner will continue: %v", err)
		opportunityStore = storage.NewUnavailable(err)
	} else {
		opportunityStore = sqliteStore
		log.Printf("Opportunity history ready")
	}
	scanner.history = newOpportunityHistory(opportunityStore, 512)

	// Start processing goroutines
	go scanner.processPrices()
	go scanner.processOrderbooks()
	go scanner.processTrades()

	// Start exchange connections with orderbook feeds
	go exchanges.ConnectBinanceFutures(symbolsForSource(sourceBinanceFutures), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)
	go exchanges.ConnectBybitFutures(symbolsForSource(sourceBybitFutures), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)
	go exchanges.ConnectHyperliquidFutures(symbolsForSource(sourceHyperliquidFutures), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)
	go exchanges.ConnectKrakenFutures(symbolsForSource(sourceKrakenFutures), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)
	go exchanges.ConnectOKXFutures(symbolsForSource(sourceOKXFutures), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)
	go exchanges.ConnectGateFutures(symbolsForSource(sourceGateFutures), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)
	go exchanges.ConnectParadexFutures(symbolsForSource(sourceParadexFutures), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)

	// Start spot exchange connections with orderbook feeds
	go exchanges.ConnectBinanceSpot(symbolsForSource(sourceBinanceSpot), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)
	go exchanges.ConnectBybitSpot(symbolsForSource(sourceBybitSpot), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)

	// Start Pyth price feed connection
	go exchanges.ConnectPythPrices(symbolsForSource(sourcePyth), scanner.priceChan, scanner.orderbookChan, scanner.tradeChan)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", scanner.handleWebSocket)
	mux.Handle("/api/", newAPIHandler(opportunityStore, scanner.IsLive))
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
