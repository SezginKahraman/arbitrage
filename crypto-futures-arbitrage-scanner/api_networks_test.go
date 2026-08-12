package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"futures-arbitrage-scanner/exchanges"
)

type fakeNetworkCatalog struct {
	snapshots map[string]networkVenueSnapshot
}

func (f fakeNetworkCatalog) Snapshots(asset string) map[string]networkVenueSnapshot {
	if asset != "COTI" {
		return nil
	}
	return f.snapshots
}

func networkTestHandler(catalog networkCatalogReader) http.Handler {
	return newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, catalog)
}

func TestAPINetworksListsSanitizedVenueSnapshots(t *testing.T) {
	catalog := fakeNetworkCatalog{snapshots: map[string]networkVenueSnapshot{
		"gate_spot":    {Source: "gate_spot", Asset: "COTI", Status: networkVenueReady, CheckedAt: 20_000, Networks: []exchanges.AssetNetwork{{Asset: "COTI", NetworkID: "coti_evm"}}},
		"binance_spot": {Source: "binance_spot", Asset: "COTI", Status: networkVenueUnavailable, ErrorCode: "credentials_rejected", CheckedAt: 19_000},
	}}
	request := httptest.NewRequest(http.MethodGet, "/api/networks?asset=COTI", nil)
	response := httptest.NewRecorder()

	networkTestHandler(catalog).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Asset  string                 `json:"asset"`
		Venues []networkVenueSnapshot `json:"venues"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Asset != "COTI" || len(payload.Venues) != 2 || payload.Venues[0].Source != "binance_spot" || payload.Venues[1].Source != "gate_spot" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestAPITransferRouteEvaluatesRequestedDirection(t *testing.T) {
	catalog := fakeNetworkCatalog{snapshots: map[string]networkVenueSnapshot{
		"gate_spot": {Source: "gate_spot", Asset: "COTI", Status: networkVenueReady, Networks: []exchanges.AssetNetwork{{
			Asset: "COTI", NetworkID: "ethereum", RawNetworkID: "ETH", ContractAddress: "0xabc", WithdrawEnabled: false,
		}}},
		"kucoin_spot": {Source: "kucoin_spot", Asset: "COTI", Status: networkVenueReady, Networks: []exchanges.AssetNetwork{{
			Asset: "COTI", NetworkID: "ethereum", RawNetworkID: "eth", ContractAddress: "0xabc", DepositEnabled: true,
		}}},
	}}
	request := httptest.NewRequest(http.MethodGet, "/api/transfer-route?asset=COTI&source=gate_spot&destination=kucoin_spot", nil)
	response := httptest.NewRecorder()

	networkTestHandler(catalog).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var route transferRouteEvaluation
	if err := json.NewDecoder(response.Body).Decode(&route); err != nil {
		t.Fatal(err)
	}
	if route.Status != transferRouteBlocked || len(route.Networks) != 1 || route.Networks[0].Reason != "source withdrawal disabled" {
		t.Fatalf("route = %+v", route)
	}
}

func TestAPINetworkRoutesRejectUnsupportedInputs(t *testing.T) {
	for _, target := range []string{
		"/api/networks?asset=NOTREAL",
		"/api/transfer-route?asset=COTI&source=not_real&destination=gate_spot",
		"/api/transfer-route?asset=COTI&source=gate_spot&destination=not_real",
	} {
		response := httptest.NewRecorder()
		networkTestHandler(fakeNetworkCatalog{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d", target, response.Code)
		}
	}
}
