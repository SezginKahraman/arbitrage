package futures

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GateTrader struct {
	baseURL   string
	client    *http.Client
	apiKey    string
	secret    string
	clock     func() time.Time
	modeMutex sync.RWMutex
	dualMode  map[string]bool
}

type gatePrivateAPIError struct {
	status int
	label  string
}

func (err *gatePrivateAPIError) Error() string {
	return fmt.Sprintf("Gate futures request returned status %d", err.status)
}

func isGatePositionNotFound(err error) bool {
	var apiErr *gatePrivateAPIError
	return errors.As(err, &apiErr) && apiErr.status == http.StatusBadRequest && apiErr.label == "POSITION_NOT_FOUND"
}

func NewGateTrader(apiKey, secret string) *GateTrader {
	return NewGateTraderWithBaseURL(gateAPIBaseURL, &http.Client{Timeout: 10 * time.Second}, apiKey, secret, time.Now)
}

func NewGateTraderWithBaseURL(baseURL string, client *http.Client, apiKey, secret string, clock func() time.Time) *GateTrader {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if clock == nil {
		clock = time.Now
	}
	return &GateTrader{baseURL: strings.TrimRight(baseURL, "/"), client: client, apiKey: apiKey, secret: secret, clock: clock, dualMode: make(map[string]bool)}
}

func (trader *GateTrader) Venue() string { return "gate_futures" }

func (trader *GateTrader) privateJSON(ctx context.Context, method, apiPath string, body any, target any) error {
	if trader.apiKey == "" || trader.secret == "" {
		return ErrLiveTradingDisabled
	}
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return errors.New("encode Gate futures request")
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, trader.baseURL+apiPath, bytes.NewReader(payload))
	if err != nil {
		return errors.New("build Gate futures request")
	}
	timestamp := strconv.FormatInt(trader.clock().Unix(), 10)
	bodyHash := sha512.Sum512(payload)
	signText := strings.Join([]string{method, apiPath, request.URL.RawQuery, hex.EncodeToString(bodyHash[:]), timestamp}, "\n")
	mac := hmac.New(sha512.New, []byte(trader.secret))
	_, _ = mac.Write([]byte(signText))
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("KEY", trader.apiKey)
	request.Header.Set("Timestamp", timestamp)
	request.Header.Set("SIGN", hex.EncodeToString(mac.Sum(nil)))
	request.Header.Set("x-gate-exptime", strconv.FormatInt(trader.clock().Add(5*time.Second).UnixMilli(), 10))
	response, err := trader.client.Do(request)
	if err != nil {
		return errors.New("Gate futures request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		var failure struct {
			Label string `json:"label"`
		}
		_ = json.Unmarshal(payload, &failure)
		return &gatePrivateAPIError{status: response.StatusCode, label: failure.Label}
	}
	if target == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return errors.New("Gate futures response was invalid")
	}
	return nil
}

func (trader *GateTrader) Preflight(ctx context.Context, contract string, requiredNotional float64) error {
	if !contractPattern.MatchString(contract) || !finitePositive(requiredNotional) {
		return errors.New("invalid Gate futures preflight")
	}
	var account struct {
		Available flexibleFloat `json:"available"`
		DualMode  bool          `json:"in_dual_mode"`
	}
	if err := trader.privateJSON(ctx, http.MethodGet, "/api/v4/futures/usdt/accounts", nil, &account); err != nil {
		return err
	}
	if float64(account.Available)+1e-9 < requiredNotional {
		return ErrInsufficientMargin
	}
	positionPath := "/api/v4/futures/usdt/positions/" + contract
	if account.DualMode {
		positionPath = "/api/v4/futures/usdt/dual_comp/positions/" + contract
	}
	if account.DualMode {
		var positions []struct {
			Size flexibleFloat `json:"size"`
		}
		if err := trader.privateJSON(ctx, http.MethodGet, positionPath, nil, &positions); err != nil && !isGatePositionNotFound(err) {
			return err
		}
		for _, position := range positions {
			if math.Abs(float64(position.Size)) > 1e-12 {
				return ErrExistingPosition
			}
		}
	} else {
		var position struct {
			Size flexibleFloat `json:"size"`
		}
		if err := trader.privateJSON(ctx, http.MethodGet, positionPath, nil, &position); err != nil && !isGatePositionNotFound(err) {
			return err
		}
		if math.Abs(float64(position.Size)) > 1e-12 {
			return ErrExistingPosition
		}
	}
	trader.modeMutex.Lock()
	trader.dualMode[contract] = account.DualMode
	trader.modeMutex.Unlock()
	return nil
}

type gateOrderResponse struct {
	ID        int64         `json:"id"`
	Size      flexibleFloat `json:"size"`
	Left      flexibleFloat `json:"left"`
	FillPrice flexibleFloat `json:"fill_price"`
	Status    string        `json:"status"`
	FinishAs  string        `json:"finish_as"`
}

func (trader *GateTrader) PlaceMarket(ctx context.Context, intent OrderIntent) (OrderFill, error) {
	size := intent.Contracts
	if intent.Side == OrderSell {
		size = -size
	}
	payload := map[string]any{
		"contract": intent.Contract, "size": strconv.FormatFloat(size, 'f', -1, 64),
		"price": "0", "tif": "ioc", "text": "t-arbitrage",
	}
	if intent.ReduceOnly {
		payload["reduce_only"] = true
	}
	var response gateOrderResponse
	if err := trader.privateJSON(ctx, http.MethodPost, "/api/v4/futures/usdt/orders", payload, &response); err != nil {
		return OrderFill{}, err
	}
	filled := math.Abs(float64(response.Size)) - math.Abs(float64(response.Left))
	if !finitePositive(filled) {
		return OrderFill{}, errors.New("Gate futures market order was not filled")
	}
	return OrderFill{Venue: trader.Venue(), OrderID: strconv.FormatInt(response.ID, 10), FilledContracts: filled, AveragePrice: float64(response.FillPrice)}, nil
}

func (trader *GateTrader) CloseFilled(ctx context.Context, intent OrderIntent, fill OrderFill) (OrderFill, error) {
	trader.modeMutex.RLock()
	dual := trader.dualMode[intent.Contract]
	trader.modeMutex.RUnlock()
	payload := map[string]any{
		"contract": intent.Contract, "price": "0", "tif": "ioc", "reduce_only": true, "text": "t-arb-comp",
	}
	if dual {
		size := fill.FilledContracts
		if intent.Side == OrderSell {
			size = -size
		}
		payload["size"] = strconv.FormatFloat(size, 'f', -1, 64)
	} else {
		payload["size"] = "0"
		payload["close"] = true
	}
	var response gateOrderResponse
	if err := trader.privateJSON(ctx, http.MethodPost, "/api/v4/futures/usdt/orders", payload, &response); err != nil {
		return OrderFill{}, err
	}
	return OrderFill{Venue: trader.Venue(), OrderID: strconv.FormatInt(response.ID, 10), FilledContracts: fill.FilledContracts, AveragePrice: float64(response.FillPrice)}, nil
}

func (trader *GateTrader) ProbePostOnly(ctx context.Context, plan OrderProbePlan) (result OrderProbeResult, err error) {
	result = OrderProbeResult{
		Venue: plan.Venue, Contract: plan.Contract, Status: OrderProbeFailed,
		Side: plan.Side, Price: plan.Price, Contracts: plan.Contracts, Notional: plan.Notional,
	}
	if plan.Venue != trader.Venue() || !contractPattern.MatchString(plan.Contract) || plan.Side != OrderBuy ||
		!finitePositive(plan.Price) || !finitePositive(plan.Contracts) || !finitePositive(plan.Notional) || plan.Notional > maxOrderProbeNotional {
		return result, ErrOrderProbeFailed
	}
	payload := map[string]any{
		"contract": plan.Contract, "size": strconv.FormatFloat(plan.Contracts, 'f', -1, 64),
		"price": strconv.FormatFloat(plan.Price, 'f', -1, 64), "tif": "poc", "text": "t-access-probe",
	}
	var created gateOrderResponse
	if err := trader.privateJSON(ctx, http.MethodPost, "/api/v4/futures/usdt/orders", payload, &created); err != nil {
		return result, ErrOrderProbeFailed
	}
	if created.ID <= 0 {
		return result, ErrOrderProbeFailed
	}
	result.OrderID = strconv.FormatInt(created.ID, 10)
	orderPath := "/api/v4/futures/usdt/orders/" + result.OrderID
	cancelled := false
	defer func() {
		if cancelled {
			return
		}
		cleanupContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = trader.privateJSON(cleanupContext, http.MethodDelete, orderPath, nil, nil)
	}()
	var cancelledOrder gateOrderResponse
	if err := trader.privateJSON(ctx, http.MethodDelete, orderPath, nil, &cancelledOrder); err != nil {
		result.Status = OrderProbeExposed
		return result, ErrOrderProbeExposed
	}
	cancelled = true
	result.FilledContracts = math.Max(0, math.Abs(float64(cancelledOrder.Size))-math.Abs(float64(cancelledOrder.Left)))
	if result.FilledContracts > 1e-12 {
		fill := OrderFill{Venue: trader.Venue(), OrderID: result.OrderID, FilledContracts: result.FilledContracts, AveragePrice: float64(cancelledOrder.FillPrice)}
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

func (trader *GateTrader) flatPosition(ctx context.Context, contract string) (bool, error) {
	trader.modeMutex.RLock()
	dual := trader.dualMode[contract]
	trader.modeMutex.RUnlock()
	positionPath := "/api/v4/futures/usdt/positions/" + contract
	if dual {
		positionPath = "/api/v4/futures/usdt/dual_comp/positions/" + contract
		var positions []struct {
			Size flexibleFloat `json:"size"`
		}
		if err := trader.privateJSON(ctx, http.MethodGet, positionPath, nil, &positions); err != nil {
			if isGatePositionNotFound(err) {
				return true, nil
			}
			return false, err
		}
		for _, position := range positions {
			if math.Abs(float64(position.Size)) > 1e-12 {
				return false, nil
			}
		}
		return true, nil
	}
	var position struct {
		Size flexibleFloat `json:"size"`
	}
	if err := trader.privateJSON(ctx, http.MethodGet, positionPath, nil, &position); err != nil {
		if isGatePositionNotFound(err) {
			return true, nil
		}
		return false, err
	}
	return math.Abs(float64(position.Size)) <= 1e-12, nil
}
