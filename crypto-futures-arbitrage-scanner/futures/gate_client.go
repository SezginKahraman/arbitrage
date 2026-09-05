package futures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const gateAPIBaseURL = "https://api.gateio.ws"

type GateService struct {
	baseURL       string
	client        *http.Client
	cacheMutex    sync.Mutex
	contractCache map[string]cachedGateContract
	bookCache     map[string]cachedGateBook
}

type cachedGateContract struct {
	value     gateContract
	expiresAt time.Time
}

type cachedGateBook struct {
	value     gateOrderBook
	expiresAt time.Time
}

func NewGateService() *GateService {
	return NewGateServiceWithBaseURL(gateAPIBaseURL, &http.Client{Timeout: 10 * time.Second})
}

func NewGateServiceWithBaseURL(baseURL string, client *http.Client) *GateService {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &GateService{
		baseURL: strings.TrimRight(baseURL, "/"), client: client,
		contractCache: make(map[string]cachedGateContract), bookCache: make(map[string]cachedGateBook),
	}
}

type flexibleFloat float64

func (value *flexibleFloat) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(string(data), `"`)
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return errors.New("invalid numeric field")
	}
	*value = flexibleFloat(parsed)
	return nil
}

type gateContract struct {
	Name             string        `json:"name"`
	Status           string        `json:"status"`
	QuantoMultiplier flexibleFloat `json:"quanto_multiplier"`
	MarkPrice        flexibleFloat `json:"mark_price"`
	IndexPrice       flexibleFloat `json:"index_price"`
	LastPrice        flexibleFloat `json:"last_price"`
	MakerFeeRate     flexibleFloat `json:"maker_fee_rate"`
	TakerFeeRate     flexibleFloat `json:"taker_fee_rate"`
	OrderSizeMin     flexibleFloat `json:"order_size_min"`
	OrderSizeMax     flexibleFloat `json:"order_size_max"`
	OrderPriceRound  flexibleFloat `json:"order_price_round"`
	LeverageMin      flexibleFloat `json:"leverage_min"`
	LeverageMax      flexibleFloat `json:"leverage_max"`
	FundingRate      flexibleFloat `json:"funding_rate"`
	FundingInterval  int64         `json:"funding_interval"`
	FundingNextApply int64         `json:"funding_next_apply"`
}

func (service *GateService) loadContract(ctx context.Context, contractName string) (Contract, gateContract, error) {
	service.cacheMutex.Lock()
	if cached, ok := service.contractCache[contractName]; ok && time.Now().Before(cached.expiresAt) {
		service.cacheMutex.Unlock()
		contract, err := cached.value.normalized()
		return contract, cached.value, err
	}
	service.cacheMutex.Unlock()

	var raw gateContract
	contractPath := "/api/v4/futures/usdt/contracts/" + url.PathEscape(contractName)
	if err := service.getJSON(ctx, contractPath, &raw); err != nil {
		return Contract{}, gateContract{}, err
	}
	contract, err := raw.normalized()
	if err == nil {
		service.cacheMutex.Lock()
		service.contractCache[contractName] = cachedGateContract{value: raw, expiresAt: time.Now().Add(10 * time.Second)}
		service.cacheMutex.Unlock()
	}
	return contract, raw, err
}

func (item gateContract) normalized() (Contract, error) {
	contract := Contract{
		Symbol: item.Name, Status: item.Status, QuantoMultiplier: float64(item.QuantoMultiplier),
		MarkPrice: float64(item.MarkPrice), IndexPrice: float64(item.IndexPrice), LastPrice: float64(item.LastPrice),
		MakerFeeRate: float64(item.MakerFeeRate), TakerFeeRate: float64(item.TakerFeeRate),
		OrderSizeMin: float64(item.OrderSizeMin), OrderSizeMax: float64(item.OrderSizeMax),
		PriceIncrement: float64(item.OrderPriceRound), LeverageMin: float64(item.LeverageMin),
		LeverageMax: float64(item.LeverageMax), FundingRate: float64(item.FundingRate), FundingInterval: item.FundingInterval,
	}
	if !contractPattern.MatchString(contract.Symbol) || !finitePositive(contract.QuantoMultiplier) || !finitePositive(contract.MarkPrice) {
		return Contract{}, errors.New("invalid Gate contract data")
	}
	return contract, nil
}

func (service *GateService) getJSON(ctx context.Context, apiPath string, target any) error {
	for attempt := 0; attempt < 2; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, service.baseURL+apiPath, nil)
		if err != nil {
			return errors.New("build Gate market request")
		}
		request.Header.Set("Accept", "application/json")
		response, requestErr := service.client.Do(request)
		if requestErr != nil {
			if attempt == 0 && ctx.Err() == nil {
				if !waitForGateRetry(ctx) {
					return errors.New("Gate market request failed")
				}
				continue
			}
			return errors.New("Gate market request failed")
		}
		payload, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			if attempt == 0 && transientGateStatus(response.StatusCode) && waitForGateRetry(ctx) {
				continue
			}
			return fmt.Errorf("Gate market request returned status %d", response.StatusCode)
		}
		if readErr != nil || json.Unmarshal(payload, target) != nil {
			if attempt == 0 && waitForGateRetry(ctx) {
				continue
			}
			return errors.New("Gate market response was invalid")
		}
		return nil
	}
	return errors.New("Gate market request failed")
}

func transientGateStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || status >= http.StatusInternalServerError
}

func waitForGateRetry(ctx context.Context) bool {
	timer := time.NewTimer(125 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (service *GateService) Contracts(ctx context.Context) ([]Contract, error) {
	var payload []gateContract
	if err := service.getJSON(ctx, "/api/v4/futures/usdt/contracts", &payload); err != nil {
		return nil, err
	}
	contracts := make([]Contract, 0, len(payload))
	for _, item := range payload {
		if item.Status != "trading" {
			continue
		}
		contract, err := item.normalized()
		if err != nil {
			continue
		}
		contracts = append(contracts, contract)
	}
	return contracts, nil
}

type gateCandle struct {
	Time   int64         `json:"t"`
	Open   flexibleFloat `json:"o"`
	High   flexibleFloat `json:"h"`
	Low    flexibleFloat `json:"l"`
	Close  flexibleFloat `json:"c"`
	Volume flexibleFloat `json:"v"`
}

type gateBookLevel struct {
	Price flexibleFloat `json:"p"`
	Size  flexibleFloat `json:"s"`
}

type gateOrderBook struct {
	Update float64         `json:"update"`
	Bids   []gateBookLevel `json:"bids"`
	Asks   []gateBookLevel `json:"asks"`
}

func (service *GateService) loadOrderBook(ctx context.Context, contractName string, depth int) (gateOrderBook, error) {
	cacheKey := contractName + ":" + strconv.Itoa(depth)
	service.cacheMutex.Lock()
	if cached, ok := service.bookCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		service.cacheMutex.Unlock()
		return cached.value, nil
	}
	service.cacheMutex.Unlock()

	bookQuery := url.Values{"contract": {contractName}, "limit": {strconv.Itoa(depth)}, "with_id": {"true"}}
	var rawBook gateOrderBook
	if err := service.getJSON(ctx, "/api/v4/futures/usdt/order_book?"+bookQuery.Encode(), &rawBook); err != nil {
		return gateOrderBook{}, err
	}
	service.cacheMutex.Lock()
	service.bookCache[cacheKey] = cachedGateBook{value: rawBook, expiresAt: time.Now().Add(time.Second)}
	service.cacheMutex.Unlock()
	return rawBook, nil
}

func (service *GateService) Analyze(ctx context.Context, request AnalyzeRequest) (Analysis, error) {
	if err := validateAnalyzeRequest(request); err != nil {
		return Analysis{}, err
	}
	contract, _, err := service.loadContract(ctx, request.Contract)
	if err != nil {
		return Analysis{}, err
	}

	candleQuery := url.Values{"contract": {request.Contract}, "interval": {request.Interval}, "limit": {"200"}}
	var rawCandles []gateCandle
	if err := service.getJSON(ctx, "/api/v4/futures/usdt/candlesticks?"+candleQuery.Encode(), &rawCandles); err != nil {
		return Analysis{}, err
	}
	candles := make([]Candle, 0, len(rawCandles))
	for _, item := range rawCandles {
		candles = append(candles, Candle{
			Time: item.Time, Open: float64(item.Open), High: float64(item.High), Low: float64(item.Low),
			Close: float64(item.Close), Volume: float64(item.Volume),
		})
	}

	rawBook, err := service.loadOrderBook(ctx, request.Contract, 50)
	if err != nil {
		return Analysis{}, err
	}
	book := OrderBook{UpdatedAt: int64(rawBook.Update * 1_000)}
	for _, item := range rawBook.Bids {
		book.Bids = append(book.Bids, BookLevel{Price: float64(item.Price), Size: float64(item.Size)})
	}
	for _, item := range rawBook.Asks {
		book.Asks = append(book.Asks, BookLevel{Price: float64(item.Price), Size: float64(item.Size)})
	}
	return AnalyzeMarket(request, contract, candles, book, time.Now())
}

func (service *GateService) VenueMarket(ctx context.Context, contractName string, depth int) (VenueMarket, error) {
	if !contractPattern.MatchString(contractName) || depth <= 0 || depth > 100 {
		return VenueMarket{}, errors.New("invalid Gate venue market request")
	}
	contract, raw, err := service.loadContract(ctx, contractName)
	if err != nil {
		return VenueMarket{}, err
	}
	rawBook, err := service.loadOrderBook(ctx, contractName, depth)
	if err != nil {
		return VenueMarket{}, err
	}
	market := VenueMarket{
		Venue: "gate_futures", Symbol: strings.ReplaceAll(contractName, "_", ""),
		IndexPrice: contract.IndexPrice, MarkPrice: contract.MarkPrice,
		ContractMultiplier: contract.QuantoMultiplier, QuantityStep: 1,
		MinQuantity: contract.OrderSizeMin, PriceIncrement: contract.PriceIncrement,
		TakerFeeRate: math.Max(contract.TakerFeeRate, 0),
		FundingRate:  contract.FundingRate, NextFundingAt: raw.FundingNextApply * 1_000,
		Book: OrderBook{UpdatedAt: int64(rawBook.Update * 1_000)},
	}
	for _, item := range rawBook.Bids {
		market.Book.Bids = append(market.Book.Bids, BookLevel{Price: float64(item.Price), Size: float64(item.Size)})
	}
	for _, item := range rawBook.Asks {
		market.Book.Asks = append(market.Book.Asks, BookLevel{Price: float64(item.Price), Size: float64(item.Size)})
	}
	return market, nil
}

func (service *GateService) LatestCandle(ctx context.Context, contractName, interval string) (Candle, error) {
	if !contractPattern.MatchString(contractName) || !validInterval(interval) {
		return Candle{}, errors.New("invalid Gate candle request")
	}
	query := url.Values{"contract": {contractName}, "interval": {interval}, "limit": {"1"}}
	var payload []gateCandle
	if err := service.getJSON(ctx, "/api/v4/futures/usdt/candlesticks?"+query.Encode(), &payload); err != nil {
		return Candle{}, err
	}
	if len(payload) != 1 {
		return Candle{}, errors.New("Gate candle response was empty")
	}
	item := payload[0]
	return Candle{Time: item.Time, Open: float64(item.Open), High: float64(item.High), Low: float64(item.Low), Close: float64(item.Close), Volume: float64(item.Volume)}, nil
}

func validInterval(interval string) bool {
	switch interval {
	case "15m", "1h", "4h":
		return true
	default:
		return false
	}
}
