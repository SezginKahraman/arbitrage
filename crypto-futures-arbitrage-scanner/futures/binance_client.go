package futures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const binanceFuturesBaseURL = "https://fapi.binance.com"

type BinancePublicClient struct {
	baseURL      string
	client       *http.Client
	takerFeeRate float64
}

func NewBinancePublicClient(takerFeeRate float64) *BinancePublicClient {
	return NewBinancePublicClientWithBaseURL(binanceFuturesBaseURL, &http.Client{Timeout: 10 * time.Second}, takerFeeRate)
}

func NewBinancePublicClientWithBaseURL(baseURL string, client *http.Client, takerFeeRate float64) *BinancePublicClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if takerFeeRate <= 0 {
		takerFeeRate = 0.0005
	}
	return &BinancePublicClient{baseURL: strings.TrimRight(baseURL, "/"), client: client, takerFeeRate: takerFeeRate}
}

func (client *BinancePublicClient) getJSON(ctx context.Context, apiPath string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+apiPath, nil)
	if err != nil {
		return errors.New("build Binance market request")
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return errors.New("Binance market request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return fmt.Errorf("Binance market request returned status %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target); err != nil {
		return errors.New("Binance market response was invalid")
	}
	return nil
}

type binanceExchangeInfo struct {
	Symbols []struct {
		Symbol, Status, ContractType, BaseAsset, QuoteAsset string
		Filters                                             []struct {
			FilterType string        `json:"filterType"`
			MinQty     flexibleFloat `json:"minQty"`
			StepSize   flexibleFloat `json:"stepSize"`
			TickSize   flexibleFloat `json:"tickSize"`
			Notional   flexibleFloat `json:"notional"`
		} `json:"filters"`
	} `json:"symbols"`
}

type binancePremium struct {
	Symbol          string        `json:"symbol"`
	MarkPrice       flexibleFloat `json:"markPrice"`
	IndexPrice      flexibleFloat `json:"indexPrice"`
	FundingRate     flexibleFloat `json:"lastFundingRate"`
	NextFundingTime int64         `json:"nextFundingTime"`
	Time            int64         `json:"time"`
}

type binanceDepth struct {
	EventTime       int64      `json:"E"`
	TransactionTime int64      `json:"T"`
	Bids            [][]string `json:"bids"`
	Asks            [][]string `json:"asks"`
}

func (client *BinancePublicClient) VenueMarket(ctx context.Context, gateContract string, depth int) (VenueMarket, error) {
	if !contractPattern.MatchString(gateContract) || depth <= 0 || depth > 100 {
		return VenueMarket{}, errors.New("invalid Binance venue market request")
	}
	symbol := strings.ReplaceAll(gateContract, "_", "")
	var info binanceExchangeInfo
	if err := client.getJSON(ctx, "/fapi/v1/exchangeInfo", &info); err != nil {
		return VenueMarket{}, err
	}
	quantityStep, minQuantity, priceIncrement, minNotional := 0.0, 0.0, 0.0, 0.0
	found := false
	for _, item := range info.Symbols {
		if item.Symbol != symbol || item.Status != "TRADING" || item.ContractType != "PERPETUAL" || item.QuoteAsset != "USDT" {
			continue
		}
		found = true
		for _, filter := range item.Filters {
			switch filter.FilterType {
			case "LOT_SIZE":
				quantityStep, minQuantity = float64(filter.StepSize), float64(filter.MinQty)
			case "PRICE_FILTER":
				priceIncrement = float64(filter.TickSize)
			case "MIN_NOTIONAL":
				minNotional = float64(filter.Notional)
			}
		}
		break
	}
	if !found || !finitePositive(quantityStep) || !finitePositive(minQuantity) || !finitePositive(priceIncrement) {
		return VenueMarket{}, errors.New("Binance perpetual contract unavailable")
	}

	var premium binancePremium
	premiumQuery := url.Values{"symbol": {symbol}}
	if err := client.getJSON(ctx, "/fapi/v1/premiumIndex?"+premiumQuery.Encode(), &premium); err != nil {
		return VenueMarket{}, err
	}
	var depthPayload binanceDepth
	depthQuery := url.Values{"limit": {strconv.Itoa(depth)}, "symbol": {symbol}}
	if err := client.getJSON(ctx, "/fapi/v1/depth?"+depthQuery.Encode(), &depthPayload); err != nil {
		return VenueMarket{}, err
	}
	market := VenueMarket{
		Venue: "binance_futures", Symbol: symbol, IndexPrice: float64(premium.IndexPrice), MarkPrice: float64(premium.MarkPrice),
		ContractMultiplier: 1, QuantityStep: quantityStep, MinQuantity: minQuantity,
		PriceIncrement: priceIncrement, MinNotional: minNotional,
		TakerFeeRate: client.takerFeeRate, FundingRate: float64(premium.FundingRate), NextFundingAt: premium.NextFundingTime,
		Book: OrderBook{UpdatedAt: depthPayload.TransactionTime},
	}
	for _, item := range depthPayload.Bids {
		level, err := binanceBookLevel(item)
		if err == nil {
			market.Book.Bids = append(market.Book.Bids, level)
		}
	}
	for _, item := range depthPayload.Asks {
		level, err := binanceBookLevel(item)
		if err == nil {
			market.Book.Asks = append(market.Book.Asks, level)
		}
	}
	if len(market.Book.Bids) == 0 || len(market.Book.Asks) == 0 {
		return VenueMarket{}, errors.New("Binance order book was empty")
	}
	return market, nil
}

func binanceBookLevel(item []string) (BookLevel, error) {
	if len(item) != 2 {
		return BookLevel{}, errors.New("invalid Binance book level")
	}
	price, priceErr := strconv.ParseFloat(item[0], 64)
	size, sizeErr := strconv.ParseFloat(item[1], 64)
	if priceErr != nil || sizeErr != nil || !finitePositive(price) || !finitePositive(size) {
		return BookLevel{}, errors.New("invalid Binance book level")
	}
	return BookLevel{Price: price, Size: size}, nil
}
