package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"futures-arbitrage-scanner/storage"
)

type fakeOpportunityStore struct {
	items     []storage.Opportunity
	listQuery storage.Query
	healthErr error
}

func (f *fakeOpportunityStore) Observe(context.Context, storage.Observation) error        { return nil }
func (f *fakeOpportunityStore) ObserveBatch(context.Context, []storage.Observation) error { return nil }
func (f *fakeOpportunityStore) CloseStale(context.Context, int64) error                   { return nil }
func (f *fakeOpportunityStore) Prune(context.Context, int64) error                        { return nil }
func (f *fakeOpportunityStore) Close() error                                              { return nil }
func (f *fakeOpportunityStore) Health(context.Context) error                              { return f.healthErr }
func (f *fakeOpportunityStore) List(_ context.Context, query storage.Query) ([]storage.Opportunity, error) {
	f.listQuery = query
	return f.items, nil
}

func TestAPIListsFilteredOpportunitiesWithStableEnvelope(t *testing.T) {
	store := &fakeOpportunityStore{items: []storage.Opportunity{{ID: 7, Symbol: "COTIUSDT", PeakSpreadPct: 0.82}}}
	request := httptest.NewRequest(http.MethodGet, "/api/opportunities?symbol=COTIUSDT&minSpread=0.5&limit=900", nil)
	response := httptest.NewRecorder()

	newAPIHandler(store, &fakeAlertStore{}, func() bool { return true }).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if store.listQuery != (storage.Query{Symbol: "COTIUSDT", MinSpread: 0.5, Limit: 500}) {
		t.Fatalf("query = %+v", store.listQuery)
	}
	var payload struct {
		Items []storage.Opportunity `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != 7 {
		t.Fatalf("payload = %+v", payload)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestAPIRejectsInvalidOpportunityFilters(t *testing.T) {
	for _, target := range []string{
		"/api/opportunities?symbol=NOTREAL",
		"/api/opportunities?minSpread=bad",
		"/api/opportunities?limit=-2",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }).ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", target, response.Code)
		}
	}
}

func TestAPIHealthReportsDatabaseDegradationWithoutInternalError(t *testing.T) {
	store := &fakeOpportunityStore{healthErr: errors.New("/secret/path/scanner.db is locked")}
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	newAPIHandler(store, &fakeAlertStore{}, func() bool { return true }).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if body := response.Body.String(); body != "{\"database\":\"degraded\",\"scanner\":\"live\",\"status\":\"degraded\"}\n" {
		t.Fatalf("health body = %q", body)
	}
}

func TestAPIHealthReportsStaleScanner(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return false }).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if body := response.Body.String(); body != "{\"database\":\"healthy\",\"scanner\":\"stale\",\"status\":\"degraded\"}\n" {
		t.Fatalf("health body = %q", body)
	}
}

func TestAPIRejectsMutationMethods(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/opportunities", nil)
	response := httptest.NewRecorder()

	newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.Code)
	}
}
