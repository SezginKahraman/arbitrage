package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"futures-arbitrage-scanner/exchanges"
)

func TestAPIListsMarketsAndReadsAndReplacesWatchlist(t *testing.T) {
	repository := &fakeWatchlistRepository{symbols: []string{"BTCUSDT"}}
	markets := &fakeMarketReader{
		candidates: []marketCandidate{{Symbol: "BTCUSDT", Base: "BTC", SpotSources: []string{sourceBinanceSpot}, FuturesSources: []string{sourceGateFutures}, Sources: []string{sourceBinanceSpot, sourceGateFutures}}},
		states:     []marketSourceState{{Source: sourceBinanceSpot, Market: marketSpot, Status: marketSourceReady, Symbols: []string{"BTCUSDT"}}},
	}
	watchlist := newWatchlistService(repository, markets, nil)
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, watchlist)

	marketsResponse := httptest.NewRecorder()
	handler.ServeHTTP(marketsResponse, httptest.NewRequest(http.MethodGet, "/api/markets", nil))
	if marketsResponse.Code != http.StatusOK {
		t.Fatalf("markets status = %d: %s", marketsResponse.Code, marketsResponse.Body.String())
	}
	var marketPayload struct {
		Items        []marketCandidate   `json:"items"`
		Sources      []marketSourceState `json:"sources"`
		MaxWatchlist int                 `json:"maxWatchlist"`
	}
	if err := json.NewDecoder(marketsResponse.Body).Decode(&marketPayload); err != nil {
		t.Fatal(err)
	}
	if len(marketPayload.Items) != 1 || marketPayload.MaxWatchlist != maxWatchlistSymbols {
		t.Fatalf("markets payload = %+v", marketPayload)
	}

	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/api/watchlist", nil))
	var getPayload struct {
		Symbols []string `json:"symbols"`
	}
	if err := json.NewDecoder(getResponse.Body).Decode(&getPayload); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(getPayload.Symbols, []string{"BTCUSDT"}) {
		t.Fatalf("GET watchlist = %v", getPayload.Symbols)
	}

	putResponse := httptest.NewRecorder()
	handler.ServeHTTP(putResponse, httptest.NewRequest(http.MethodPut, "/api/watchlist", bytes.NewBufferString(`{"symbols":["BTCUSDT"]}`)))
	if putResponse.Code != http.StatusOK || len(repository.writes) != 1 {
		t.Fatalf("PUT status=%d writes=%v body=%s", putResponse.Code, repository.writes, putResponse.Body.String())
	}
}

func TestAPIBatchTransferRoutesUsesActiveWatchlistAndDirectionalNetworks(t *testing.T) {
	repository := &fakeWatchlistRepository{symbols: []string{"COTIUSDT"}}
	watchlist := newWatchlistService(repository, &fakeMarketReader{candidates: []marketCandidate{{Symbol: "COTIUSDT"}}}, nil)
	networks := newNetworkCatalog([]string{"COTI"}, nil)
	checkedAt := time.UnixMilli(50_000)
	networks.snapshots["COTI"][sourceGateSpot] = readyNetworkSnapshot(sourceGateSpot, "COTI", []exchanges.AssetNetwork{{
		Asset: "COTI", NetworkID: "coti_evm", RawNetworkID: "COTI", WithdrawEnabled: true, CheckedAt: checkedAt.UnixMilli(),
	}}, checkedAt)
	networks.snapshots["COTI"][sourceKuCoinSpot] = readyNetworkSnapshot(sourceKuCoinSpot, "COTI", []exchanges.AssetNetwork{{
		Asset: "COTI", NetworkID: "coti_evm", RawNetworkID: "cotievm", DepositEnabled: true, CheckedAt: checkedAt.UnixMilli(),
	}}, checkedAt)
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, networks, watchlist)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/transfer-routes", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []transferRouteEvaluation `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range payload.Items {
		if item.Asset == "COTI" && item.Source == sourceGateSpot && item.Destination == sourceKuCoinSpot {
			found = item.Status == transferRouteCheck
		}
	}
	if !found {
		t.Fatalf("expected Gate to KuCoin CHECK route in %+v", payload.Items)
	}
}

func TestAPIBatchTransferRoutesAlwaysSerializesCollectionsAsArrays(t *testing.T) {
	repository := &fakeWatchlistRepository{symbols: []string{"A47USDT"}}
	watchlist := newWatchlistService(repository, &fakeMarketReader{candidates: []marketCandidate{{Symbol: "A47USDT"}}}, nil)
	networks := newNetworkCatalog([]string{"A47"}, nil)
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, networks, watchlist)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/transfer-routes", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var raw struct {
		Items []struct {
			Networks            json.RawMessage `json:"networks"`
			SourceNetworks      json.RawMessage `json:"source_networks"`
			DestinationNetworks json.RawMessage `json:"destination_networks"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	for _, item := range raw.Items {
		if string(item.Networks) != "[]" || string(item.SourceNetworks) != "[]" || string(item.DestinationNetworks) != "[]" {
			t.Fatalf("route collections must be arrays: %+v", item)
		}
	}
}

func TestAPIRejectsUnsupportedWatchlistWithoutChangingCurrentSelection(t *testing.T) {
	repository := &fakeWatchlistRepository{symbols: []string{"BTCUSDT"}}
	watchlist := newWatchlistService(repository, &fakeMarketReader{candidates: []marketCandidate{{Symbol: "BTCUSDT"}}}, nil)
	handler := newAPIHandler(&fakeOpportunityStore{}, &fakeAlertStore{}, func() bool { return true }, watchlist)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/watchlist", bytes.NewBufferString(`{"symbols":["NOPEUSDT"]}`)))
	if response.Code != http.StatusBadRequest || len(repository.writes) != 0 {
		t.Fatalf("status=%d writes=%v body=%s", response.Code, repository.writes, response.Body.String())
	}
}
