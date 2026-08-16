package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const marketDiscoveryResponseLimit = 32 << 20

type marketPayloadParser func([]byte) ([]string, error)

func newProductionMarketCatalog() *marketCatalog {
	client := &http.Client{Timeout: 15 * time.Second}
	definitions := []marketSourceDefinition{
		{Source: sourceBinanceSpot, Market: marketSpot, Fetch: marketGETFetcher(client, "https://api.binance.com/api/v3/exchangeInfo", parseBinanceSpotMarkets)},
		{Source: sourceBinanceFutures, Market: marketFutures, Fetch: marketGETFetcher(client, "https://fapi.binance.com/fapi/v1/exchangeInfo", parseBinanceFuturesMarkets)},
		{Source: sourceBybitSpot, Market: marketSpot, Fetch: bybitMarketFetcher(client, "spot")},
		{Source: sourceBybitFutures, Market: marketFutures, Fetch: bybitMarketFetcher(client, "linear")},
		{Source: sourceGateSpot, Market: marketSpot, Fetch: marketGETFetcher(client, "https://api.gateio.ws/api/v4/spot/currency_pairs", parseGateSpotMarkets)},
		{Source: sourceGateFutures, Market: marketFutures, Fetch: marketGETFetcher(client, "https://api.gateio.ws/api/v4/futures/usdt/contracts", parseGateFuturesMarkets)},
		{Source: sourceKuCoinSpot, Market: marketSpot, Fetch: marketGETFetcher(client, "https://api.kucoin.com/api/v2/symbols", parseKuCoinSpotMarkets)},
		{Source: sourceKuCoinFutures, Market: marketFutures, Fetch: marketGETFetcher(client, "https://api-futures.kucoin.com/api/v1/contracts/active", parseKuCoinFuturesMarkets)},
		{Source: sourceOKXFutures, Market: marketFutures, Fetch: marketGETFetcher(client, "https://www.okx.com/api/v5/public/instruments?instType=SWAP", parseOKXFuturesMarkets)},
		{Source: sourceKrakenFutures, Market: marketFutures, Fetch: marketGETFetcher(client, "https://futures.kraken.com/derivatives/api/v3/instruments", parseKrakenFuturesMarkets)},
		{Source: sourceHyperliquidFutures, Market: marketFutures, Fetch: hyperliquidMarketFetcher(client)},
		{Source: sourceParadexFutures, Market: marketFutures, Fetch: marketGETFetcher(client, "https://api.prod.paradex.trade/v1/markets", parseParadexMarkets)},
	}
	return newMarketCatalog(definitions)
}

func marketGETFetcher(client *http.Client, endpoint string, parse marketPayloadParser) marketSourceFetcher {
	return func(ctx context.Context) ([]string, error) {
		payload, err := fetchMarketPayload(ctx, client, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		return parse(payload)
	}
}

func fetchMarketPayload(ctx context.Context, client *http.Client, method, endpoint string, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build market request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("market request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("market request returned HTTP %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, marketDiscoveryResponseLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read market response: %w", err)
	}
	if len(payload) > marketDiscoveryResponseLimit {
		return nil, fmt.Errorf("market response exceeds %d bytes", marketDiscoveryResponseLimit)
	}
	return payload, nil
}

func bybitMarketFetcher(client *http.Client, category string) marketSourceFetcher {
	return func(ctx context.Context) ([]string, error) {
		cursor := ""
		result := make([]string, 0, 1000)
		for page := 0; page < 10; page++ {
			values := url.Values{"category": {category}, "limit": {"1000"}}
			if cursor != "" {
				values.Set("cursor", cursor)
			}
			payload, err := fetchMarketPayload(ctx, client, http.MethodGet, "https://api.bybit.com/v5/market/instruments-info?"+values.Encode(), nil)
			if err != nil {
				return nil, err
			}
			symbols, nextCursor, err := parseBybitMarketPage(payload)
			if err != nil {
				return nil, err
			}
			result = append(result, symbols...)
			if nextCursor == "" || nextCursor == cursor {
				return result, nil
			}
			cursor = nextCursor
		}
		return result, nil
	}
}

func hyperliquidMarketFetcher(client *http.Client) marketSourceFetcher {
	return func(ctx context.Context) ([]string, error) {
		payload, err := fetchMarketPayload(ctx, client, http.MethodPost, "https://api.hyperliquid.xyz/info", []byte(`{"type":"meta"}`))
		if err != nil {
			return nil, err
		}
		return parseHyperliquidMarkets(payload)
	}
}

func parseBinanceSpotMarkets(payload []byte) ([]string, error) {
	var response struct {
		Symbols []struct {
			Symbol string `json:"symbol"`
			Status string `json:"status"`
			Quote  string `json:"quoteAsset"`
			Spot   bool   `json:"isSpotTradingAllowed"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(response.Symbols))
	for _, item := range response.Symbols {
		if item.Status == "TRADING" && item.Quote == "USDT" && item.Spot {
			result = append(result, item.Symbol)
		}
	}
	return normalizeDiscoveredSymbols(result), nil
}

func parseBinanceFuturesMarkets(payload []byte) ([]string, error) {
	var response struct {
		Symbols []struct {
			Symbol       string `json:"symbol"`
			Status       string `json:"status"`
			Quote        string `json:"quoteAsset"`
			ContractType string `json:"contractType"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(response.Symbols))
	for _, item := range response.Symbols {
		if item.Status == "TRADING" && item.Quote == "USDT" && item.ContractType == "PERPETUAL" {
			result = append(result, item.Symbol)
		}
	}
	return normalizeDiscoveredSymbols(result), nil
}

func parseBybitMarketPage(payload []byte) ([]string, string, error) {
	var response struct {
		Code   int `json:"retCode"`
		Result struct {
			List []struct {
				Symbol string `json:"symbol"`
				Quote  string `json:"quoteCoin"`
				Status string `json:"status"`
			} `json:"list"`
			NextCursor string `json:"nextPageCursor"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, "", err
	}
	if response.Code != 0 {
		return nil, "", fmt.Errorf("Bybit market response code %d", response.Code)
	}
	result := make([]string, 0, len(response.Result.List))
	for _, item := range response.Result.List {
		if item.Quote == "USDT" && item.Status == "Trading" {
			result = append(result, item.Symbol)
		}
	}
	return normalizeDiscoveredSymbols(result), response.Result.NextCursor, nil
}

func parseBybitMarkets(payload []byte) ([]string, error) {
	symbols, _, err := parseBybitMarketPage(payload)
	return symbols, err
}

func parseGateSpotMarkets(payload []byte) ([]string, error) {
	var response []struct {
		ID     string `json:"id"`
		Quote  string `json:"quote"`
		Status string `json:"trade_status"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(response))
	for _, item := range response {
		if item.Quote == "USDT" && item.Status == "tradable" {
			result = append(result, item.ID)
		}
	}
	return normalizeDiscoveredSymbols(result), nil
}

func parseGateFuturesMarkets(payload []byte) ([]string, error) {
	var response []struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		Delisting bool   `json:"in_delisting"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(response))
	for _, item := range response {
		if item.Type == "direct" && !item.Delisting && strings.HasSuffix(item.Name, "_USDT") {
			result = append(result, item.Name)
		}
	}
	return normalizeDiscoveredSymbols(result), nil
}

func parseKuCoinSpotMarkets(payload []byte) ([]string, error) {
	var response struct {
		Code string `json:"code"`
		Data []struct {
			Symbol  string `json:"symbol"`
			Quote   string `json:"quoteCurrency"`
			Enabled bool   `json:"enableTrading"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	if response.Code != "200000" {
		return nil, fmt.Errorf("KuCoin market response code %q", response.Code)
	}
	result := make([]string, 0, len(response.Data))
	for _, item := range response.Data {
		if item.Quote == "USDT" && item.Enabled {
			result = append(result, item.Symbol)
		}
	}
	return normalizeDiscoveredSymbols(result), nil
}

func parseKuCoinFuturesMarkets(payload []byte) ([]string, error) {
	var response struct {
		Code string `json:"code"`
		Data []struct {
			Symbol  string `json:"symbol"`
			Quote   string `json:"quoteCurrency"`
			Status  string `json:"status"`
			Inverse bool   `json:"isInverse"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	if response.Code != "200000" {
		return nil, fmt.Errorf("KuCoin market response code %q", response.Code)
	}
	result := make([]string, 0, len(response.Data))
	for _, item := range response.Data {
		if item.Quote != "USDT" || item.Status != "Open" || item.Inverse {
			continue
		}
		symbol := strings.TrimSuffix(item.Symbol, "M")
		if strings.HasPrefix(symbol, "XBT") {
			symbol = "BTC" + strings.TrimPrefix(symbol, "XBT")
		}
		result = append(result, symbol)
	}
	return normalizeDiscoveredSymbols(result), nil
}

func parseOKXFuturesMarkets(payload []byte) ([]string, error) {
	var response struct {
		Code string `json:"code"`
		Data []struct {
			Instrument   string `json:"instId"`
			Settle       string `json:"settleCcy"`
			State        string `json:"state"`
			ContractType string `json:"ctType"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	if response.Code != "0" {
		return nil, fmt.Errorf("OKX market response code %q", response.Code)
	}
	result := make([]string, 0, len(response.Data))
	for _, item := range response.Data {
		if item.Settle == "USDT" && item.State == "live" && item.ContractType == "linear" {
			result = append(result, strings.TrimSuffix(item.Instrument, "-SWAP"))
		}
	}
	return normalizeDiscoveredSymbols(result), nil
}

func parseKrakenFuturesMarkets(payload []byte) ([]string, error) {
	var response struct {
		Result      string `json:"result"`
		Instruments []struct {
			Symbol    string `json:"symbol"`
			Tradeable bool   `json:"tradeable"`
		} `json:"instruments"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	if response.Result != "success" {
		return nil, fmt.Errorf("Kraken market response %q", response.Result)
	}
	result := make([]string, 0, len(response.Instruments))
	for _, item := range response.Instruments {
		if !item.Tradeable || !strings.HasPrefix(item.Symbol, "PF_") || !strings.HasSuffix(item.Symbol, "USD") {
			continue
		}
		base := strings.TrimSuffix(strings.TrimPrefix(item.Symbol, "PF_"), "USD")
		if base == "XBT" {
			base = "BTC"
		}
		result = append(result, base+"USDT")
	}
	return normalizeDiscoveredSymbols(result), nil
}

func parseHyperliquidMarkets(payload []byte) ([]string, error) {
	var response struct {
		Universe []struct {
			Name     string `json:"name"`
			Delisted bool   `json:"isDelisted"`
		} `json:"universe"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(response.Universe))
	for _, item := range response.Universe {
		if !item.Delisted {
			result = append(result, item.Name+"USDT")
		}
	}
	return normalizeDiscoveredSymbols(result), nil
}

func parseParadexMarkets(payload []byte) ([]string, error) {
	var response struct {
		Results []struct {
			Symbol string `json:"symbol"`
			Base   string `json:"base_currency"`
			Kind   string `json:"asset_kind"`
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(response.Results))
	for _, item := range response.Results {
		if item.Kind == "PERP" && (item.Status == "" || item.Status == "ACTIVE") && strings.HasSuffix(item.Symbol, "-USD-PERP") {
			result = append(result, item.Base+"USDT")
		}
	}
	return normalizeDiscoveredSymbols(result), nil
}
