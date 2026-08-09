package exchanges

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	spotRESTRefreshInterval = 5 * time.Second
	spotRESTRequestTimeout  = 4 * time.Second
	spotRESTResponseLimit   = 8 << 20
)

type spotRESTParser func([]byte, map[string]struct{}, time.Time) ([]OrderbookData, error)

type spotRESTSource struct {
	name     string
	endpoint string
	parse    spotRESTParser
}

var spotRESTSources = []spotRESTSource{
	{name: "binance_spot", endpoint: "https://api.binance.com/api/v3/ticker/bookTicker", parse: parseBinanceSpotRESTBooks},
	{name: "bybit_spot", endpoint: "https://api.bybit.com/v5/market/tickers?category=spot", parse: parseBybitSpotRESTBooks},
	{name: "gate_spot", endpoint: "https://api.gateio.ws/api/v4/spot/tickers", parse: parseGateSpotRESTBooks},
	{name: "kucoin_spot", endpoint: "https://api.kucoin.com/api/v1/market/allTickers", parse: parseKuCoinSpotRESTBooks},
}

// StartSpotRESTFallbacks periodically validates quiet spot books. Some book-ticker
// WebSockets publish only when a quote changes, so silence alone must not make a
// still-valid best bid/ask look stale.
func StartSpotRESTFallbacks(ctx context.Context, symbolsBySource map[string][]string, orderbookChan chan<- OrderbookData) {
	client := &http.Client{Timeout: spotRESTRequestTimeout}
	for _, source := range spotRESTSources {
		symbols := append([]string(nil), symbolsBySource[source.name]...)
		if len(symbols) == 0 {
			continue
		}
		go refreshSpotRESTSource(ctx, client, source, symbols, orderbookChan)
	}
}

func refreshSpotRESTSource(ctx context.Context, client *http.Client, source spotRESTSource, symbols []string, orderbookChan chan<- OrderbookData) {
	refresh := func() bool {
		books, err := fetchSpotRESTBooks(ctx, client, source.endpoint, symbols, source.parse, time.Now())
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("%s REST book refresh failed: %v", source.name, err)
			}
			return true
		}
		for _, book := range books {
			select {
			case orderbookChan <- book:
			case <-ctx.Done():
				return false
			}
		}
		return true
	}

	if !refresh() {
		return
	}
	ticker := time.NewTicker(spotRESTRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !refresh() {
				return
			}
		}
	}
}

func fetchSpotRESTBooks(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	symbols []string,
	parse spotRESTParser,
	receivedAt time.Time,
) ([]OrderbookData, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("unexpected HTTP status %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, spotRESTResponseLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(body) > spotRESTResponseLimit {
		return nil, fmt.Errorf("response exceeds %d bytes", spotRESTResponseLimit)
	}
	allowed := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		allowed[normalizeSpotSymbol(symbol)] = struct{}{}
	}
	return parse(body, allowed, receivedAt)
}

func parseBinanceSpotRESTBooks(payload []byte, allowed map[string]struct{}, receivedAt time.Time) ([]OrderbookData, error) {
	var response []struct {
		Symbol string `json:"symbol"`
		Bid    string `json:"bidPrice"`
		Ask    string `json:"askPrice"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode Binance books: %w", err)
	}
	books := make([]OrderbookData, 0, len(allowed))
	for _, row := range response {
		if book, ok := normalizeSpotRESTBook(row.Symbol, "binance_spot", row.Bid, row.Ask, allowed, receivedAt); ok {
			books = append(books, book)
		}
	}
	return books, nil
}

func parseGateSpotRESTBooks(payload []byte, allowed map[string]struct{}, receivedAt time.Time) ([]OrderbookData, error) {
	var response []struct {
		Symbol string `json:"currency_pair"`
		Bid    string `json:"highest_bid"`
		Ask    string `json:"lowest_ask"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode Gate.io books: %w", err)
	}
	books := make([]OrderbookData, 0, len(allowed))
	for _, row := range response {
		if book, ok := normalizeSpotRESTBook(row.Symbol, "gate_spot", row.Bid, row.Ask, allowed, receivedAt); ok {
			books = append(books, book)
		}
	}
	return books, nil
}

func parseKuCoinSpotRESTBooks(payload []byte, allowed map[string]struct{}, receivedAt time.Time) ([]OrderbookData, error) {
	var response struct {
		Code string `json:"code"`
		Data struct {
			Tickers []struct {
				Symbol string `json:"symbol"`
				Bid    string `json:"buy"`
				Ask    string `json:"sell"`
			} `json:"ticker"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode KuCoin books: %w", err)
	}
	if response.Code != "200000" {
		return nil, fmt.Errorf("KuCoin response code %q", response.Code)
	}
	books := make([]OrderbookData, 0, len(allowed))
	for _, row := range response.Data.Tickers {
		if book, ok := normalizeSpotRESTBook(row.Symbol, "kucoin_spot", row.Bid, row.Ask, allowed, receivedAt); ok {
			books = append(books, book)
		}
	}
	return books, nil
}

func parseBybitSpotRESTBooks(payload []byte, allowed map[string]struct{}, receivedAt time.Time) ([]OrderbookData, error) {
	var response struct {
		Code   int `json:"retCode"`
		Result struct {
			Rows []struct {
				Symbol string `json:"symbol"`
				Bid    string `json:"bid1Price"`
				Ask    string `json:"ask1Price"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode Bybit books: %w", err)
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("Bybit response code %d", response.Code)
	}
	books := make([]OrderbookData, 0, len(allowed))
	for _, row := range response.Result.Rows {
		if book, ok := normalizeSpotRESTBook(row.Symbol, "bybit_spot", row.Bid, row.Ask, allowed, receivedAt); ok {
			books = append(books, book)
		}
	}
	return books, nil
}

func normalizeSpotRESTBook(symbol, source, bidText, askText string, allowed map[string]struct{}, receivedAt time.Time) (OrderbookData, bool) {
	symbol = normalizeSpotSymbol(symbol)
	if _, ok := allowed[symbol]; !ok {
		return OrderbookData{}, false
	}
	bestBid, bidErr := strconv.ParseFloat(bidText, 64)
	bestAsk, askErr := strconv.ParseFloat(askText, 64)
	if bidErr != nil || askErr != nil || !validSpotPrice(bestBid) || !validSpotPrice(bestAsk) || bestBid > bestAsk {
		return OrderbookData{}, false
	}
	return OrderbookData{
		Symbol: symbol, Source: source, BestBid: bestBid, BestAsk: bestAsk, Timestamp: receivedAt.UnixMilli(),
	}, true
}

func normalizeSpotSymbol(symbol string) string {
	return strings.ToUpper(strings.NewReplacer("_", "", "-", "", "/", "").Replace(symbol))
}

func validSpotPrice(price float64) bool {
	return price > 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}
