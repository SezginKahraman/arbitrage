package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	futuresdomain "futures-arbitrage-scanner/futures"
	"futures-arbitrage-scanner/storage"
)

type apiServer struct {
	store            storage.OpportunityStore
	alerts           storage.AlertStore
	scannerLive      func() bool
	networks         networkCatalogReader
	watchlist        *watchlistService
	futures          futuresdomain.Service
	futuresLive      futuresdomain.LiveService
	futuresExecution futuresdomain.ExecutionService
	futuresAccess    futuresdomain.AccessService
	paper            storage.PaperPositionStore
}

type networkCatalogReader interface {
	Snapshots(asset string) map[string]networkVenueSnapshot
}

func newAPIHandler(store storage.OpportunityStore, alerts storage.AlertStore, scannerLive func() bool, dependencies ...any) http.Handler {
	var networks networkCatalogReader
	var watchlist *watchlistService
	for _, dependency := range dependencies {
		switch value := dependency.(type) {
		case networkCatalogReader:
			networks = value
		case *watchlistService:
			watchlist = value
		}
	}
	server := &apiServer{store: store, alerts: alerts, scannerLive: scannerLive, networks: networks, watchlist: watchlist}
	for _, dependency := range dependencies {
		if value, ok := dependency.(futuresdomain.Service); ok {
			server.futures = value
		}
		if value, ok := dependency.(futuresdomain.LiveService); ok {
			server.futuresLive = value
		}
		if value, ok := dependency.(futuresdomain.ExecutionService); ok {
			server.futuresExecution = value
		}
		if value, ok := dependency.(futuresdomain.AccessService); ok {
			server.futuresAccess = value
		}
		if value, ok := dependency.(storage.PaperPositionStore); ok {
			server.paper = value
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/opportunities", server.handleOpportunities)
	mux.HandleFunc("/api/markets", server.handleMarkets)
	mux.HandleFunc("/api/watchlist", server.handleWatchlist)
	mux.HandleFunc("/api/networks", server.handleNetworks)
	mux.HandleFunc("/api/transfer-route", server.handleTransferRoute)
	mux.HandleFunc("/api/transfer-routes", server.handleTransferRoutes)
	mux.HandleFunc("/api/alert-rules", server.handleAlertRules)
	mux.HandleFunc("/api/alert-rules/", server.handleAlertRule)
	mux.HandleFunc("/api/alert-triggers", server.handleAlertTriggers)
	mux.HandleFunc("/api/futures/contracts", server.handleFuturesContracts)
	mux.HandleFunc("/api/futures/analyze", server.handleFuturesAnalyze)
	mux.HandleFunc("/api/futures/live", server.handleFuturesLive)
	mux.HandleFunc("/api/futures/execute", server.handleFuturesExecute)
	mux.HandleFunc("/api/futures/access", server.handleFuturesAccess)
	mux.HandleFunc("/api/futures/order-probe", server.handleFuturesOrderProbe)
	mux.HandleFunc("/api/futures/paper-positions", server.handlePaperPositions)
	mux.HandleFunc("/api/futures/paper-positions/", server.handlePaperPosition)
	mux.HandleFunc("/api/health", server.handleHealth)
	return mux
}

func (s *apiServer) handleFuturesAccess(w http.ResponseWriter, r *http.Request) {
	if !methodIs(w, r, http.MethodGet) {
		return
	}
	if s.futuresAccess == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Futures private access checks unavailable"})
		return
	}
	requiredNotional, err := strconv.ParseFloat(r.URL.Query().Get("required_notional"), 64)
	request := futuresdomain.AccessRequest{
		Contract:         strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("contract"))),
		RequiredNotional: requiredNotional,
	}
	if err != nil || requiredNotional <= 0 || requiredNotional > 1_000_000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid futures access request"})
		return
	}
	report, accessErr := s.futuresAccess.Access(r.Context(), request)
	if accessErr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Futures private access checks failed"})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *apiServer) handleFuturesOrderProbe(w http.ResponseWriter, r *http.Request) {
	if !methodIs(w, r, http.MethodPost) {
		return
	}
	if s.futuresAccess == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Live futures order probes unavailable"})
		return
	}
	var request futuresdomain.OrderProbeRequest
	if !decodeStrictJSON(w, r, &request) || !request.ConfirmLive ||
		(request.Venue != "binance_futures" && request.Venue != "gate_futures") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid live futures order probe request"})
		return
	}
	request.Contract = strings.ToUpper(strings.TrimSpace(request.Contract))
	result, err := s.futuresAccess.Probe(r.Context(), request)
	if err == nil {
		writeJSON(w, http.StatusOK, result)
		return
	}
	status := http.StatusBadGateway
	message := "Futures order probe failed before cleanup could be verified"
	if errors.Is(err, futuresdomain.ErrLiveTradingDisabled) {
		status, message = http.StatusServiceUnavailable, "Live futures order probes are disabled on the server"
	} else if errors.Is(err, futuresdomain.ErrLiveConfirmationRequired) {
		status, message = http.StatusBadRequest, "Explicit live futures confirmation is required"
	} else if errors.Is(err, futuresdomain.ErrPreflightFailed) {
		status, message = http.StatusUnprocessableEntity, "Futures order probe preflight failed"
	} else if errors.Is(err, futuresdomain.ErrOrderProbeInProgress) {
		status, message = http.StatusConflict, "A live futures order probe is already in progress for this venue and contract"
	} else if errors.Is(err, futuresdomain.ErrOrderProbeExposed) {
		status, message = http.StatusInternalServerError, "Order probe cleanup is unverified; inspect the venue immediately"
	}
	writeJSON(w, status, struct {
		Error  string                         `json:"error"`
		Result futuresdomain.OrderProbeResult `json:"result"`
	}{Error: message, Result: result})
}

func (s *apiServer) handleFuturesExecute(w http.ResponseWriter, r *http.Request) {
	if !methodIs(w, r, http.MethodPost) {
		return
	}
	if s.futuresExecution == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Live futures execution unavailable"})
		return
	}
	var input futuresdomain.ExecuteRequest
	if !decodeStrictJSON(w, r, &input) || !input.ConfirmLive || input.TradeNotional < 5 || input.TradeNotional > 1_000_000 ||
		input.MinNetSpreadPct < 0 || input.MinNetSpreadPct > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid live futures execution request"})
		return
	}
	result, err := s.futuresExecution.Execute(r.Context(), input)
	if err == nil {
		writeJSON(w, http.StatusOK, result)
		return
	}
	status := http.StatusBadGateway
	message := "One or more futures order legs failed; inspect the sanitized execution status"
	if errors.Is(err, futuresdomain.ErrLiveTradingDisabled) {
		status, message = http.StatusServiceUnavailable, "Live futures execution is disabled on the server"
	} else if errors.Is(err, futuresdomain.ErrLiveConfirmationRequired) {
		status, message = http.StatusBadRequest, "Explicit live futures confirmation is required"
	} else if errors.Is(err, futuresdomain.ErrRouteChanged) {
		status, message = http.StatusConflict, "The executable route changed; analyze the market again"
	} else if errors.Is(err, futuresdomain.ErrExecutionInProgress) {
		status, message = http.StatusConflict, "A live futures execution is already in progress for this contract"
	} else if errors.Is(err, futuresdomain.ErrPreflightFailed) {
		status = http.StatusUnprocessableEntity
		message = "Futures account preflight failed; verify venue margin, trade permission, and existing positions"
		if result.ReasonCode == "binance_futures_preflight_failed" {
			message = "Binance Futures preflight failed; verify USDT margin, trade permission, and existing positions"
		} else if result.ReasonCode == "gate_futures_preflight_failed" {
			message = "Gate Futures preflight failed; verify USDT margin, trade permission, and existing positions"
		}
	} else if errors.Is(err, futuresdomain.ErrCompensationFailed) {
		status, message = http.StatusInternalServerError, "A futures leg remains exposed; inspect both exchange positions immediately"
	}
	writeJSON(w, status, struct {
		Error  string                        `json:"error"`
		Result futuresdomain.ExecutionResult `json:"result"`
	}{Error: message, Result: result})
}

func (s *apiServer) handleFuturesLive(w http.ResponseWriter, r *http.Request) {
	if !methodIs(w, r, http.MethodGet) {
		return
	}
	if s.futuresLive == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Futures live data unavailable"})
		return
	}
	query := r.URL.Query()
	notional, notionalErr := strconv.ParseFloat(query.Get("trade_notional"), 64)
	minSpread, spreadErr := strconv.ParseFloat(query.Get("min_net_spread_pct"), 64)
	input := futuresdomain.LiveRequest{
		Contract:      strings.ToUpper(strings.TrimSpace(query.Get("contract"))),
		Interval:      strings.TrimSpace(query.Get("interval")),
		TradeNotional: notional, MinNetSpreadPct: minSpread,
	}
	if notionalErr != nil || spreadErr != nil || input.TradeNotional < 5 || input.TradeNotional > 1_000_000 ||
		input.MinNetSpreadPct < 0 || input.MinNetSpreadPct > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid futures live request"})
		return
	}
	snapshot, err := s.futuresLive.Live(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Futures live market data unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	return errors.Is(decoder.Decode(&struct{}{}), io.EOF)
}

func methodIs(w http.ResponseWriter, r *http.Request, allowed string) bool {
	if r.Method == allowed {
		return true
	}
	w.Header().Set("Allow", allowed)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

func (s *apiServer) handleFuturesContracts(w http.ResponseWriter, r *http.Request) {
	if !methodIs(w, r, http.MethodGet) {
		return
	}
	if s.futures == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Futures analysis unavailable"})
		return
	}
	items, err := s.futures.Contracts(r.Context())
	if err != nil {
		log.Printf("Gate futures contracts failed: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Gate futures market data temporarily unavailable; retry analysis"})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Items []futuresdomain.Contract `json:"items"`
	}{Items: items})
}

func (s *apiServer) handleFuturesAnalyze(w http.ResponseWriter, r *http.Request) {
	if !methodIs(w, r, http.MethodPost) {
		return
	}
	if s.futures == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Futures analysis unavailable"})
		return
	}
	var input futuresdomain.AnalyzeRequest
	if !decodeStrictJSON(w, r, &input) || futuresdomain.ValidateAnalyzeRequest(input) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid futures analysis request"})
		return
	}
	analysis, err := s.futures.Analyze(r.Context(), input)
	if err != nil {
		log.Printf("Gate futures analysis failed for %s: %v", input.Contract, err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Gate futures market data temporarily unavailable; retry analysis"})
		return
	}
	writeJSON(w, http.StatusOK, analysis)
}

func (s *apiServer) handlePaperPositions(w http.ResponseWriter, r *http.Request) {
	if s.paper == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "paper trading unavailable"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid paper position limit"})
				return
			}
			limit = min(value, 200)
		}
		items, err := s.paper.ListPaperPositions(r.Context(), limit)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "paper positions unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Items []storage.PaperPosition `json:"items"`
		}{Items: items})
	case http.MethodPost:
		var input storage.PaperPositionInput
		if !decodeStrictJSON(w, r, &input) || storage.ValidatePaperPositionInput(input) != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid paper position"})
			return
		}
		item, err := s.paper.CreatePaperPosition(r.Context(), input, time.Now().UnixMilli())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "could not create paper position"})
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *apiServer) handlePaperPosition(w http.ResponseWriter, r *http.Request) {
	if !methodIs(w, r, http.MethodPut) {
		return
	}
	if s.paper == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "paper trading unavailable"})
		return
	}
	pathValue := strings.TrimPrefix(r.URL.Path, "/api/futures/paper-positions/")
	parts := strings.Split(pathValue, "/")
	if len(parts) != 2 || parts[1] != "close" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "paper position not found"})
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "paper position not found"})
		return
	}
	var input struct {
		ExitPrice float64 `json:"exit_price"`
	}
	if !decodeStrictJSON(w, r, &input) || input.ExitPrice <= 0 || math.IsNaN(input.ExitPrice) || math.IsInf(input.ExitPrice, 0) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid paper close request"})
		return
	}
	item, err := s.paper.ClosePaperPosition(r.Context(), id, input.ExitPrice, time.Now().UnixMilli())
	if errors.Is(err, storage.ErrPaperPositionNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "paper position not found"})
		return
	}
	if errors.Is(err, storage.ErrPaperPositionClosed) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "paper position already closed"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "could not close paper position"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func supportedNetworkAsset(asset string) bool {
	return asset != "" && normalizeDiscoveredSymbol(asset+"USDT") == asset+"USDT"
}

func supportedNetworkSource(source string) bool {
	switch source {
	case sourceBinanceSpot, sourceGateSpot, sourceKuCoinSpot:
		return true
	default:
		return false
	}
}

func (s *apiServer) handleNetworks(w http.ResponseWriter, r *http.Request) {
	if !methodAllowed(w, r) {
		return
	}
	asset := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("asset")))
	if !supportedNetworkAsset(asset) || s.networks == nil || len(s.networks.Snapshots(asset)) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid network asset"})
		return
	}
	var snapshots map[string]networkVenueSnapshot
	if s.networks != nil {
		snapshots = s.networks.Snapshots(asset)
	}
	venues := make([]networkVenueSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		venues = append(venues, snapshot)
	}
	sort.Slice(venues, func(left, right int) bool { return venues[left].Source < venues[right].Source })
	writeJSON(w, http.StatusOK, struct {
		Asset  string                 `json:"asset"`
		Venues []networkVenueSnapshot `json:"venues"`
	}{Asset: asset, Venues: venues})
}

func (s *apiServer) handleTransferRoute(w http.ResponseWriter, r *http.Request) {
	if !methodAllowed(w, r) {
		return
	}
	query := r.URL.Query()
	asset := strings.ToUpper(strings.TrimSpace(query.Get("asset")))
	source := strings.ToLower(strings.TrimSpace(query.Get("source")))
	destination := strings.ToLower(strings.TrimSpace(query.Get("destination")))
	if !supportedNetworkAsset(asset) || s.networks == nil || len(s.networks.Snapshots(asset)) == 0 || !supportedNetworkSource(source) || !supportedNetworkSource(destination) || source == destination {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid transfer route"})
		return
	}
	var snapshots map[string]networkVenueSnapshot
	if s.networks != nil {
		snapshots = s.networks.Snapshots(asset)
	}
	writeJSON(w, http.StatusOK, evaluateTransferRoute(asset, source, destination, snapshots))
}

func symbolsToAssets(symbols []string) []string {
	assets := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if strings.HasSuffix(symbol, "USDT") {
			assets = append(assets, strings.TrimSuffix(symbol, "USDT"))
		}
	}
	return normalizeNetworkAssets(assets)
}

func (s *apiServer) handleTransferRoutes(w http.ResponseWriter, r *http.Request) {
	if !methodAllowed(w, r) {
		return
	}
	if s.networks == nil || s.watchlist == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "transfer routes unavailable"})
		return
	}
	symbols, err := s.watchlist.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "watchlist unavailable"})
		return
	}
	requested := make(map[string]struct{})
	if raw := strings.TrimSpace(r.URL.Query().Get("assets")); raw != "" {
		for _, asset := range strings.Split(raw, ",") {
			asset = strings.ToUpper(strings.TrimSpace(asset))
			if supportedNetworkAsset(asset) {
				requested[asset] = struct{}{}
			}
		}
	}
	assets := symbolsToAssets(symbols)
	sources := []string{sourceBinanceSpot, sourceGateSpot, sourceKuCoinSpot}
	items := make([]transferRouteEvaluation, 0, len(assets)*6)
	for _, asset := range assets {
		if len(requested) > 0 {
			if _, exists := requested[asset]; !exists {
				continue
			}
		}
		snapshots := s.networks.Snapshots(asset)
		for _, source := range sources {
			for _, destination := range sources {
				if source == destination {
					continue
				}
				items = append(items, evaluateTransferRoute(asset, source, destination, snapshots))
			}
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Items []transferRouteEvaluation `json:"items"`
	}{Items: items})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func methodAllowed(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

func (s *apiServer) handleMarkets(w http.ResponseWriter, r *http.Request) {
	if !methodAllowed(w, r) {
		return
	}
	if s.watchlist == nil || s.watchlist.markets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "market catalog unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Items        []marketCandidate   `json:"items"`
		Sources      []marketSourceState `json:"sources"`
		MaxWatchlist int                 `json:"maxWatchlist"`
	}{
		Items: s.watchlist.markets.Candidates(), Sources: s.watchlist.markets.SourceStates(), MaxWatchlist: maxWatchlistSymbols,
	})
}

func (s *apiServer) handleWatchlist(w http.ResponseWriter, r *http.Request) {
	if s.watchlist == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "watchlist unavailable"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		symbols, err := s.watchlist.List(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "watchlist unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Symbols []string `json:"symbols"`
			Limit   int      `json:"limit"`
		}{Symbols: symbols, Limit: maxWatchlistSymbols})
	case http.MethodPut:
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
		decoder.DisallowUnknownFields()
		var input struct {
			Symbols []string `json:"symbols"`
		}
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid watchlist payload"})
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid watchlist payload"})
			return
		}
		if err := s.watchlist.Replace(r.Context(), input.Symbols); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		symbols, err := s.watchlist.List(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "watchlist unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Symbols []string `json:"symbols"`
			Limit   int      `json:"limit"`
		}{Symbols: symbols, Limit: maxWatchlistSymbols})
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func supportedSymbol(symbol string) bool {
	if symbol == "" {
		return true
	}
	for _, candidate := range defaultWatchlistSymbols {
		if symbol == candidate {
			return true
		}
	}
	return false
}

func (s *apiServer) supportsSymbol(ctx context.Context, symbol string) bool {
	if symbol == "" {
		return true
	}
	if s.watchlist != nil {
		return s.watchlist.IsActive(ctx, symbol)
	}
	return supportedSymbol(symbol)
}

func (s *apiServer) supportsAlertSymbol(symbol string) bool {
	if symbol == "" {
		return true
	}
	if s.watchlist != nil && s.watchlist.markets != nil {
		return s.watchlist.markets.Supports(symbol)
	}
	return supportedSymbol(symbol)
}

func supportedAlertSource(source string) bool {
	if source == "" {
		return true
	}
	switch source {
	case sourceBinanceFutures, sourceBybitFutures, sourceHyperliquidFutures,
		sourceKrakenFutures, sourceOKXFutures, sourceGateFutures,
		sourceParadexFutures, sourceBinanceSpot, sourceBybitSpot,
		sourceGateSpot, sourceKuCoinFutures, sourceKuCoinSpot:
		return true
	default:
		return false
	}
}

func (s *apiServer) parseOpportunityQuery(r *http.Request) (storage.Query, bool) {
	values := r.URL.Query()
	query := storage.Query{Symbol: values.Get("symbol"), Limit: 100}
	if !s.supportsSymbol(r.Context(), query.Symbol) {
		return storage.Query{}, false
	}

	if raw := values.Get("minSpread"); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return storage.Query{}, false
		}
		query.MinSpread = value
	}
	if raw := values.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return storage.Query{}, false
		}
		query.Limit = min(value, 500)
	}
	return query, true
}

func (s *apiServer) handleOpportunities(w http.ResponseWriter, r *http.Request) {
	if !methodAllowed(w, r) {
		return
	}
	query, ok := s.parseOpportunityQuery(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid opportunity filters"})
		return
	}
	items, err := s.store.List(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opportunity history unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Items []storage.Opportunity `json:"items"`
	}{Items: items})
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !methodAllowed(w, r) {
		return
	}
	database := "healthy"
	scanner := "live"
	status := "healthy"
	if err := s.store.Health(r.Context()); err != nil {
		database = "degraded"
		status = "degraded"
	}
	if s.scannerLive == nil || !s.scannerLive() {
		scanner = "stale"
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":   status,
		"scanner":  scanner,
		"database": database,
	})
}

func normalizeAlertInput(input storage.AlertRuleInput) storage.AlertRuleInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Symbol = strings.ToUpper(strings.TrimSpace(input.Symbol))
	input.MarketMode = strings.ToLower(strings.TrimSpace(input.MarketMode))
	input.BuySource = strings.ToLower(strings.TrimSpace(input.BuySource))
	input.SellSource = strings.ToLower(strings.TrimSpace(input.SellSource))
	return input
}

func (s *apiServer) decodeAlertInput(w http.ResponseWriter, r *http.Request) (storage.AlertRuleInput, bool) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	var input storage.AlertRuleInput
	if err := decoder.Decode(&input); err != nil {
		return storage.AlertRuleInput{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return storage.AlertRuleInput{}, false
	}
	input = normalizeAlertInput(input)
	if !s.supportsAlertSymbol(input.Symbol) || !supportedAlertSource(input.BuySource) ||
		!supportedAlertSource(input.SellSource) || storage.ValidateAlertRuleInput(input) != nil {
		return storage.AlertRuleInput{}, false
	}
	return input, true
}

func (s *apiServer) handleAlertRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.alerts.ListAlertRules(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "alert rules unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Items []storage.AlertRule `json:"items"`
		}{Items: items})
	case http.MethodPost:
		input, ok := s.decodeAlertInput(w, r)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid alert rule"})
			return
		}
		rule, err := s.alerts.CreateAlertRule(r.Context(), input, time.Now().UnixMilli())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "could not create alert rule"})
			return
		}
		writeJSON(w, http.StatusCreated, rule)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *apiServer) handleAlertRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", http.MethodPut)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	rawID := strings.TrimPrefix(r.URL.Path, "/api/alert-rules/")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 || strings.Contains(rawID, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert rule not found"})
		return
	}
	input, ok := s.decodeAlertInput(w, r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid alert rule"})
		return
	}
	rule, err := s.alerts.UpdateAlertRule(r.Context(), id, input, time.Now().UnixMilli())
	if errors.Is(err, storage.ErrAlertRuleNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert rule not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "could not update alert rule"})
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *apiServer) handleAlertTriggers(w http.ResponseWriter, r *http.Request) {
	if !methodAllowed(w, r) {
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid trigger limit"})
			return
		}
		limit = min(value, 500)
	}
	items, err := s.alerts.ListAlertTriggers(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "alert triggers unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Items []storage.AlertTrigger `json:"items"`
	}{Items: items})
}
