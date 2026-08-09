package main

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"futures-arbitrage-scanner/storage"
)

type apiServer struct {
	store       storage.OpportunityStore
	scannerLive func() bool
}

func newAPIHandler(store storage.OpportunityStore, scannerLive func() bool) http.Handler {
	server := &apiServer{store: store, scannerLive: scannerLive}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/opportunities", server.handleOpportunities)
	mux.HandleFunc("/api/health", server.handleHealth)
	return mux
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
