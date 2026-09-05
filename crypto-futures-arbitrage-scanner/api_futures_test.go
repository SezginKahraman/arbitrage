package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	futuresdomain "futures-arbitrage-scanner/futures"
	"futures-arbitrage-scanner/storage"
)

type fakeFuturesService struct {
	contracts  []futuresdomain.Contract
	analysis   futuresdomain.Analysis
	err        error
	analyzeReq futuresdomain.AnalyzeRequest
}

type fakeFuturesLiveService struct {
	snapshot futuresdomain.LiveSnapshot
	request  futuresdomain.LiveRequest
}

type fakeFuturesExecutionService struct {
	result  futuresdomain.ExecutionResult
	err     error
	request futuresdomain.ExecuteRequest
}

type fakeFuturesAccessService struct {
	report       futuresdomain.AccessReport
	accessErr    error
	accessReq    futuresdomain.AccessRequest
	probeResult  futuresdomain.OrderProbeResult
	probeErr     error
	probeRequest futuresdomain.OrderProbeRequest
}

func (service *fakeFuturesAccessService) Access(_ context.Context, request futuresdomain.AccessRequest) (futuresdomain.AccessReport, error) {
	service.accessReq = request
	return service.report, service.accessErr
}

func (service *fakeFuturesAccessService) Probe(_ context.Context, request futuresdomain.OrderProbeRequest) (futuresdomain.OrderProbeResult, error) {
	service.probeRequest = request
	return service.probeResult, service.probeErr
}

func (service *fakeFuturesExecutionService) Execute(_ context.Context, request futuresdomain.ExecuteRequest) (futuresdomain.ExecutionResult, error) {
	service.request = request
	return service.result, service.err
}

func (service *fakeFuturesLiveService) Live(_ context.Context, request futuresdomain.LiveRequest) (futuresdomain.LiveSnapshot, error) {
	service.request = request
	return service.snapshot, nil
}

func (service *fakeFuturesService) Contracts(context.Context) ([]futuresdomain.Contract, error) {
	return service.contracts, service.err
}

func (service *fakeFuturesService) Analyze(_ context.Context, request futuresdomain.AnalyzeRequest) (futuresdomain.Analysis, error) {
	service.analyzeReq = request
	return service.analysis, service.err
}

func openAPIPaperStore(t *testing.T) *storage.SQLiteStore {
	t.Helper()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestFuturesAPIListsContractsAndAnalyzesExplicitly(t *testing.T) {
	service := &fakeFuturesService{
		contracts: []futuresdomain.Contract{{Symbol: "COTI_USDT", Status: "trading", MarkPrice: 0.0106}},
		analysis: futuresdomain.Analysis{
			Contract: futuresdomain.Contract{Symbol: "COTI_USDT", MarkPrice: 0.0106},
			Long:     futuresdomain.Scenario{Direction: "long", Status: futuresdomain.ScenarioWatch},
		},
	}
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, service, openAPIPaperStore(t))

	contractsResponse := httptest.NewRecorder()
	handler.ServeHTTP(contractsResponse, httptest.NewRequest(http.MethodGet, "/api/futures/contracts", nil))
	if contractsResponse.Code != http.StatusOK || !strings.Contains(contractsResponse.Body.String(), "COTI_USDT") {
		t.Fatalf("contracts response = %d %s", contractsResponse.Code, contractsResponse.Body.String())
	}

	body := []byte(`{"contract":"COTI_USDT","interval":"15m","strategy":"trend_pullback","account_balance":1000,"risk_percent":1}`)
	analysisResponse := httptest.NewRecorder()
	handler.ServeHTTP(analysisResponse, httptest.NewRequest(http.MethodPost, "/api/futures/analyze", bytes.NewReader(body)))
	if analysisResponse.Code != http.StatusOK {
		t.Fatalf("analysis response = %d %s", analysisResponse.Code, analysisResponse.Body.String())
	}
	if service.analyzeReq.Contract != "COTI_USDT" || service.analyzeReq.RiskPercent != 1 {
		t.Fatalf("analysis request = %+v", service.analyzeReq)
	}
	if got := analysisResponse.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestFuturesAPIRejectsInvalidInputBeforeCallingGate(t *testing.T) {
	service := &fakeFuturesService{}
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, service, openAPIPaperStore(t))
	body := []byte(`{"contract":"../../BTC","interval":"15m","strategy":"trend_pullback","account_balance":1000,"risk_percent":1}`)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/futures/analyze", bytes.NewReader(body)))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", response.Code, response.Body.String())
	}
	if service.analyzeReq.Contract != "" {
		t.Fatalf("invalid input reached Gate service: %+v", service.analyzeReq)
	}
}

func TestFuturesAPISanitizesGateFailure(t *testing.T) {
	service := &fakeFuturesService{err: errors.New("upstream body with account detail")}
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, service)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/futures/contracts", nil))

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.Code)
	}
	if strings.Contains(response.Body.String(), "account detail") || response.Body.String() != "{\"error\":\"Gate futures market data temporarily unavailable; retry analysis\"}\n" {
		t.Fatalf("unsanitized body = %q", response.Body.String())
	}
}

func TestFuturesLiveAPIReturnsSelectedMarketSnapshot(t *testing.T) {
	live := &fakeFuturesLiveService{snapshot: futuresdomain.LiveSnapshot{
		CapturedAt: 1_700_000_000_100,
		Candle:     futuresdomain.Candle{Time: 1_700_000_000, Close: 0.0107},
		Route:      futuresdomain.NeutralRoute{Status: futuresdomain.NeutralRouteExecutable},
	}}
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, live)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/futures/live?contract=COTI_USDT&interval=15m&trade_notional=250&min_net_spread_pct=0.2", nil))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"captured_at":1700000000100`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	if live.request.Contract != "COTI_USDT" || live.request.TradeNotional != 250 || live.request.MinNetSpreadPct != 0.2 {
		t.Fatalf("live request = %+v", live.request)
	}
}

func TestFuturesExecuteAPIRequiresExplicitConfirmationAndReturnsSanitizedResult(t *testing.T) {
	executor := &fakeFuturesExecutionService{result: futuresdomain.ExecutionResult{
		Status: futuresdomain.ExecutionOpened,
		Fills:  []futuresdomain.OrderFill{{Venue: "gate_futures", OrderID: "123", FilledContracts: 12}},
	}}
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, executor)
	body := `{"contract":"COTI_USDT","trade_notional":100,"min_net_spread_pct":0.5,"expected_long_venue":"gate_futures","expected_short_venue":"binance_futures","confirm_live":true}`
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/futures/execute", strings.NewReader(body)))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"opened"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	if !executor.request.ConfirmLive || executor.request.ExpectedShortVenue != "binance_futures" {
		t.Fatalf("execution request = %+v", executor.request)
	}

	executor.err = futuresdomain.ErrRouteChanged
	conflict := httptest.NewRecorder()
	handler.ServeHTTP(conflict, httptest.NewRequest(http.MethodPost, "/api/futures/execute", strings.NewReader(body)))
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "analyze the market again") {
		t.Fatalf("conflict = %d %s", conflict.Code, conflict.Body.String())
	}

	executor.result = futuresdomain.ExecutionResult{Status: futuresdomain.ExecutionFailed, ReasonCode: "binance_futures_preflight_failed"}
	executor.err = futuresdomain.ErrPreflightFailed
	preflight := httptest.NewRecorder()
	handler.ServeHTTP(preflight, httptest.NewRequest(http.MethodPost, "/api/futures/execute", strings.NewReader(body)))
	if preflight.Code != http.StatusUnprocessableEntity || !strings.Contains(preflight.Body.String(), "Binance Futures preflight failed") ||
		strings.Contains(preflight.Body.String(), "private account") {
		t.Fatalf("preflight = %d %s", preflight.Code, preflight.Body.String())
	}
}

func TestFuturesAccessAPISeparatesReadOnlyPreflightFromExplicitLiveProbe(t *testing.T) {
	access := &fakeFuturesAccessService{
		report: futuresdomain.AccessReport{
			Contract: "H_USDT", RequiredNotional: 5,
			Venues: []futuresdomain.VenueAccessReport{{Venue: "gate_futures", Ready: true}, {Venue: "binance_futures", ReasonCode: "insufficient_margin"}},
		},
		probeResult: futuresdomain.OrderProbeResult{
			Venue: "gate_futures", Contract: "H_USDT", Status: futuresdomain.OrderProbeCancelled,
			OrderID: "123", CleanupVerified: true,
		},
	}
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, access)

	preflight := httptest.NewRecorder()
	handler.ServeHTTP(preflight, httptest.NewRequest(http.MethodGet, "/api/futures/access?contract=H_USDT&required_notional=5", nil))
	if preflight.Code != http.StatusOK || !strings.Contains(preflight.Body.String(), `"reason_code":"insufficient_margin"`) {
		t.Fatalf("preflight = %d %s", preflight.Code, preflight.Body.String())
	}
	if access.accessReq.Contract != "H_USDT" || access.probeRequest.Contract != "" {
		t.Fatalf("requests after read-only check: access=%+v probe=%+v", access.accessReq, access.probeRequest)
	}

	probe := httptest.NewRecorder()
	handler.ServeHTTP(probe, httptest.NewRequest(http.MethodPost, "/api/futures/order-probe", strings.NewReader(
		`{"venue":"gate_futures","contract":"H_USDT","confirm_live":true}`,
	)))
	if probe.Code != http.StatusOK || !strings.Contains(probe.Body.String(), `"cleanup_verified":true`) {
		t.Fatalf("probe = %d %s", probe.Code, probe.Body.String())
	}
	if !access.probeRequest.ConfirmLive || access.probeRequest.Venue != "gate_futures" {
		t.Fatalf("probe request = %+v", access.probeRequest)
	}
}

func TestFuturesOrderProbeAPIRejectsMissingConfirmationAndSanitizesExposure(t *testing.T) {
	access := &fakeFuturesAccessService{}
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, access)
	missingConfirmation := httptest.NewRecorder()
	handler.ServeHTTP(missingConfirmation, httptest.NewRequest(http.MethodPost, "/api/futures/order-probe", strings.NewReader(
		`{"venue":"gate_futures","contract":"H_USDT","confirm_live":false}`,
	)))
	if missingConfirmation.Code != http.StatusBadRequest || access.probeRequest.Contract != "" {
		t.Fatalf("missing confirmation = %d %s, request=%+v", missingConfirmation.Code, missingConfirmation.Body.String(), access.probeRequest)
	}

	access.probeResult = futuresdomain.OrderProbeResult{Venue: "gate_futures", Contract: "H_USDT", Status: futuresdomain.OrderProbeExposed}
	access.probeErr = futuresdomain.ErrOrderProbeExposed
	exposed := httptest.NewRecorder()
	handler.ServeHTTP(exposed, httptest.NewRequest(http.MethodPost, "/api/futures/order-probe", strings.NewReader(
		`{"venue":"gate_futures","contract":"H_USDT","confirm_live":true}`,
	)))
	if exposed.Code != http.StatusInternalServerError || !strings.Contains(exposed.Body.String(), "inspect the venue immediately") ||
		strings.Contains(exposed.Body.String(), "private") {
		t.Fatalf("exposed = %d %s", exposed.Code, exposed.Body.String())
	}

	access.probeErr = futuresdomain.ErrOrderProbeInProgress
	duplicate := httptest.NewRecorder()
	handler.ServeHTTP(duplicate, httptest.NewRequest(http.MethodPost, "/api/futures/order-probe", strings.NewReader(
		`{"venue":"gate_futures","contract":"H_USDT","confirm_live":true}`,
	)))
	if duplicate.Code != http.StatusConflict || !strings.Contains(duplicate.Body.String(), "already in progress") {
		t.Fatalf("duplicate = %d %s", duplicate.Code, duplicate.Body.String())
	}
}

func TestFuturesAPIPersistsAndClosesPaperPositionsWithoutAnOrderRoute(t *testing.T) {
	store := openAPIPaperStore(t)
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, &fakeFuturesService{}, store)
	createBody := []byte(`{"contract":"COTI_USDT","direction":"long","strategy":"trend_pullback","interval":"15m","entry_price":0.0105,"stop_price":0.01,"target_price":0.0115,"contracts":20000,"base_quantity":20000,"notional":210,"risk_amount":10,"estimated_fees":0.315,"analysis_at_ms":1700000000000}`)
	createdResponse := httptest.NewRecorder()
	handler.ServeHTTP(createdResponse, httptest.NewRequest(http.MethodPost, "/api/futures/paper-positions", bytes.NewReader(createBody)))
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created storage.PaperPosition
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	closeResponse := httptest.NewRecorder()
	handler.ServeHTTP(closeResponse, httptest.NewRequest(http.MethodPut,
		"/api/futures/paper-positions/"+strconv.FormatInt(created.ID, 10)+"/close", strings.NewReader(`{"exit_price":0.0112}`)))
	if closeResponse.Code != http.StatusOK || !strings.Contains(closeResponse.Body.String(), `"status":"closed"`) {
		t.Fatalf("close response = %d %s", closeResponse.Code, closeResponse.Body.String())
	}

	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/futures/paper-positions", nil))
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "COTI_USDT") {
		t.Fatalf("list response = %d %s", listResponse.Code, listResponse.Body.String())
	}

	orderResponse := httptest.NewRecorder()
	handler.ServeHTTP(orderResponse, httptest.NewRequest(http.MethodPost, "/api/futures/orders", strings.NewReader(`{}`)))
	if orderResponse.Code != http.StatusNotFound {
		t.Fatalf("live order route status = %d, want 404", orderResponse.Code)
	}
}
