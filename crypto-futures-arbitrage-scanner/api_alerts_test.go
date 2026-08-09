package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"futures-arbitrage-scanner/storage"
)

type fakeAlertStore struct {
	rules        []storage.AlertRule
	triggers     []storage.AlertTrigger
	evaluated    []storage.AlertObservation
	triggered    []storage.AlertTrigger
	createdInput storage.AlertRuleInput
	updatedID    int64
	updatedInput storage.AlertRuleInput
}

func (f *fakeAlertStore) ListAlertRules(context.Context) ([]storage.AlertRule, error) {
	return f.rules, nil
}
func (f *fakeAlertStore) CreateAlertRule(_ context.Context, input storage.AlertRuleInput, now int64) (storage.AlertRule, error) {
	f.createdInput = input
	return storage.AlertRule{ID: 9, AlertRuleInput: input, CreatedAtMS: now, UpdatedAtMS: now}, nil
}
func (f *fakeAlertStore) UpdateAlertRule(_ context.Context, id int64, input storage.AlertRuleInput, now int64) (storage.AlertRule, error) {
	f.updatedID = id
	f.updatedInput = input
	return storage.AlertRule{ID: id, AlertRuleInput: input, CreatedAtMS: 1, UpdatedAtMS: now}, nil
}
func (f *fakeAlertStore) ListAlertTriggers(context.Context, int) ([]storage.AlertTrigger, error) {
	return f.triggers, nil
}
func (f *fakeAlertStore) EvaluateAlerts(_ context.Context, observation storage.AlertObservation) ([]storage.AlertTrigger, error) {
	f.evaluated = append(f.evaluated, observation)
	return f.triggered, nil
}

func TestAlertRuleAPICreatesAndListsRules(t *testing.T) {
	alerts := &fakeAlertStore{}
	handler := newAPIHandler(&fakeOpportunityStore{}, alerts, func() bool { return true })
	body := `{"name":"COTI gap","symbol":"COTIUSDT","market_mode":"spot","buy_source":"gate_spot","sell_source":"binance_spot","min_spread_pct":0.8,"cooldown_seconds":300,"enabled":true,"browser_enabled":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/alert-rules", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", response.Code, response.Body.String())
	}
	if alerts.createdInput.Symbol != "COTIUSDT" || alerts.createdInput.MinSpreadPct != 0.8 {
		t.Fatalf("created input = %+v", alerts.createdInput)
	}
	var created storage.AlertRule
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID != 9 {
		t.Fatalf("created rule = %+v", created)
	}

	alerts.rules = []storage.AlertRule{created}
	request = httptest.NewRequest(http.MethodGet, "/api/alert-rules", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"items":[{"id":9`) {
		t.Fatalf("list response = %d %s", response.Code, response.Body.String())
	}
}

func TestAlertRuleAPIUpdatesACompleteRule(t *testing.T) {
	alerts := &fakeAlertStore{}
	handler := newAPIHandler(&fakeOpportunityStore{}, alerts, func() bool { return true })
	body := `{"name":"BTC futures","symbol":"BTCUSDT","market_mode":"futures","buy_source":"","sell_source":"","min_spread_pct":0.4,"cooldown_seconds":60,"enabled":false,"browser_enabled":true}`
	request := httptest.NewRequest(http.MethodPut, "/api/alert-rules/42", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if alerts.updatedID != 42 || alerts.updatedInput.Enabled || alerts.updatedInput.MarketMode != storage.AlertMarketFutures {
		t.Fatalf("updated id/input = %d %+v", alerts.updatedID, alerts.updatedInput)
	}
}

func TestAlertAPIRejectsUnknownSymbolsAndMalformedJSON(t *testing.T) {
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true })
	for _, body := range []string{
		`{"name":"bad","symbol":"NOPEUSDT","market_mode":"all","min_spread_pct":1,"cooldown_seconds":60,"enabled":true,"browser_enabled":true}`,
		`{"name":"bad source","symbol":"COTIUSDT","market_mode":"all","buy_source":"made_up_spot","min_spread_pct":1,"cooldown_seconds":60,"enabled":true,"browser_enabled":true}`,
		`{"name":`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/alert-rules", strings.NewReader(body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want 400", body, response.Code)
		}
	}
}

func TestAlertTriggerAPIListsRecentTriggers(t *testing.T) {
	alerts := &fakeAlertStore{triggers: []storage.AlertTrigger{{ID: 3, RuleID: 1, RuleName: "COTI gap", Symbol: "COTIUSDT"}}}
	handler := newAPIHandler(&fakeOpportunityStore{}, alerts, func() bool { return true })
	request := httptest.NewRequest(http.MethodGet, "/api/alert-triggers?limit=20", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"rule_name":"COTI gap"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}
