package main

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"futures-arbitrage-scanner/storage"
)

type apiServer struct {
	store       storage.OpportunityStore
	alerts      storage.AlertStore
	scannerLive func() bool
	networks    networkCatalogReader
}

type networkCatalogReader interface {
	Snapshots(asset string) map[string]networkVenueSnapshot
}

func newAPIHandler(store storage.OpportunityStore, alerts storage.AlertStore, scannerLive func() bool, networkReaders ...networkCatalogReader) http.Handler {
	var networks networkCatalogReader
	if len(networkReaders) > 0 {
		networks = networkReaders[0]
	}
	server := &apiServer{store: store, alerts: alerts, scannerLive: scannerLive, networks: networks}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/opportunities", server.handleOpportunities)
	mux.HandleFunc("/api/networks", server.handleNetworks)
	mux.HandleFunc("/api/transfer-route", server.handleTransferRoute)
	mux.HandleFunc("/api/alert-rules", server.handleAlertRules)
	mux.HandleFunc("/api/alert-rules/", server.handleAlertRule)
	mux.HandleFunc("/api/alert-triggers", server.handleAlertTriggers)
	mux.HandleFunc("/api/health", server.handleHealth)
	return mux
}

func supportedNetworkAsset(asset string) bool {
	switch asset {
	case "BTC", "ETH", "XRP", "SOL", "COTI":
		return true
	default:
		return false
	}
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
	if !supportedNetworkAsset(asset) {
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
	if !supportedNetworkAsset(asset) || !supportedNetworkSource(source) || !supportedNetworkSource(destination) || source == destination {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid transfer route"})
		return
	}
	var snapshots map[string]networkVenueSnapshot
	if s.networks != nil {
		snapshots = s.networks.Snapshots(asset)
	}
	writeJSON(w, http.StatusOK, evaluateTransferRoute(asset, source, destination, snapshots))
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

func supportedSymbol(symbol string) bool {
	if symbol == "" {
		return true
	}
	if symbol == "COTIUSDT" {
		return true
	}
	for _, candidate := range coreSymbols {
		if symbol == candidate {
			return true
		}
	}
	return false
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

func parseOpportunityQuery(r *http.Request) (storage.Query, bool) {
	values := r.URL.Query()
	query := storage.Query{Symbol: values.Get("symbol"), Limit: 100}
	if !supportedSymbol(query.Symbol) {
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
	query, ok := parseOpportunityQuery(r)
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

func decodeAlertInput(w http.ResponseWriter, r *http.Request) (storage.AlertRuleInput, bool) {
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
	if !supportedSymbol(input.Symbol) || !supportedAlertSource(input.BuySource) ||
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
		input, ok := decodeAlertInput(w, r)
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
	input, ok := decodeAlertInput(w, r)
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
