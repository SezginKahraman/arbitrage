package futures

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

type BinanceTrader struct {
	baseURL   string
	client    *http.Client
	apiKey    string
	secret    string
	clock     func() time.Time
	modeMutex sync.RWMutex
	dualMode  map[string]bool
}

func NewBinanceTrader(apiKey, secret string) *BinanceTrader {
	return NewBinanceTraderWithBaseURL(binanceFuturesBaseURL, &http.Client{Timeout: 10 * time.Second}, apiKey, secret, time.Now)
}

func NewBinanceTraderWithBaseURL(baseURL string, client *http.Client, apiKey, secret string, clock func() time.Time) *BinanceTrader {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if clock == nil {
		clock = time.Now
	}
	return &BinanceTrader{baseURL: strings.TrimRight(baseURL, "/"), client: client, apiKey: apiKey, secret: secret, clock: clock, dualMode: make(map[string]bool)}
}

func (trader *BinanceTrader) Venue() string { return "binance_futures" }

func (trader *BinanceTrader) privateJSON(ctx context.Context, method, apiPath string, values url.Values, target any) error {
	if trader.apiKey == "" || trader.secret == "" {
		return ErrLiveTradingDisabled
	}
	if values == nil {
		values = make(url.Values)
	}
	values.Set("recvWindow", "5000")
	values.Set("timestamp", strconv.FormatInt(trader.clock().UnixMilli(), 10))
	mac := hmac.New(sha256.New, []byte(trader.secret))
	_, _ = mac.Write([]byte(values.Encode()))
	values.Set("signature", hex.EncodeToString(mac.Sum(nil)))
	var request *http.Request
	var err error
	if method == http.MethodGet || method == http.MethodDelete {
		request, err = http.NewRequestWithContext(ctx, method, trader.baseURL+apiPath+"?"+values.Encode(), nil)
	} else {
		request, err = http.NewRequestWithContext(ctx, method, trader.baseURL+apiPath, strings.NewReader(values.Encode()))
		if request != nil {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return errors.New("build Binance futures request")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-MBX-APIKEY", trader.apiKey)
	response, err := trader.client.Do(request)
	if err != nil {
		return errors.New("Binance futures request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return fmt.Errorf("Binance futures request returned status %d", response.StatusCode)
	}
	if target == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return errors.New("Binance futures response was invalid")
	}
	return nil
}

func (trader *BinanceTrader) Preflight(ctx context.Context, contract string, requiredNotional float64) error {
	if !contractPattern.MatchString(contract) || !finitePositive(requiredNotional) {
		return errors.New("invalid Binance futures preflight")
	}
	symbol := strings.ReplaceAll(contract, "_", "")
	var config struct {
		CanTrade bool `json:"canTrade"`
		Dual     bool `json:"dualSidePosition"`
	}
	if err := trader.privateJSON(ctx, http.MethodGet, "/fapi/v1/accountConfig", nil, &config); err != nil {
		return err
	}
	if !config.CanTrade {
		return ErrTradeNotAllowed
	}
	var positions []struct {
		PositionAmount flexibleFloat `json:"positionAmt"`
	}
	if err := trader.privateJSON(ctx, http.MethodGet, "/fapi/v2/positionRisk", url.Values{"symbol": {symbol}}, &positions); err != nil {
		return err
	}
	for _, position := range positions {
		if math.Abs(float64(position.PositionAmount)) > 1e-12 {
			return ErrExistingPosition
		}
	}
	var balances []struct {
		Asset     string        `json:"asset"`
		Available flexibleFloat `json:"availableBalance"`
	}
	if err := trader.privateJSON(ctx, http.MethodGet, "/fapi/v3/balance", nil, &balances); err != nil {
		return err
	}
	available := 0.0
	for _, balance := range balances {
		if balance.Asset == "USDT" {
			available = float64(balance.Available)
		}
	}
	if available+1e-9 < requiredNotional {
		return ErrInsufficientMargin
	}
	trader.modeMutex.Lock()
	trader.dualMode[contract] = config.Dual
	trader.modeMutex.Unlock()
	return nil
}

type binanceOrderResponse struct {
	OrderID          int64         `json:"orderId"`
	ExecutedQuantity flexibleFloat `json:"executedQty"`
	AveragePrice     flexibleFloat `json:"avgPrice"`
	Status           string        `json:"status"`
}

func (trader *BinanceTrader) order(ctx context.Context, intent OrderIntent) (OrderFill, error) {
	symbol := strings.ReplaceAll(intent.Contract, "_", "")
	side := strings.ToUpper(string(intent.Side))
	values := url.Values{
		"symbol": {symbol}, "side": {side}, "type": {"MARKET"},
		"quantity": {strconv.FormatFloat(intent.Contracts, 'f', -1, 64)}, "newOrderRespType": {"RESULT"},
	}
	trader.modeMutex.RLock()
	dual := trader.dualMode[intent.Contract]
	trader.modeMutex.RUnlock()
	if dual {
		if intent.Side == OrderBuy {
			values.Set("positionSide", "LONG")
		} else {
			values.Set("positionSide", "SHORT")
		}
		if intent.ReduceOnly {
			if intent.Side == OrderSell {
				values.Set("positionSide", "LONG")
			} else {
				values.Set("positionSide", "SHORT")
			}
		}
	} else if intent.ReduceOnly {
		values.Set("reduceOnly", "true")
	}
	var response binanceOrderResponse
	if err := trader.privateJSON(ctx, http.MethodPost, "/fapi/v1/order", values, &response); err != nil {
		return OrderFill{}, err
	}
	filled := float64(response.ExecutedQuantity)
	if !finitePositive(filled) {
		return OrderFill{}, errors.New("Binance futures market order was not filled")
	}
	return OrderFill{Venue: trader.Venue(), OrderID: strconv.FormatInt(response.OrderID, 10), FilledContracts: filled, AveragePrice: float64(response.AveragePrice)}, nil
}

func (trader *BinanceTrader) PlaceMarket(ctx context.Context, intent OrderIntent) (OrderFill, error) {
	return trader.order(ctx, intent)
}

func (trader *BinanceTrader) CloseFilled(ctx context.Context, intent OrderIntent, fill OrderFill) (OrderFill, error) {
	intent.Contracts = fill.FilledContracts
	intent.ReduceOnly = true
	return trader.order(ctx, intent)
}

func (trader *BinanceTrader) ProbePostOnly(ctx context.Context, plan OrderProbePlan) (result OrderProbeResult, err error) {
	result = OrderProbeResult{
		Venue: plan.Venue, Contract: plan.Contract, Status: OrderProbeFailed,
		Side: plan.Side, Price: plan.Price, Contracts: plan.Contracts, Notional: plan.Notional,
	}
	if plan.Venue != trader.Venue() || !contractPattern.MatchString(plan.Contract) || plan.Side != OrderBuy ||
		!finitePositive(plan.Price) || !finitePositive(plan.Contracts) || !finitePositive(plan.Notional) || plan.Notional > maxOrderProbeNotional {
		return result, ErrOrderProbeFailed
	}
	symbol := strings.ReplaceAll(plan.Contract, "_", "")
	values := url.Values{
		"symbol": {symbol}, "side": {"BUY"}, "type": {"LIMIT"}, "timeInForce": {"GTX"},
		"quantity": {strconv.FormatFloat(plan.Contracts, 'f', -1, 64)},
		"price":    {strconv.FormatFloat(plan.Price, 'f', -1, 64)}, "newOrderRespType": {"RESULT"},
	}
	trader.modeMutex.RLock()
	dual := trader.dualMode[plan.Contract]
	trader.modeMutex.RUnlock()
	if dual {
		values.Set("positionSide", "LONG")
	}
	var created binanceOrderResponse
	if err := trader.privateJSON(ctx, http.MethodPost, "/fapi/v1/order", values, &created); err != nil {
		return result, ErrOrderProbeFailed
	}
	if created.OrderID <= 0 {
		return result, ErrOrderProbeFailed
	}
	result.OrderID = strconv.FormatInt(created.OrderID, 10)
	cancelled := false
	defer func() {
		if cancelled {
			return
		}
		cleanupContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = trader.privateJSON(cleanupContext, http.MethodDelete, "/fapi/v1/order", url.Values{
			"symbol": {symbol}, "orderId": {result.OrderID},
		}, nil)
	}()
	var cancelledOrder binanceOrderResponse
	if err := trader.privateJSON(ctx, http.MethodDelete, "/fapi/v1/order", url.Values{
		"symbol": {symbol}, "orderId": {result.OrderID},
	}, &cancelledOrder); err != nil {
		result.Status = OrderProbeExposed
		return result, ErrOrderProbeExposed
	}
	cancelled = true
	result.FilledContracts = float64(cancelledOrder.ExecutedQuantity)
	if result.FilledContracts > 1e-12 {
		fill := OrderFill{Venue: trader.Venue(), OrderID: result.OrderID, FilledContracts: result.FilledContracts, AveragePrice: float64(cancelledOrder.AveragePrice)}
		if _, err := trader.CloseFilled(ctx, OrderIntent{Venue: trader.Venue(), Contract: plan.Contract, Side: OrderSell, ReduceOnly: true}, fill); err != nil {
			result.Status = OrderProbeExposed
			return result, ErrOrderProbeExposed
		}
		result.Status = OrderProbeCompensated
	} else {
		result.Status = OrderProbeCancelled
	}
	flat, err := trader.flatPosition(ctx, plan.Contract)
	if err != nil || !flat {
		result.Status = OrderProbeExposed
		return result, ErrOrderProbeExposed
	}
	result.CleanupVerified = true
	return result, nil
}

func (trader *BinanceTrader) flatPosition(ctx context.Context, contract string) (bool, error) {
	var positions []struct {
		PositionAmount flexibleFloat `json:"positionAmt"`
	}
	if err := trader.privateJSON(ctx, http.MethodGet, "/fapi/v2/positionRisk", url.Values{
		"symbol": {strings.ReplaceAll(contract, "_", "")},
	}, &positions); err != nil {
		return false, err
	}
	for _, position := range positions {
		if math.Abs(float64(position.PositionAmount)) > 1e-12 {
			return false, nil
		}
	}
	return true, nil
}
